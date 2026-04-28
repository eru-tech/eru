package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// These tests exercise the Redis backend against a real server.
// They run only when REDIS_TEST_ADDR is set, so they are safe in CI
// environments without Redis. Point REDIS_TEST_ADDR at a disposable
// Redis (e.g. localhost:6379, db 15) — the tests FLUSHDB before use.
func newRedisTestCache(t *testing.T) *RedisCache {
	t.Helper()
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR not set; skipping redis-backed test")
	}
	rc := &RedisCache{
		CacheStore:      CacheStore{CacheStoreType: "REDIS"},
		RedisAddr:       addr,
		RedisUsername:   os.Getenv("REDIS_TEST_USERNAME"),
		RedisPassword:   os.Getenv("REDIS_TEST_PASSWORD"),
		RedisDB:         15,
		PoolSize:        5,
		MinIdleConns:    1,
		ReadTimeoutMs:   2000,
		WriteTimeoutMs:  2000,
		MaxRetries:      1,
		TagSetTTLSecMax: 60,
		TLSEnabled:      os.Getenv("REDIS_TEST_TLS") == "true",
		TLSSkipVerify:   os.Getenv("REDIS_TEST_TLS_SKIP_VERIFY") == "true",
		TLSServerName:   os.Getenv("REDIS_TEST_TLS_SERVER_NAME"),
	}
	if err := rc.Init(context.Background()); err != nil {
		t.Skipf("cannot connect to redis at %s: %v", addr, err)
	}
	if err := rc.Client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flushdb: %v", err)
	}
	return rc
}

func TestRedis_SetGetTTL(t *testing.T) {
	rc := newRedisTestCache(t)
	ctx := context.Background()
	if err := rc.SetWithTTL(ctx, "k", "v", 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := rc.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != `"v"` { // JSON-encoded by SetWithTTL
		t.Fatalf("got %q", got)
	}
}

func TestRedis_TagInvalidation(t *testing.T) {
	rc := newRedisTestCache(t)
	ctx := context.Background()

	if err := rc.SetWithTagsTTL(ctx, "k1", "v1", time.Minute, []string{"tblA", "tblB"}); err != nil {
		t.Fatalf("set k1: %v", err)
	}
	if err := rc.SetWithTagsTTL(ctx, "k2", "v2", time.Minute, []string{"tblB"}); err != nil {
		t.Fatalf("set k2: %v", err)
	}
	if err := rc.SetWithTagsTTL(ctx, "k3", "v3", time.Minute, []string{"tblC"}); err != nil {
		t.Fatalf("set k3: %v", err)
	}

	n, err := rc.InvalidateByTags(ctx, []string{"tblB"})
	if err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deletions, got %d", n)
	}
	if _, err := rc.Get(ctx, "k1"); err != redis.Nil {
		t.Fatalf("k1 should be gone; err=%v", err)
	}
	if _, err := rc.Get(ctx, "k2"); err != redis.Nil {
		t.Fatalf("k2 should be gone; err=%v", err)
	}
	if _, err := rc.Get(ctx, "k3"); err != nil {
		t.Fatalf("k3 should survive; err=%v", err)
	}
}

func TestRedis_LockCAS(t *testing.T) {
	rc := newRedisTestCache(t)
	ctx := context.Background()

	ok, err := rc.AcquireLock(ctx, "job", time.Second, "A")
	if err != nil || !ok {
		t.Fatalf("A should acquire; ok=%v err=%v", ok, err)
	}
	ok, err = rc.AcquireLock(ctx, "job", time.Second, "B")
	if err != nil {
		t.Fatalf("B acquire err: %v", err)
	}
	if ok {
		t.Fatalf("B should not acquire")
	}
	// Bogus release by B should not unlock A
	if err := rc.ReleaseLock(ctx, "job", "B"); err != nil {
		t.Fatalf("release B: %v", err)
	}
	ok, _ = rc.AcquireLock(ctx, "job", time.Second, "B")
	if ok {
		t.Fatalf("B should still not acquire after bogus release")
	}
	// Proper release by A
	if err := rc.ReleaseLock(ctx, "job", "A"); err != nil {
		t.Fatalf("release A: %v", err)
	}
	ok, _ = rc.AcquireLock(ctx, "job", time.Second, "B")
	if !ok {
		t.Fatalf("B should acquire after A released")
	}
}

func TestRedis_LockExpiry(t *testing.T) {
	rc := newRedisTestCache(t)
	ctx := context.Background()
	ok, _ := rc.AcquireLock(ctx, "j", 100*time.Millisecond, "A")
	if !ok {
		t.Fatalf("A should acquire")
	}
	time.Sleep(150 * time.Millisecond)
	ok, _ = rc.AcquireLock(ctx, "j", time.Second, "B")
	if !ok {
		t.Fatalf("B should acquire after TTL")
	}
}
