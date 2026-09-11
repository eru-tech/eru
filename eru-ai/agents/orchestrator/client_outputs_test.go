package orchestrator

import (
	"encoding/json"
	"testing"
)

// A step can answer with more than one action. The Eru Studio agent returns the
// page it was asked about plus one action per nested page it had to create, and
// each of those is a page the user saves separately. Keeping only actions[0]
// discarded them one hop before the client.

func envelope(t *testing.T, raw string) interface{} {
	t.Helper()
	var body interface{}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestExtractStructuredOutputsForwardsEveryAction(t *testing.T) {
	body := envelope(t, `{
	  "actions": [
	    {"action_type": "answer", "action_name": "eru_studio",
	     "action": {"role": "root", "mode": "patch", "page_id": "invoice_page"}},
	    {"action_type": "answer", "action_name": "eru_studio",
	     "action": {"role": "nested", "mode": "full", "page_id": "line_item_tpl", "mounted_at": "items_ref"}},
	    {"action_type": "answer", "action_name": "eru_studio",
	     "action": {"role": "nested", "mode": "full", "page_id": "detail_tpl", "mounted_at": "panel_ref"}}
	  ]
	}`)

	outputs := extractStructuredOutputs(body)
	if len(outputs) != 3 {
		t.Fatalf("forwarded %d of 3 actions - the nested pages are being discarded", len(outputs))
	}
	if outputs[0]["role"] != "root" {
		t.Errorf("the root page is not first: %v", outputs[0]["role"])
	}
	for _, i := range []int{1, 2} {
		if outputs[i]["role"] != "nested" {
			t.Errorf("output %d is not a nested page: %v", i, outputs[i])
		}
	}
	if outputs[1]["page_id"] != "line_item_tpl" || outputs[2]["page_id"] != "detail_tpl" {
		t.Errorf("nested page identity was lost: %v, %v", outputs[1]["page_id"], outputs[2]["page_id"])
	}
}

func TestExtractStructuredOutputsKeepsTheSingleActionCaseUnchanged(t *testing.T) {
	body := envelope(t, `{"actions": [{"action_type": "answer", "action": {"id": "page_1", "components": []}}]}`)
	outputs := extractStructuredOutputs(body)
	if len(outputs) != 1 {
		t.Fatalf("expected one output, got %d", len(outputs))
	}
	if outputs[0]["id"] != "page_1" {
		t.Errorf("the action payload was not unwrapped: %v", outputs[0])
	}
}

func TestExtractStructuredOutputsPassesToolResultsThrough(t *testing.T) {
	// A tool result has no actions envelope; it is already the payload.
	body := envelope(t, `{"rows": [{"id": 1}], "count": 1}`)
	outputs := extractStructuredOutputs(body)
	if len(outputs) != 1 {
		t.Fatalf("expected one output, got %d", len(outputs))
	}
	if outputs[0]["count"] != float64(1) {
		t.Errorf("the tool result was altered: %v", outputs[0])
	}
}

func TestExtractStructuredOutputsHandlesUnusableBodies(t *testing.T) {
	cases := []struct {
		name string
		body interface{}
		want func([]map[string]interface{}) bool
	}{
		{
			name: "not an object at all",
			body: "just a string",
			want: func(out []map[string]interface{}) bool { return len(out) == 1 && out[0]["output"] == "just a string" },
		},
		{
			name: "an empty actions array",
			body: map[string]interface{}{"actions": []interface{}{}},
			want: func(out []map[string]interface{}) bool { return len(out) == 1 },
		},
		{
			name: "actions that carry no action object",
			body: map[string]interface{}{"actions": []interface{}{map[string]interface{}{"action_type": "answer"}}},
			want: func(out []map[string]interface{}) bool { return len(out) == 1 },
		},
		{
			name: "actions that is not an array",
			body: map[string]interface{}{"actions": "nonsense"},
			want: func(out []map[string]interface{}) bool { return len(out) == 1 },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Forwarding the body is right where the shape is unusable: the client
			// can see what arrived, instead of the step looking like it returned
			// nothing.
			if got := extractStructuredOutputs(tc.body); !tc.want(got) {
				t.Errorf("got %v", got)
			}
		})
	}
}

func TestExtractStructuredOutputsSkipsMalformedEntriesButKeepsTheRest(t *testing.T) {
	body := envelope(t, `{
	  "actions": [
	    {"action": {"role": "root"}},
	    "not an action",
	    {"action_type": "answer"},
	    {"action": {"role": "nested", "page_id": "tpl"}}
	  ]
	}`)
	outputs := extractStructuredOutputs(body)
	if len(outputs) != 2 {
		t.Fatalf("expected the two usable actions, got %d: %v", len(outputs), outputs)
	}
	if outputs[0]["role"] != "root" || outputs[1]["page_id"] != "tpl" {
		t.Errorf("outputs = %v", outputs)
	}
}
