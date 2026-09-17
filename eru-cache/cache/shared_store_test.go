package cache

import (
	"context"
	"encoding/json"
	"testing"
)

func TestOneConfigurationIsOneStore(t *testing.T) {
	ctx := context.Background()
	cfg := json.RawMessage(`{"cache_store_type":"INMEMORY","cache_db_alias":"a1","persist_enabled":true}`)

	// An agent is rebuilt from its JSON on every request. If that produced a new
	// store each time, the store would start empty every time and could never
	// return a hit - which is a cache in name only.
	first, err := GetSharedCacheStore(ctx, "INMEMORY", &cfg)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := first.Set(ctx, "k", "v"); err != nil {
		t.Fatalf("set: %v", err)
	}

	second, err := GetSharedCacheStore(ctx, "INMEMORY", &cfg)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second {
		t.Fatal("the same configuration produced two stores, so nothing written by one request is visible to the next")
	}
	got, err := second.Get(ctx, "k")
	if err != nil || got != "v" {
		t.Fatalf("value written before the rebuild was lost: %q %v", got, err)
	}
}

func TestDifferentConfigurationsDoNotShareAStore(t *testing.T) {
	ctx := context.Background()
	one := json.RawMessage(`{"cache_store_type":"INMEMORY","cache_db_alias":"tenant-a"}`)
	two := json.RawMessage(`{"cache_store_type":"INMEMORY","cache_db_alias":"tenant-b"}`)

	a, _ := GetSharedCacheStore(ctx, "INMEMORY", &one)
	b, _ := GetSharedCacheStore(ctx, "INMEMORY", &two)
	if a == b {
		t.Fatal("two differently configured stores were collapsed into one")
	}
}

func TestVolatileFieldsDoNotSplitOneConfiguration(t *testing.T) {
	ctx := context.Background()
	clean := json.RawMessage(`{"cache_store_type":"INMEMORY","cache_db_alias":"a1"}`)
	// persist_error and cache_values are written by the store itself; a config
	// that has been through one round trip must still name the same store.
	dirty := json.RawMessage(`{"cache_store_type":"INMEMORY","cache_db_alias":"a1","persist_error":true,"cache_values":{}}`)

	a, _ := GetSharedCacheStore(ctx, "INMEMORY", &clean)
	b, _ := GetSharedCacheStore(ctx, "INMEMORY", &dirty)
	if a != b {
		t.Fatal("a store's own bookkeeping split one configuration into two caches")
	}
}

func TestSessionTtlIsPartOfTheStoreConfiguration(t *testing.T) {
	cs := CacheStore{SessionTtl: "30m"}
	if got := cs.SessionTtlOrDefault().String(); got != "30m0s" {
		t.Fatalf("session_ttl = %s", got)
	}
	empty := CacheStore{}
	if got := empty.SessionTtlOrDefault(); got != DefaultSessionTtl {
		t.Fatalf("default = %s", got)
	}
	bad := CacheStore{SessionTtl: "nonsense"}
	if got := bad.SessionTtlOrDefault(); got != DefaultSessionTtl {
		t.Fatalf("a bad duration should fall back to the default, got %s", got)
	}
}

func TestSweepReclaimsWhatLazyExpiryNeverWill(t *testing.T) {
	ctx := context.Background()
	cfg := json.RawMessage(`{"cache_store_type":"INMEMORY","cache_db_alias":"sweep-test"}`)
	store, _ := GetSharedCacheStore(ctx, "INMEMORY", &cfg)

	if err := store.SetWithTTL(ctx, "gone", "v", 1); err != nil {
		t.Fatalf("set: %v", err)
	}
	if purged := PurgeExpiredSharedStores(ctx); purged < 1 {
		t.Fatal("an expired entry nobody reads again is never reclaimed without the sweep")
	}
}
