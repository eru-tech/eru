package qlcache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/eru-tech/eru/eru-cache/cache"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_model"
)

func TestMain(m *testing.M) {
	logs.LogInit("qlcache-test", "test-instance")
	os.Exit(m.Run())
}

func newEnabledDS() *module_model.DataSource {
	ds := &module_model.DataSource{ProjectId: "p1", DbAlias: "primary"}
	ds.QueryCache = cache.NewInMemoryCache()
	ds.QueryCacheClone = ds.QueryCache
	ds.QueryCacheConfig.Enabled = true
	ds.QueryCacheConfig.DefaultTTLSec = 300
	return ds
}

func waitForPopulate(ds *module_model.DataSource, key string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := ds.QueryCache.Get(context.Background(), key)
		if err == nil {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func TestNormalizeSQL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"select * from t", "select * from t"},
		{"  select   *\tfrom\n t  ", "select * from t"},
		{"select\n*\nfrom\nt", "select * from t"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeSQL(c.in); got != c.want {
			t.Errorf("NormalizeSQL(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestBuildKey_Determinism(t *testing.T) {
	if BuildKey("p1", "", "primary", "select * from t") != BuildKey("p1", "", "primary", "select  *   from  t") {
		t.Fatalf("whitespace variations should collide")
	}
}

func TestBuildKey_DsAliasSeparation(t *testing.T) {
	sql := "select * from users"
	if BuildKey("p1", "", "primary", sql) == BuildKey("p1", "", "analytics", sql) {
		t.Fatalf("different datasources must produce different keys")
	}
}

func TestBuildKey_ProjectSeparation(t *testing.T) {
	sql := "select * from users"
	if BuildKey("pA", "", "primary", sql) == BuildKey("pB", "", "primary", sql) {
		t.Fatalf("different projects must produce different keys (same ds alias, same SQL)")
	}
}

func TestBuildKey_TenantSeparation(t *testing.T) {
	sql := "select * from users"
	if BuildKey("p1", "t1", "primary", sql) == BuildKey("p1", "t2", "primary", sql) {
		t.Fatalf("different tenants must produce different keys")
	}
	if BuildKey("p1", "t1", "primary", sql) == BuildKey("p1", "", "primary", sql) {
		t.Fatalf("tenant-scoped key must differ from project-level key")
	}
	if BuildKey("p1", "", "primary", sql) != BuildKey("p1", "", "primary", sql) {
		t.Fatalf("project-level key must be stable")
	}
}

func TestServe_BypassWhenNoCache(t *testing.T) {
	ds := &module_model.DataSource{DbAlias: "x"}
	ds.QueryCacheConfig.Enabled = true
	hit, _, _ := Serve(context.Background(), ds, "", "select 1")
	if hit {
		t.Fatalf("expected miss/bypass when QueryCache nil")
	}
}

func TestServe_BypassWhenDisabled(t *testing.T) {
	ds := &module_model.DataSource{DbAlias: "x"}
	ds.QueryCache = cache.NewInMemoryCache()
	ds.QueryCacheClone = ds.QueryCache
	ds.QueryCacheConfig.Enabled = false
	hit, _, key := Serve(context.Background(), ds, "", "select 1")
	if hit || key != "" {
		t.Fatalf("expected bypass with empty key when disabled")
	}
}

func TestServe_NilDataSource(t *testing.T) {
	hit, _, _ := Serve(context.Background(), nil, "", "select 1")
	if hit {
		t.Fatalf("nil ds should not hit")
	}
}

func TestPopulateAndServe_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	sql := "select id, name from users where id=1"

	// first Serve: miss with non-empty key
	hit, _, key := Serve(ctx, ds, "", sql)
	if hit {
		t.Fatalf("first call should miss")
	}
	if key == "" {
		t.Fatalf("expected key on miss")
	}

	result := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"id": float64(1), "name": "alice"},
		},
	}
	Populate(ctx, ds, key, result, []string{"primary::public.users"})
	if !waitForPopulate(ds, key, 200*time.Millisecond) {
		t.Fatalf("populate did not land in cache")
	}

	hit2, got, key2 := Serve(ctx, ds, "", sql)
	if !hit2 {
		t.Fatalf("expected hit after populate")
	}
	if key2 != key {
		t.Fatalf("key drift: %s vs %s", key, key2)
	}
	users, _ := got["users"].([]interface{})
	if len(users) != 1 {
		t.Fatalf("round-tripped payload is wrong: %#v", got)
	}
}

func TestPopulate_SkipsWhenOversize(t *testing.T) {
	ctx := context.Background()
	ds := newEnabledDS()
	ds.QueryCacheConfig.MaxValueBytes = 10 // tiny

	_, _, key := Serve(ctx, ds, "", "select 1")
	result := map[string]interface{}{
		"padding": "this string is definitely more than ten bytes long",
	}
	before := GetStats().Writes
	Populate(ctx, ds, key, result, nil)
	// wait briefly to let the goroutine run
	time.Sleep(50 * time.Millisecond)
	if GetStats().Writes != before {
		t.Fatalf("oversize payload should not increment Writes")
	}
	// cache should not contain the key
	if _, err := ds.QueryCache.Get(ctx, key); err == nil {
		t.Fatalf("oversize payload should not be cached")
	}
}

func TestPopulate_NoOpWhenDisabled(t *testing.T) {
	ds := newEnabledDS()
	ds.QueryCacheConfig.Enabled = false
	before := GetStats().Writes
	Populate(context.Background(), ds, "some-key", map[string]interface{}{"x": 1}, nil)
	time.Sleep(30 * time.Millisecond)
	if GetStats().Writes != before {
		t.Fatalf("Populate should not write when cache disabled")
	}
}

func TestTablesToTags(t *testing.T) {
	tables := module_model.TablesInQuery{
		Tables: []module_model.TableInQuery{
			{TableName: "orders"},
			{TableName: "public.users"},
			{TableName: "orders"}, // duplicate
			{TableName: ""},
		},
	}
	got := TablesToTags("p1", "primary", tables, "public.")
	if len(got) != 2 {
		t.Fatalf("expected 2 unique tags, got %v", got)
	}
	want := map[string]bool{
		"p1::primary::public.orders": true,
		"p1::primary::public.users":  true,
	}
	for _, tag := range got {
		if !want[tag] {
			t.Errorf("unexpected tag %q", tag)
		}
	}
}
