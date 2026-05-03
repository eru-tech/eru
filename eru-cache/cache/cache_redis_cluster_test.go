package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Set REDIS_CLUSTER_TEST_ADDRS=host1:6379,host2:6379,... to run.
// Tests FlushDB on every master, so point at a disposable cluster only.
func newRedisClusterTestCache(t *testing.T) *RedisClusterCache {
	t.Helper()
	addrs := os.Getenv("REDIS_CLUSTER_TEST_ADDRS")
	if addrs == "" {
		t.Skip("REDIS_CLUSTER_TEST_ADDRS not set; skipping cluster-backed test")
	}
	rc := &RedisClusterCache{
		CacheStore:      CacheStore{CacheStoreType: "REDIS_CLUSTER"},
		RedisAddr:       addrs,
		HashTag:         "eruqltest",
		PoolSize:        5,
		MinIdleConns:    1,
		ReadTimeoutMs:   2000,
		WriteTimeoutMs:  2000,
		MaxRetries:      1,
		TagSetTTLSecMax: 60,
		TLSEnabled:      os.Getenv("REDIS_CLUSTER_TEST_TLS") == "true",
		TLSSkipVerify:   os.Getenv("REDIS_CLUSTER_TEST_TLS_SKIP_VERIFY") == "true",
		TLSServerName:   os.Getenv("REDIS_CLUSTER_TEST_TLS_SERVER_NAME"),
	}
	if err := rc.Init(context.Background()); err != nil {
		t.Skipf("cannot connect to cluster at %s: %v", addrs, err)
	}
	if err := rc.Client.ForEachMaster(context.Background(), func(ctx context.Context, master *redis.Client) error {
		return master.FlushDB(ctx).Err()
	}); err != nil {
		t.Fatalf("flushdb: %v", err)
	}
	return rc
}

func TestRedisCluster_SetGetTTL(t *testing.T) {
	rc := newRedisClusterTestCache(t)
	ctx := context.Background()
	if err := rc.SetWithTTL(ctx, "k", "v", 0); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := rc.Get(ctx, "k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != `"v"` {
		t.Fatalf("got %q", got)
	}
}

func TestRedisCluster_TagInvalidation(t *testing.T) {
	rc := newRedisClusterTestCache(t)
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
	if _, err := rc.Get(ctx, "k3"); err != nil {
		t.Fatalf("k3 should survive; err=%v", err)
	}
}

func TestRedisCluster_LockCAS(t *testing.T) {
	rc := newRedisClusterTestCache(t)
	ctx := context.Background()

	ok, err := rc.AcquireLock(ctx, "job", time.Second, "A")
	if err != nil || !ok {
		t.Fatalf("A should acquire; ok=%v err=%v", ok, err)
	}
	ok, _ = rc.AcquireLock(ctx, "job", time.Second, "B")
	if ok {
		t.Fatalf("B should not acquire")
	}
	if err := rc.ReleaseLock(ctx, "job", "B"); err != nil {
		t.Fatalf("release B: %v", err)
	}
	ok, _ = rc.AcquireLock(ctx, "job", time.Second, "B")
	if ok {
		t.Fatalf("B should still not acquire after bogus release")
	}
	if err := rc.ReleaseLock(ctx, "job", "A"); err != nil {
		t.Fatalf("release A: %v", err)
	}
	ok, _ = rc.AcquireLock(ctx, "job", time.Second, "B")
	if !ok {
		t.Fatalf("B should acquire after A released")
	}
}
