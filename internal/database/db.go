// Package database manages the PostgreSQL connection pool, schema migrations
// and optional seed data.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Open creates a configured, verified connection pool.
func Open(ctx context.Context, dsn string, maxOpen, maxIdle int, maxLifetime time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)

	// Retry ping to tolerate databases that are still starting up.
	var lastErr error
	for i := 0; i < 10; i++ {
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := db.PingContext(pctx)
		cancel()
		if err == nil {
			return db, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	_ = db.Close()
	return nil, fmt.Errorf("database unreachable after retries: %w", lastErr)
}

// Migrate creates or upgrades the schema. It is idempotent.
func Migrate(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id            BIGSERIAL PRIMARY KEY,
			name          TEXT        NOT NULL,
			cron_schedule TEXT        NOT NULL,
			status        TEXT        NOT NULL DEFAULT 'active',
			next_run_at   TIMESTAMPTZ NOT NULL,
			last_run_at   TIMESTAMPTZ,
			run_count     BIGINT      NOT NULL DEFAULT 0,
			fail_count    BIGINT      NOT NULL DEFAULT 0,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS job_history (
			id          BIGSERIAL PRIMARY KEY,
			job_id      BIGINT      NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
			run_at      TIMESTAMPTZ NOT NULL,
			status      TEXT        NOT NULL,
			duration_ms BIGINT      NOT NULL DEFAULT 0,
			node_id     TEXT        NOT NULL DEFAULT '',
			details     TEXT        NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_next_run ON jobs (next_run_at) WHERE status = 'active'`,
		`CREATE INDEX IF NOT EXISTS idx_history_job_id ON job_history (job_id, run_at DESC)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// Seed inserts a couple of demonstration jobs when the table is empty.
func Seed(ctx context.Context, db *sql.DB) error {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs").Scan(&count); err != nil {
		return fmt.Errorf("seed count: %w", err)
	}
	if count > 0 {
		return nil
	}
	seeds := []struct {
		name string
		cron string
	}{
		{"Send Digest Email", "*/1 * * * *"},
		{"Database Backup", "*/5 * * * *"},
		{"Cache Warmup", "*/2 * * * *"},
	}
	for _, s := range seeds {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO jobs (name, cron_schedule, status, next_run_at)
			 VALUES ($1, $2, 'active', now())`, s.name, s.cron); err != nil {
			return fmt.Errorf("seed insert: %w", err)
		}
	}
	return nil
}
