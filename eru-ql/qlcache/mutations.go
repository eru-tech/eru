package qlcache

import (
	"strings"
)

// QualifyTables prefixes any table lacking a schema with schemaPrefix.
func QualifyTables(tables []string, schemaPrefix string) []string {
	out := make([]string, 0, len(tables))
	for _, t := range tables {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !strings.Contains(t, ".") && schemaPrefix != "" {
			t = schemaPrefix + t
		}
		out = append(out, t)
	}
	return out
}

// IsDML reports whether a SQL statement looks like a data-modification
// statement. Used by the raw-SQL path to decide whether to enqueue an
// invalidation after a successful execute. Deliberately conservative:
// false positives trigger an unnecessary Redis call; false negatives leave
// the cache stale until TTL.
func IsDML(sql string) bool {
	trimmed := strings.TrimLeft(sql, " \t\n\r(")
	trimmed = strings.ToLower(trimmed)
	prefixes := []string{"insert", "update", "delete", "truncate", "merge", "upsert", "replace"}
	for _, p := range prefixes {
		if strings.HasPrefix(trimmed, p) {
			return true
		}
	}
	return false
}
