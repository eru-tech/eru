package qlcache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eru-tech/eru/eru-ql/module_model"
)

func TestServeOrLoad_HitSkipsLoader(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	sql := "select * from a"

	// seed
	key := BuildKey(ds.ProjectId, ds.DbAlias, sql)
	populateWithOverride(ctx, ds, key, map[string]interface{}{"x": "y"}, nil, 60)
	waitForPopulate(ds, key, 200*time.Millisecond)

	var called int32
	loader := func(ctx context.Context) (map[string]interface{}, []string, error) {
		atomic.AddInt32(&called, 1)
		return map[string]interface{}{"x": "FRESH"}, nil, nil
	}
	res, err := ServeOrLoad(ctx, ds, sql, "public.", loader, Options{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if atomic.LoadInt32(&called) != 0 {
		t.Fatalf("loader should not have run on cache hit")
	}
	if res["x"] != "y" {
		t.Fatalf("got fresh value instead of cached: %v", res)
	}
}

func TestServeOrLoad_MissRunsLoaderAndPopulates(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	sql := "select * from b"

	loader := func(ctx context.Context) (map[string]interface{}, []string, error) {
		return map[string]interface{}{"b": "val"}, []string{"orders"}, nil
	}
	res, err := ServeOrLoad(ctx, ds, sql, "public.", loader, Options{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res["b"] != "val" {
		t.Fatalf("wrong result: %v", res)
	}
	key := BuildKey(ds.ProjectId, ds.DbAlias, sql)
	if !waitForPopulate(ds, key, 200*time.Millisecond) {
		t.Fatalf("cache should have been populated on miss")
	}
}

func TestServeOrLoad_SkipCacheBypassesEntirely(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	sql := "select * from c"

	loader := func(ctx context.Context) (map[string]interface{}, []string, error) {
		return map[string]interface{}{"c": "val"}, nil, nil
	}
	_, err := ServeOrLoad(ctx, ds, sql, "public.", loader, Options{SkipCache: true})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	key := BuildKey(ds.ProjectId, ds.DbAlias, sql)
	time.Sleep(30 * time.Millisecond)
	if _, err := ds.QueryCache.Get(ctx, key); err == nil {
		t.Fatalf("cache should NOT be populated when SkipCache=true")
	}
}

func TestServeOrLoad_VolatileTableSkipsPopulate(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	ds.QueryCacheConfig.VolatileTables = []string{"public.sessions"}
	sql := "select * from sessions"

	loader := func(ctx context.Context) (map[string]interface{}, []string, error) {
		return map[string]interface{}{"x": 1}, []string{"sessions"}, nil
	}
	_, err := ServeOrLoad(ctx, ds, sql, "public.", loader, Options{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	key := BuildKey(ds.ProjectId, ds.DbAlias, sql)
	if _, err := ds.QueryCache.Get(ctx, key); err == nil {
		t.Fatalf("volatile table result should not be cached")
	}
}

func TestServeOrLoad_TTLOverride(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	ds.QueryCacheConfig.DefaultTTLSec = 1
	sql := "select * from d"

	loader := func(ctx context.Context) (map[string]interface{}, []string, error) {
		return map[string]interface{}{"ok": true}, nil, nil
	}
	_, err := ServeOrLoad(ctx, ds, sql, "public.", loader, Options{TTLSec: 60})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	key := BuildKey(ds.ProjectId, ds.DbAlias, sql)
	if !waitForPopulate(ds, key, 200*time.Millisecond) {
		t.Fatalf("expected populate")
	}
	// After default TTL would have expired, value should still be present
	time.Sleep(1200 * time.Millisecond)
	if _, err := ds.QueryCache.Get(ctx, key); err != nil {
		t.Fatalf("override TTL 60s — should still be present: %v", err)
	}
}

func TestServeOrLoad_SingleflightDedupesConcurrentMisses(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	sql := "select * from big_join"

	var callCount int32
	loader := func(ctx context.Context) (map[string]interface{}, []string, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(80 * time.Millisecond) // simulate slow query
		return map[string]interface{}{"row": 1}, nil, nil
	}

	const N = 20
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_, _ = ServeOrLoad(ctx, ds, sql, "public.", loader, Options{})
		}()
	}
	wg.Wait()

	if n := atomic.LoadInt32(&callCount); n != 1 {
		t.Fatalf("singleflight should have reduced loader calls to 1, got %d", n)
	}
}

func TestServeOrLoad_LoaderErrorPropagates(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	want := errors.New("db boom")
	loader := func(ctx context.Context) (map[string]interface{}, []string, error) {
		return nil, nil, want
	}
	_, err := ServeOrLoad(ctx, ds, "select error", "public.", loader, Options{})
	if err == nil || err.Error() != want.Error() {
		t.Fatalf("expected loader error, got %v", err)
	}
}

func TestServeOrLoad_BypassWhenDisabled(t *testing.T) {
	ctx := context.Background()
	ds := &module_model.DataSource{ProjectId: "p1", DbAlias: "x"}
	loader := func(ctx context.Context) (map[string]interface{}, []string, error) {
		return map[string]interface{}{"v": 1}, nil, nil
	}
	res, err := ServeOrLoad(ctx, ds, "select 1", "public.", loader, Options{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res["v"] != 1 {
		t.Fatalf("loader result not returned: %v", res)
	}
}

func TestHasVolatileTable(t *testing.T) {
	if !hasVolatileTable([]string{"orders", "sessions"}, []string{"sessions"}) {
		t.Fatalf("expected match")
	}
	if hasVolatileTable([]string{"orders"}, []string{"sessions"}) {
		t.Fatalf("unexpected match")
	}
	if hasVolatileTable(nil, []string{"sessions"}) {
		t.Fatalf("nil tables should not match")
	}
}
