package catalog

import (
	"strings"
	"testing"
)

// An empty query_result_path lets the runtime unwrap the response by itself. A
// wrong one replaces that with a literal walk into nothing, so the component
// renders blank while its query returns 200 with rows - which is what happened
// to every chart and grid on the first generated dashboard.
func TestQueryResultPathRejectsAnInventedPrefix(t *testing.T) {
	issues := Get().ValidatePage(map[string]interface{}{
		"id": "p", "title": "P",
		"components": []interface{}{
			map[string]interface{}{
				"id": "c1", "type": "line_chart",
				"properties": map[string]interface{}{"base": map[string]interface{}{
					"query":             "db_disb",
					"query_result_path": "result.0.Results",
				}},
			},
		},
	})
	var found bool
	for _, i := range issues {
		if i.Code == CodeQueryResultPathWrong {
			found = true
			if !strings.Contains(i.Message, "0.Results") {
				t.Errorf("the message should show the right shape: %q", i.Message)
			}
		}
	}
	if !found {
		t.Errorf("result.0.Results should be rejected; got %v", issues)
	}
}

func TestQueryResultPathAcceptsTheEruqlShapeAndEmpty(t *testing.T) {
	for _, path := range []string{"", "0.Results", "Results", "0"} {
		issues := Get().ValidatePage(map[string]interface{}{
			"id": "p", "title": "P",
			"components": []interface{}{
				map[string]interface{}{
					"id": "c1", "type": "line_chart",
					"properties": map[string]interface{}{"base": map[string]interface{}{
						"query":             "db_disb",
						"query_result_path": path,
					}},
				},
			},
		})
		for _, i := range issues {
			if i.Code == CodeQueryResultPathWrong {
				t.Errorf("path %q should be accepted, got %q", path, i.Message)
			}
		}
	}
}
