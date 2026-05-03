package module_model

import (
	"encoding/json"
	"os"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("eru-ql-test", "test-instance")
	os.Exit(m.Run())
}

// Existing DataSource payloads (from disk / from Postgres store rows written
// before cache support was added) must deserialize unchanged.
func TestDataSource_UnmarshalJSON_NoCacheKeysBackwardCompat(t *testing.T) {
	legacy := []byte(`{
		"db_alias": "primary",
		"db_type": "SQL",
		"db_name": "postgres",
		"con_status": true
	}`)
	var ds DataSource
	if err := json.Unmarshal(legacy, &ds); err != nil {
		t.Fatalf("legacy JSON should deserialize: %v", err)
	}
	if ds.DbAlias != "primary" {
		t.Fatalf("db_alias lost: %q", ds.DbAlias)
	}
	if ds.QueryCache != nil {
		t.Fatalf("QueryCache should be nil for legacy JSON, got %#v", ds.QueryCache)
	}
	if ds.QueryCacheConfig.Enabled {
		t.Fatalf("QueryCacheConfig.Enabled should be false for legacy JSON")
	}
}

func TestDataSource_UnmarshalJSON_WithQueryCacheConfigOnly(t *testing.T) {
	payload := []byte(`{
		"db_alias": "primary",
		"db_type": "SQL",
		"db_name": "postgres",
		"query_cache_config": {
			"enabled": true,
			"default_ttl_sec": 300,
			"max_value_bytes": 1048576,
			"volatile_tables": ["audit_log"],
			"lock_hot_queries": true
		}
	}`)
	var ds DataSource
	if err := json.Unmarshal(payload, &ds); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !ds.QueryCacheConfig.Enabled {
		t.Fatalf("Enabled not parsed")
	}
	if ds.QueryCacheConfig.DefaultTTLSec != 300 {
		t.Fatalf("DefaultTTLSec=%d", ds.QueryCacheConfig.DefaultTTLSec)
	}
	if len(ds.QueryCacheConfig.VolatileTables) != 1 || ds.QueryCacheConfig.VolatileTables[0] != "audit_log" {
		t.Fatalf("VolatileTables=%v", ds.QueryCacheConfig.VolatileTables)
	}
	if !ds.QueryCacheConfig.LockHotQueries {
		t.Fatalf("LockHotQueries not parsed")
	}
	if ds.QueryCache != nil {
		t.Fatalf("QueryCache should still be nil when only config provided")
	}
}

// When query_cache.cache_store_type is unknown, we must not crash; ds.QueryCache
// should end up nil and no error should bubble up so existing flows keep working.
func TestDataSource_UnmarshalJSON_UnknownCacheType(t *testing.T) {
	payload := []byte(`{
		"db_alias": "primary",
		"db_type": "SQL",
		"db_name": "postgres",
		"query_cache": {"cache_store_type": "NOT_A_REAL_BACKEND"}
	}`)
	var ds DataSource
	if err := json.Unmarshal(payload, &ds); err != nil {
		t.Fatalf("unmarshal should not error on unknown cache type: %v", err)
	}
	if ds.QueryCache != nil {
		t.Fatalf("QueryCache should be nil for unknown backend")
	}
}
