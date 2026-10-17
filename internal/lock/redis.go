package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// releaseScript performs a compare-and-delete so a node can only release a
// lock that it still owns. This prevents a slow node from deleting a lock that
// has already expired and been re-acquired by another node.
var releaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end`)

// RedisLocker implements Locker on top of Redis.
type RedisLocker struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisLockerFromURL builds a RedisLocker from a redis:// connection URL.
func NewRedisLockerFromURL(url string, ttl time.Duration) (*RedisLocker, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return &RedisLocker{client: redis.NewClient(opts), ttl: ttl}, nil
}

// Acquire takes the lock using SET key token NX PX ttl.
func (r *RedisLocker) Acquire(ctx context.Context, key string) (bool, func(), error) {
	token, err := randomToken()
	if err != nil {
		return false, noop, err
	}
	ok, err := r.client.SetNX(ctx, key, token, r.ttl).Result()
	if err != nil {
		return false, noop, fmt.Errorf("redis setnx: %w", err)
	}
	if !ok {
		return false, noop, nil
	}
	release := func() {
		// Use a short, independent context so release still runs even if the
		// caller's context was cancelled.
		rctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = releaseScript.Run(rctx, r.client, []string{key}, token).Err()
	}
	return true, release, nil
}

// Ping verifies connectivity to Redis.
func (r *RedisLocker) Ping(ctx context.Context) error { return r.client.Ping(ctx).Err() }

// Kind returns the backend name.
func (r *RedisLocker) Kind() string { return "redis" }

// Close closes the underlying client.
func (r *RedisLocker) Close() error { return r.client.Close() }

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
