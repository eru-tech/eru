package agents

import "testing"

func TestOneStepAnswerIsUsedAsIs(t *testing.T) {
	out, err := decodeFuncGroupResponse([]byte(`{"sql":"select 1"}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["sql"] != "select 1" {
		t.Fatalf("got %v", out)
	}
}

func TestSeveralTopLevelStepsDoNotFailTheRun(t *testing.T) {
	// A plan with two top-level branches answers with an array. Rejecting it
	// failed a run that had succeeded, and cost a replan.
	out, err := decodeFuncGroupResponse([]byte(`[{"sql":"select 1"},{"page":{"id":"p1"}}]`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	results, ok := out["results"].([]interface{})
	if !ok || len(results) != 2 {
		t.Fatalf("got %v", out)
	}
}

func TestASingleElementArrayIsUnwrapped(t *testing.T) {
	out, err := decodeFuncGroupResponse([]byte(`[{"sql":"select 1"}]`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["sql"] != "select 1" {
		t.Fatalf("got %v", out)
	}
}

func TestAScalarBodyIsCarriedRatherThanRejected(t *testing.T) {
	out, err := decodeFuncGroupResponse([]byte(`"done"`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["result"] != "done" {
		t.Fatalf("got %v", out)
	}
}

func TestInvalidJsonIsStillAnError(t *testing.T) {
	if _, err := decodeFuncGroupResponse([]byte(`{oops`)); err == nil {
		t.Fatal("accepted invalid json")
	}
}
