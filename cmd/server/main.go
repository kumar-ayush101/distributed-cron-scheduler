// Command server is the entrypoint for the distributed cron scheduler. It
// wires configuration, logging, the database, the lock backend, the HTTP API
// and the scheduling engine, and manages graceful startup and shutdown.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/config"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/database"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/httpapi"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/lock"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/metrics"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/repository"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/scheduler"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := newLogger(cfg)
	log.Info("starting distributed cron scheduler",
		"env", cfg.Env, "node_id", cfg.NodeID, "port", cfg.HTTPPort,
		"redis_enabled", cfg.RedisEnabled())

	// Root context cancelled on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Database
	db, err := database.Open(ctx, cfg.DatabaseURL, cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := database.Migrate(ctx, db); err != nil {
		return err
	}
	if cfg.SeedData {
		if err := database.Seed(ctx, db); err != nil {
			log.Warn("seed failed", "error", err)
		}
	}

	// Lock backend: Redis if configured, otherwise Postgres advisory locks.
	locker, err := buildLocker(ctx, cfg, db, log)
	if err != nil {
		return err
	}
	defer locker.Close()

	// Metrics
	m := metrics.New()

	// Wiring
	repo := repository.NewJobRepository(db)
	jobSvc := service.NewJobService(repo)
	handlers := httpapi.NewHandlers(jobSvc, db, locker, log, cfg.NodeID)

	router := httpapi.NewRouter(httpapi.RouterConfig{
		Handlers:      handlers,
		Metrics:       m,
		Log:           log,
		ServeFrontend: cfg.ServeFrontend,
		FrontendDir:   cfg.FrontendDir,
	})

	// Scheduler
	sched := scheduler.New(repo, locker, log, m, scheduler.Options{
		NodeID:       cfg.NodeID,
		PollInterval: cfg.PollInterval,
		LockTTL:      cfg.LockTTL,
	})
	go sched.Run(ctx)

	// HTTP server
	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server failure.
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
		return err
	}
	log.Info("shutdown complete")
	return nil
}

// buildLocker selects the lock backend based on configuration.
func buildLocker(ctx context.Context, cfg *config.Config, db *sql.DB, log *slog.Logger) (lock.Locker, error) {
	if cfg.RedisEnabled() {
		rl, err := lock.NewRedisLockerFromURL(cfg.RedisURL, cfg.LockTTL)
		if err != nil {
			return nil, err
		}
		if err := rl.Ping(ctx); err != nil {
			log.Warn("redis unreachable, falling back to postgres advisory locks", "error", err)
			rl.Close()
			return lock.NewPostgresLocker(db), nil
		}
		log.Info("using redis lock backend")
		return rl, nil
	}
	log.Info("using postgres advisory lock backend (no REDIS_URL set)")
	return lock.NewPostgresLocker(db), nil
}

func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	if !cfg.IsProduction() {
		level = slog.LevelDebug
	}
	var handler slog.Handler
	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	return slog.New(handler).With("service", "cron-scheduler")
}
