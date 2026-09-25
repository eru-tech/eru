package utiltiy

import "testing"

// run_query walks its OWN return envelope, which wraps the query response under
// "result". A component is handed the response itself. Reporting the envelope
// path told pages to look one level too deep: they walked into nothing and
// rendered blank while the query answered 200 with rows in it.
//
// This is the defect that produced `query_result_path: "result.0.Results"` on a
// generated dashboard - the agent was not guessing, it was using what the tool
// reported.
func TestLocateRowsPathIsStrippedToTheComponentFrame(t *testing.T) {
	// What the delegate hands back: the eru-ql answer under the tool's own key.
	toolResult := map[string]interface{}{
		"result": []interface{}{
			map[string]interface{}{"Results": []interface{}{
				map[string]interface{}{"iff_amt": 519000, "iff_cnt": 122},
			}},
		},
	}

	path, rows := locateRows(toolResult)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if path != "result.0.Results" {
		t.Fatalf("the envelope path should be %q, got %q", "result.0.Results", path)
	}

	// What a page must be told: the same rows, addressed from the response.
	component := stripToolEnvelope(path)
	if component != "0.Results" {
		t.Errorf("component path = %q, want %q", component, "0.Results")
	}
}

// A response that is already a bare array has no envelope to strip.
func TestBareArrayNeedsNoPath(t *testing.T) {
	toolResult := map[string]interface{}{
		"result": []interface{}{
			map[string]interface{}{"a": 1},
		},
	}
	path, rows := locateRows(toolResult)
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if got := stripToolEnvelope(path); got != "0" && got != "" {
		t.Errorf("component path = %q, want the array index or nothing", got)
	}
}
