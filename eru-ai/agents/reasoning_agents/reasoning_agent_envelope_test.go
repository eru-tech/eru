package reasoning_agents

import (
	"testing"

	eru_models "github.com/eru-tech/eru/eru-models"
)

func envelopeSchema() eru_models.JSONSchema {
	return eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"summary":  {Type: "string"},
			"entities": {Type: "array"},
			"fields":   {Type: "array"},
		},
		Required: []string{"summary"},
	}
}

// The model sometimes wraps an otherwise correct answer in {"output": ...}.
// Undoing it costs nothing; leaving it costs a retry that can provoke a
// fabricated report.
func TestUnwrapsAnOutputEnvelope(t *testing.T) {
	inner := map[string]interface{}{"summary": "added one field", "fields": []interface{}{}}
	got := unwrapOutputEnvelope(map[string]interface{}{"output": inner}, envelopeSchema())

	if _, wrapped := got["output"]; wrapped {
		t.Fatalf("the envelope should have been removed, got %v", got)
	}
	if got["summary"] != "added one field" {
		t.Errorf("the inner answer should survive unwrapping, got %v", got)
	}
}

func TestLeavesAWellShapedAnswerAlone(t *testing.T) {
	answer := map[string]interface{}{"summary": "added one field"}
	if got := unwrapOutputEnvelope(answer, envelopeSchema()); got["summary"] != "added one field" {
		t.Errorf("a flat answer should pass through untouched, got %v", got)
	}
}

// Only an unambiguous envelope is unwrapped: anything that could be a real
// answer is left for validation to judge.
func TestDoesNotUnwrapWhatMightBeAnAnswer(t *testing.T) {
	schema := envelopeSchema()

	cases := map[string]map[string]interface{}{
		"a declared key holding an object": {"entities": map[string]interface{}{"summary": "x"}},
		"an inner object missing required": {"output": map[string]interface{}{"fields": []interface{}{}}},
		"more than one top-level key":      {"output": map[string]interface{}{"summary": "x"}, "extra": 1},
		"a wrapper that is not an object":  {"output": "summary"},
	}
	for name, in := range cases {
		got := unwrapOutputEnvelope(in, schema)
		if len(got) != len(in) {
			t.Errorf("%s: should have been left alone, got %v", name, got)
		}
	}
}
