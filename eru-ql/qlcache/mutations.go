package qlcache

import (
	"strings"
)

// QualifyTables prefixes any table lacking a schema with schemaPrefix and
// normalizes identifier case via NormalizeTableName.
func QualifyTables(tables []string, schemaPrefix string) []string {
	out := make([]string, 0, len(tables))
	for _, t := range tables {
		t = NormalizeTableName(t)
		if t == "" {
			continue
		}
		if !strings.Contains(t, ".") && schemaPrefix != "" {
			t = NormalizeTableName(schemaPrefix) + t
		}
		out = append(out, t)
	}
	return out
}

// IsDML reports whether a SQL statement writes (DML, DDL, DCL, or admin) or
// is a writable CTE wrapping a write. Used for cache invalidation and to route
// the standalone SQL endpoint to the writer connection. Deliberately conservative:
// false positives mean an unnecessary main-routing or Redis call; false
// negatives mean a write may hit a read replica and fail with read-only txn.
func IsDML(sql string) bool {
	s := stripLeadingNoise(sql)
	if s == "" {
		return false
	}
	end := strings.IndexAny(s, " \t\n\r(")
	if end == -1 {
		end = len(s)
	}
	switch s[:end] {
	case "insert", "update", "delete", "truncate", "merge", "upsert", "replace",
		"create", "alter", "drop", "grant", "revoke", "copy", "call", "do",
		"vacuum", "analyze", "cluster", "lock", "reindex", "refresh":
		return true
	case "with":
		return cteHasWrite(s)
	}
	return false
}

// stripLeadingNoise drops leading whitespace, opening parens, line comments
// (-- to EOL) and block comments (/* ... */) repeatedly, then lowercases.
// Returns "" if everything got stripped.
func stripLeadingNoise(sql string) string {
	s := sql
	for {
		s = strings.TrimLeft(s, " \t\n\r(")
		if strings.HasPrefix(s, "--") {
			if i := strings.IndexAny(s, "\n\r"); i >= 0 {
				s = s[i+1:]
				continue
			}
			return ""
		}
		if strings.HasPrefix(s, "/*") {
			if i := strings.Index(s, "*/"); i >= 0 {
				s = s[i+2:]
				continue
			}
			return ""
		}
		return strings.ToLower(s)
	}
}

// cteHasWrite scans a lowercased WITH-prefixed statement for a writable inner
// statement. Substring match is conservative on purpose: a CTE that selects
// `updated_at` will be classified as write and forced to main, which is safe.
// Only WITH-prefixed queries reach here, so the over-match surface is narrow.
func cteHasWrite(s string) bool {
	return strings.Contains(s, "insert") || strings.Contains(s, "update") ||
		strings.Contains(s, "delete") || strings.Contains(s, "merge")
}
