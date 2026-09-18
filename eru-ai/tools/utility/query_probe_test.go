package utiltiy

import (
	"encoding/json"
	"strings"
	"testing"
)

func decode(t *testing.T, raw string) interface{} {
	t.Helper()
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return value
}

func TestResultPathIsDerivedNotAssumed(t *testing.T) {
	cases := []struct {
		name     string
		response string
		path     string
		columns  string
	}{
		{
			name:     "eru-ql sql envelope",
			response: `[{"Results":[{"latest_rate":95.2,"chart_date":"2026-09-17"}]}]`,
			path:     "0.Results",
			columns:  "chart_date,latest_rate",
		},
		{
			name:     "bare array of rows",
			response: `[{"a":1},{"a":2}]`,
			path:     "",
			columns:  "a",
		},
		{
			name:     "named object",
			response: `{"rows":[{"b":1}]}`,
			path:     "rows",
			columns:  "b",
		},
		{
			name:     "nested under a graphql-style key",
			response: `{"data":{"invoices":[{"c":1}]}}`,
			path:     "data.invoices",
			columns:  "c",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, rows := locateRows(decode(t, tc.response))
			if path != tc.path {
				t.Errorf("result_path = %q, want %q", path, tc.path)
			}
			if len(rows) == 0 {
				t.Fatal("no rows found")
			}
			if got := strings.Join(rowKeys(rows[0]), ","); got != tc.columns {
				t.Errorf("columns = %q, want %q", got, tc.columns)
			}
		})
	}
}

func TestAnEmptyResultYieldsNoRows(t *testing.T) {
	for _, raw := range []string{`[]`, `[{"Results":[]}]`, `{}`, `null`} {
		if _, rows := locateRows(decode(t, raw)); len(rows) != 0 {
			t.Errorf("%s produced rows", raw)
		}
	}
}

func TestScalarArraysAreNotMistakenForRows(t *testing.T) {
	// A column of strings is not a row set; reporting it as one would hand the
	// page a path that resolves to values it cannot read fields out of.
	if path, rows := locateRows(decode(t, `{"names":["a","b"],"rows":[{"id":1}]}`)); path != "rows" || len(rows) != 1 {
		t.Fatalf("path = %q with %d row(s)", path, len(rows))
	}
}

func TestDeclaredVarsAreFoundInsideTheFetchEnvelope(t *testing.T) {
	// The definition is recognised by its own keys, so the envelope around it
	// can change without this going quiet.
	for _, raw := range []string{
		`{"myquery":{"query_name":"q","vars":{"fc":"","tc":""}}}`,
		`{"result":{"myquery":{"query_name":"q","vars":{"fc":"","tc":""}}}}`,
		`[{"myquery":{"query_name":"q","vars":{"fc":"","tc":""}}}]`,
	} {
		vars := findDeclaredVars(decode(t, raw))
		if len(vars) != 2 {
			t.Fatalf("%s -> %v", raw, vars)
		}
		if _, ok := vars["fc"]; !ok {
			t.Fatalf("%s did not yield fc: %v", raw, vars)
		}
	}
}

func TestAVarsMapThatIsNotAQueryDefinitionIsIgnored(t *testing.T) {
	// A row that happens to have a "vars" column must not be mistaken for the
	// query definition.
	if vars := findDeclaredVars(decode(t, `{"Results":[{"vars":{"a":1}}]}`)); vars != nil {
		t.Fatalf("got %v", vars)
	}
}

func TestNoDefinitionYieldsNothing(t *testing.T) {
	if vars := findDeclaredVars(decode(t, `{"error":"nope"}`)); vars != nil {
		t.Fatalf("got %v", vars)
	}
}
