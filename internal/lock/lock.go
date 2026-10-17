// Package lock provides a distributed mutual-exclusion abstraction used to
// guarantee that each scheduled job runs exactly once across a cluster of
// stateless worker nodes.
//
// Two implementations are provided:
//
//   - RedisLocker uses Redis SET NX PX with a fencing token and a Lua
//     compare-and-delete for safe release. Preferred for multi-node clusters.
//   - PostgresLocker uses session-scoped advisory locks
//     (pg_try_advisory_lock). It requires no extra infrastructure, which makes
//     it ideal for single-database deployments such as Render's free tier.
//
// Both satisfy the Locker interface so the scheduler is agnostic to the
// backing store.
package lock

import "context"

// Locker acquires and releases named distributed locks.
type Locker interface {
	// Acquire attempts to take the lock for key. It returns ok=true and a
	// release function when the lock was obtained. When the lock is already
	// held elsewhere it returns ok=false and a no-op release function.
	Acquire(ctx context.Context, key string) (ok bool, release func(), err error)

	// Ping verifies the backing store is reachable.
	Ping(ctx context.Context) error

	// Kind returns a human-readable backend name (e.g. "redis", "postgres").
	Kind() string

	// Close releases any resources held by the locker.
	Close() error
}

// noop is returned as the release function when a lock was not acquired.
func noop() {}
