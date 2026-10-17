// Package config loads and validates application configuration from the
// environment. It supports both discrete variables (DB_HOST, DB_PORT, ...)
// for local Docker Compose development and single connection strings
// (DATABASE_URL, REDIS_URL) used by managed platforms such as Render,
// Railway, Fly.io and Heroku.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	// Core
	Env      string // "development" | "production"
	HTTPPort string
	NodeID   string // unique identifier for this node, used in logs and locks

	// Database
	DatabaseURL     string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	// Redis (optional). When empty the scheduler falls back to Postgres
	// advisory locks, which keeps the exactly-once guarantee without Redis.
	RedisURL string

	// Scheduler
	PollInterval time.Duration
	LockTTL      time.Duration
	MaxRetries   int

	// Behaviour
	SeedData       bool
	ServeFrontend  bool
	FrontendDir    string
	ShutdownGrace  time.Duration
}

// Load reads configuration from the environment, applying sensible defaults.
func Load() (*Config, error) {
	c := &Config{
		Env:             getenv("APP_ENV", "development"),
		HTTPPort:        normalizePort(firstNonEmpty(os.Getenv("PORT"), os.Getenv("HTTP_PORT"), "8080")),
		NodeID:          getenv("NODE_ID", defaultNodeID()),
		MaxOpenConns:    getenvInt("DB_MAX_OPEN_CONNS", 10),
		MaxIdleConns:    getenvInt("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: getenvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		RedisURL:        resolveRedisURL(),
		PollInterval:    getenvDuration("POLL_INTERVAL", 5*time.Second),
		LockTTL:         getenvDuration("LOCK_TTL", 30*time.Second),
		MaxRetries:      getenvInt("MAX_RETRIES", 3),
		SeedData:        getenvBool("SEED_DATA", true),
		ServeFrontend:   getenvBool("SERVE_FRONTEND", true),
		FrontendDir:     getenv("FRONTEND_DIR", "./web"),
		ShutdownGrace:   getenvDuration("SHUTDOWN_GRACE", 10*time.Second),
	}

	dbURL, err := resolveDatabaseURL()
	if err != nil {
		return nil, err
	}
	c.DatabaseURL = dbURL

	if c.PollInterval <= 0 {
		return nil, fmt.Errorf("POLL_INTERVAL must be positive")
	}
	if c.LockTTL <= 0 {
		return nil, fmt.Errorf("LOCK_TTL must be positive")
	}
	return c, nil
}

// IsProduction reports whether the app is running in production mode.
func (c *Config) IsProduction() bool { return c.Env == "production" }

// RedisEnabled reports whether a Redis URL was configured.
func (c *Config) RedisEnabled() bool { return strings.TrimSpace(c.RedisURL) != "" }

// resolveDatabaseURL builds a libpq-compatible connection string either from
// DATABASE_URL (preferred, platform style) or from discrete DB_* variables.
func resolveDatabaseURL() (string, error) {
	if raw := strings.TrimSpace(os.Getenv("DATABASE_URL")); raw != "" {
		return raw, nil
	}

	host := getenv("DB_HOST", "localhost")
	port := getenv("DB_PORT", "5432")
	user := getenv("DB_USER", "postgres")
	pass := getenv("DB_PASSWORD", "secret")
	name := getenv("DB_NAME", "postgres")
	sslmode := getenv("DB_SSLMODE", "disable")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   "/" + name,
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// resolveRedisURL accepts either REDIS_URL or discrete REDIS_HOST/REDIS_PORT.
// Returns an empty string when Redis is not configured.
func resolveRedisURL() string {
	if raw := strings.TrimSpace(os.Getenv("REDIS_URL")); raw != "" {
		return raw
	}
	host := strings.TrimSpace(os.Getenv("REDIS_HOST"))
	if host == "" {
		return ""
	}
	port := getenv("REDIS_PORT", "6379")
	return fmt.Sprintf("redis://%s:%s", host, port)
}

// --- small env helpers ---

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func getenvInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func normalizePort(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, ":")
	return p
}

func defaultNodeID() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "node-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
