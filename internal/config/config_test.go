package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.HTTPPort != "8080" {
		t.Errorf("HTTPPort = %q, want 8080", cfg.HTTPPort)
	}
	if cfg.RedisEnabled() {
		t.Error("RedisEnabled should be false by default")
	}
	if !strings.HasPrefix(cfg.DatabaseURL, "postgres://") {
		t.Errorf("DatabaseURL = %q, want postgres:// prefix", cfg.DatabaseURL)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", cfg.PollInterval)
	}
}

func TestLoad_DatabaseURLWins(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://u:p@db.example.com:5432/app?sslmode=require")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://u:p@db.example.com:5432/app?sslmode=require" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
}

func TestLoad_RedisURL(t *testing.T) {
	clearEnv(t)
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.RedisEnabled() {
		t.Error("RedisEnabled should be true when REDIS_URL set")
	}
}

func TestLoad_PortNormalization(t *testing.T) {
	clearEnv(t)
	t.Setenv("PORT", ":9090")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.HTTPPort != "9090" {
		t.Errorf("HTTPPort = %q, want 9090", cfg.HTTPPort)
	}
}

func TestResolveRedisURL_Discrete(t *testing.T) {
	clearEnv(t)
	t.Setenv("REDIS_HOST", "cache")
	t.Setenv("REDIS_PORT", "6380")
	got := resolveRedisURL()
	if got != "redis://cache:6380" {
		t.Errorf("resolveRedisURL = %q, want redis://cache:6380", got)
	}
}

// clearEnv ensures a clean environment for each test.
func clearEnv(t *testing.T) {
	for _, k := range []string{
		"APP_ENV", "PORT", "HTTP_PORT", "NODE_ID",
		"DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"REDIS_URL", "REDIS_HOST", "REDIS_PORT",
		"POLL_INTERVAL", "LOCK_TTL", "SEED_DATA", "SERVE_FRONTEND",
	} {
		t.Setenv(k, "")
	}
}
