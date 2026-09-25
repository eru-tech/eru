package catalog

import "testing"

// The dashboard run bound every tile to a real column and still rendered a
// dash, because it filled the value path with the path it had used to reach
// that column in the query response.
func TestFieldPathCopiedFromTheQueryResult(t *testing.T) {
	page := map[string]interface{}{
		"id": "p1", "name": "dash", "styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id": "tile_iff_amt", "type": "tile",
				"styles": map[string]interface{}{},
				"properties": map[string]interface{}{
					"base": map[string]interface{}{
						"data_source":           "query",
						"query":                 "db_tiles_rff",
						"primary_value_field":   "iff_amt",
						"primary_value_path":    "0.iff_amt",
						"secondary_value_field": "iff_cnt",
						"secondary_value_path":  "0.iff_cnt",
					},
				},
			},
		},
	}
	var found int
	for _, issue := range Get().ValidatePage(page) {
		if issue.Code == CodeFieldPathIsRowPath {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected both value paths to be reported, got %d", found)
	}
}

// A path that digs into a JSON column is the reason the property exists.
func TestFieldPathIntoAJsonColumnIsAllowed(t *testing.T) {
	page := map[string]interface{}{
		"id": "p1", "name": "dash", "styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id": "tile_limit", "type": "tile",
				"styles": map[string]interface{}{},
				"properties": map[string]interface{}{
					"base": map[string]interface{}{
						"data_source":         "query",
						"query":               "db_cust_limit",
						"primary_value_field": "entity_data",
						"primary_value_path":  "limit.sanctioned",
					},
				},
			},
		},
	}
	for _, issue := range Get().ValidatePage(page) {
		if issue.Code == CodeFieldPathIsRowPath {
			t.Fatalf("a path into a JSON column must be allowed: %s", issue.Message)
		}
	}
}
