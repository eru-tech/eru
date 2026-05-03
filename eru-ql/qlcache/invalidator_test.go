package qlcache

import (
	"context"
	"testing"
	"time"

	"github.com/eru-tech/eru/eru-cache/cache"
	"github.com/eru-tech/eru/eru-ql/module_model"
)

func TestEnqueueInvalidate_RemovesTaggedEntries(t *testing.T) {
	ctx := context.Background()
	StartInvalidator(ctx, 1)

	ds := newEnabledDS()
	c := ds.QueryCache

	// seed two entries tagged with different tables
	if err := c.SetWithTagsTTL(ctx, "k1", "v1", time.Minute, []string{TableTag("p1", "primary","public.orders")}); err != nil {
		t.Fatalf("seed k1: %v", err)
	}
	if err := c.SetWithTagsTTL(ctx, "k2", "v2", time.Minute, []string{TableTag("p1", "primary","public.users")}); err != nil {
		t.Fatalf("seed k2: %v", err)
	}

	EnqueueInvalidate(ctx, ds, []string{"public.orders"})

	// wait for worker to process
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := c.Get(ctx, "k1"); err != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := c.Get(ctx, "k1"); err == nil {
		t.Fatalf("k1 should be invalidated")
	}
	if _, err := c.Get(ctx, "k2"); err != nil {
		t.Fatalf("k2 should survive: %v", err)
	}
}

func TestEnqueueInvalidate_NoopWhenDisabled(t *testing.T) {
	ctx := context.Background()
	ds := &module_model.DataSource{ProjectId: "p1", DbAlias: "p"}
	ds.QueryCache = cache.NewInMemoryCache()
	ds.QueryCacheConfig.Enabled = false
	// should simply return without touching anything
	EnqueueInvalidate(ctx, ds, []string{"any"})
}

func TestEnqueueInvalidate_NoopWhenNoCache(t *testing.T) {
	ctx := context.Background()
	ds := &module_model.DataSource{ProjectId: "p1", DbAlias: "p"}
	ds.QueryCacheConfig.Enabled = true
	EnqueueInvalidate(ctx, ds, []string{"any"})
}

func TestInvalidateBlocking_ReturnsDeletedCount(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	_ = ds.QueryCache.SetWithTagsTTL(ctx, "a", "va", time.Minute, []string{TableTag("p1", "primary","public.products")})
	_ = ds.QueryCache.SetWithTagsTTL(ctx, "b", "vb", time.Minute, []string{TableTag("p1", "primary","public.products")})
	n, err := InvalidateBlocking(ctx, ds, []string{"public.products"})
	if err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deletions, got %d", n)
	}
}

func TestIsDML(t *testing.T) {
	cases := map[string]bool{
		"INSERT INTO x VALUES(1)":   true,
		" update x set y=1":         true,
		"DELETE FROM x":             true,
		"truncate table x":          true,
		"select * from x":           false,
		"with cte as (select 1) select * from cte": false,
		"":                          false,
	}
	for sql, want := range cases {
		if got := IsDML(sql); got != want {
			t.Errorf("IsDML(%q)=%v want %v", sql, got, want)
		}
	}
}
