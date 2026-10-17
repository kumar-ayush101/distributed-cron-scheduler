package lock

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"time"
)

// PostgresLocker implements Locker using PostgreSQL session-level advisory
// locks. A dedicated connection is pulled from the pool per acquisition so the
// lock stays bound to that session until it is explicitly released, giving the
// same exactly-once semantics as Redis without any extra infrastructure.
type PostgresLocker struct {
	db *sql.DB
}

// NewPostgresLocker builds a PostgresLocker from an existing *sql.DB pool.
func NewPostgresLocker(db *sql.DB) *PostgresLocker {
	return &PostgresLocker{db: db}
}

// Acquire tries a non-blocking advisory lock keyed by a 64-bit hash of key.
func (p *PostgresLocker) Acquire(ctx context.Context, key string) (bool, func(), error) {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return false, noop, fmt.Errorf("acquire conn: %w", err)
	}

	id := hashKey(key)
	var ok bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", id).Scan(&ok); err != nil {
		_ = conn.Close()
		return false, noop, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	if !ok {
		// Lock held by another session; return the connection to the pool.
		_ = conn.Close()
		return false, noop, nil
	}

	release := func() {
		rctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(rctx, "SELECT pg_advisory_unlock($1)", id)
		// Closing the connection returns it to the pool; the unlock above
		// also releases it, but we unlock explicitly for clarity.
		_ = conn.Close()
	}
	return true, release, nil
}

// Ping verifies the database is reachable.
func (p *PostgresLocker) Ping(ctx context.Context) error { return p.db.PingContext(ctx) }

// Kind returns the backend name.
func (p *PostgresLocker) Kind() string { return "postgres" }

// Close is a no-op; the shared *sql.DB is owned by the caller.
func (p *PostgresLocker) Close() error { return nil }

// hashKey maps an arbitrary lock key to a signed 64-bit advisory-lock id.
func hashKey(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int64(h.Sum64())
}
