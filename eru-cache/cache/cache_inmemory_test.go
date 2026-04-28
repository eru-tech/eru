package cache

import (
	"context"
	"testing"
	"time"
)

func TestInMemory_SetGet(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()
	if err := c.Set(ctx, "k1", "v1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "v1" {
		t.Fatalf("want v1, got %q", got)
	}
}

func TestInMemory_TTLExpires(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()
	if err := c.SetWithTTL(ctx, "k", "v", 30*time.Millisecond); err != nil {
		t.Fatalf("SetWithTTL: %v", err)
	}
	if _, err := c.Get(ctx, "k"); err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	if _, err := c.Get(ctx, "k"); err == nil {
		t.Fatalf("expected expiry error, got nil")
	}
}

func TestInMemory_TagInvalidation(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()
	if err := c.SetWithTagsTTL(ctx, "k1", "v1", 0, []string{"tblA", "tblB"}); err != nil {
		t.Fatalf("SetWithTagsTTL k1: %v", err)
	}
	if err := c.SetWithTagsTTL(ctx, "k2", "v2", 0, []string{"tblB"}); err != nil {
		t.Fatalf("SetWithTagsTTL k2: %v", err)
	}
	if err := c.SetWithTagsTTL(ctx, "k3", "v3", 0, []string{"tblC"}); err != nil {
		t.Fatalf("SetWithTagsTTL k3: %v", err)
	}

	n, err := c.InvalidateByTags(ctx, []string{"tblB"})
	if err != nil {
		t.Fatalf("InvalidateByTags: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deletions, got %d", n)
	}
	if _, err := c.Get(ctx, "k1"); err == nil {
		t.Fatalf("k1 should be gone")
	}
	if _, err := c.Get(ctx, "k2"); err == nil {
		t.Fatalf("k2 should be gone")
	}
	if v, err := c.Get(ctx, "k3"); err != nil || v != "v3" {
		t.Fatalf("k3 should survive, got %q err %v", v, err)
	}
}

func TestInMemory_TagReassignOnReset(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()
	_ = c.SetWithTagsTTL(ctx, "k", "v1", 0, []string{"old"})
	_ = c.SetWithTagsTTL(ctx, "k", "v2", 0, []string{"new"})

	n, _ := c.InvalidateByTags(ctx, []string{"old"})
	if n != 0 {
		t.Fatalf("expected 0 (old tag should have been cleared on re-set), got %d", n)
	}
	if v, err := c.Get(ctx, "k"); err != nil || v != "v2" {
		t.Fatalf("k should still have v2, got %q err %v", v, err)
	}
	n, _ = c.InvalidateByTags(ctx, []string{"new"})
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
}

func TestInMemory_LockAcquireAndCAS(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()
	ok, err := c.AcquireLock(ctx, "k", time.Second, "owner-A")
	if err != nil || !ok {
		t.Fatalf("A should acquire: ok=%v err=%v", ok, err)
	}
	ok, err = c.AcquireLock(ctx, "k", time.Second, "owner-B")
	if err != nil {
		t.Fatalf("B acquire err: %v", err)
	}
	if ok {
		t.Fatalf("B should not acquire while A holds")
	}
	// B tries to release — should be no-op due to CAS
	if err := c.ReleaseLock(ctx, "k", "owner-B"); err != nil {
		t.Fatalf("ReleaseLock B: %v", err)
	}
	ok, _ = c.AcquireLock(ctx, "k", time.Second, "owner-B")
	if ok {
		t.Fatalf("B still should not acquire after bogus release")
	}
	// A releases properly
	if err := c.ReleaseLock(ctx, "k", "owner-A"); err != nil {
		t.Fatalf("ReleaseLock A: %v", err)
	}
	ok, _ = c.AcquireLock(ctx, "k", time.Second, "owner-B")
	if !ok {
		t.Fatalf("B should acquire after A released")
	}
}

func TestInMemory_LockExpiry(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()
	ok, _ := c.AcquireLock(ctx, "k", 30*time.Millisecond, "A")
	if !ok {
		t.Fatalf("A should acquire")
	}
	time.Sleep(60 * time.Millisecond)
	ok, _ = c.AcquireLock(ctx, "k", time.Second, "B")
	if !ok {
		t.Fatalf("B should acquire after expiry")
	}
}

func TestInMemory_DeleteClearsTagMembership(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()
	_ = c.SetWithTagsTTL(ctx, "k", "v", 0, []string{"t1"})
	_ = c.Delete(ctx, "k")
	n, _ := c.InvalidateByTags(ctx, []string{"t1"})
	if n != 0 {
		t.Fatalf("expected 0 after delete, got %d", n)
	}
}

func TestInMemory_GetKeysRespectsExpiry(t *testing.T) {
	c := NewInMemoryCache()
	ctx := context.Background()
	_ = c.SetWithTTL(ctx, "alive", "v", time.Second)
	_ = c.SetWithTTL(ctx, "dead", "v", 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	keys, _ := c.GetKeys(ctx, "*")
	for _, k := range keys {
		if k == "dead" {
			t.Fatalf("expired key should not be listed")
		}
	}
}
