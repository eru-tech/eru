package cache

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCacheDataWriteShapeHasNoDerivedColumns(t *testing.T) {
	row := CacheData{
		CacheKey:     "conv-1",
		CacheValue:   "{}",
		ProjectId:    "processo",
		TenantId:     "t1",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		ExpiresAt:    time.Now(),
		LastAccessed: time.Now(),
		CreatedBy:    "u1",
		AgentName:    "eru_studio",
	}

	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, derived := range []string{"last_updated"} {
		if strings.Contains(string(b), derived) {
			t.Fatalf("%q is derived by the list query and is not a column of eru_cache; "+
				"including it makes every insert fail with "+
				"'column \"%s\" of relation \"eru_cache\" does not exist': %s",
				derived, derived, b)
		}
	}
}

func TestCacheDataReadsDerivedLastUpdated(t *testing.T) {
	var row CacheData
	if err := json.Unmarshal([]byte(`{"cache_key":"c","last_updated":"2026-09-15T21:07:00Z"}`), &row); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if row.LastUpdated == nil {
		t.Fatal("last_updated from the list query was dropped on unmarshal")
	}
	if got := row.LastUpdated.UTC().Format(time.RFC3339); got != "2026-09-15T21:07:00Z" {
		t.Fatalf("last_updated = %s", got)
	}
}
