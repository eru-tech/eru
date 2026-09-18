package utiltiy

import "testing"

func rowsFrom(t *testing.T, result map[string]interface{}) []map[string]interface{} {
	t.Helper()
	return extractRows(result)
}

func TestTheEruQlEnvelopeIsLookedThrough(t *testing.T) {
	// The shape a processo page lookup actually returns.
	result := map[string]interface{}{"result": []interface{}{
		map[string]interface{}{"Results": []interface{}{
			map[string]interface{}{"page_id": "p1", "page_name": "invoice_360_detail"},
			map[string]interface{}{"page_id": "p2", "page_name": "er_dashboard"},
		}},
	}}
	rows := rowsFrom(t, result)
	if len(rows) != 2 {
		t.Fatalf("got %d row(s): %v", len(rows), rows)
	}
	if rows[0]["page_name"] != "invoice_360_detail" {
		t.Fatalf("wrong row: %v", rows[0])
	}
}

func TestAnEmptyEnvelopeIsNoRowsRatherThanOne(t *testing.T) {
	result := map[string]interface{}{"result": []interface{}{
		map[string]interface{}{"Results": []interface{}{}},
	}}
	if rows := rowsFrom(t, result); len(rows) != 0 {
		t.Fatalf("an empty result produced %d row(s): %v", len(rows), rows)
	}
}

func TestPlainRowsAreUntouched(t *testing.T) {
	result := map[string]interface{}{"rows": []interface{}{
		map[string]interface{}{"entity_name": "er", "table_name": "exchange_rates"},
	}}
	rows := rowsFrom(t, result)
	if len(rows) != 1 || rows[0]["entity_name"] != "er" {
		t.Fatalf("got %v", rows)
	}
}

func TestARowCarryingItsOwnListIsNotUnwrapped(t *testing.T) {
	// A record with nested line items is a row, not an envelope.
	result := map[string]interface{}{"result": []interface{}{
		map[string]interface{}{"fields": []interface{}{
			map[string]interface{}{"field_code": "fc"},
		}, "entity_name": "er"},
	}}
	rows := rowsFrom(t, result)
	if len(rows) != 1 || rows[0]["entity_name"] != "er" {
		t.Fatalf("a real row was unwrapped: %v", rows)
	}
}
