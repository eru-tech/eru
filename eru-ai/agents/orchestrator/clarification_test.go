package orchestrator

import (
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	functions "github.com/eru-tech/eru/eru-functions/functions"
)

func TestCollectQuestionsFindsSubAgentQuestion(t *testing.T) {
	result := map[string]interface{}{
		"extractor": map[string]interface{}{
			"actions": []interface{}{
				map[string]interface{}{
					"action_type": agents.ActionTypeQuestion,
					"action_name": "extractor",
					"action": map[string]interface{}{
						"questions": []interface{}{
							map[string]interface{}{"id": "q1", "question": "Which sheet?"},
						},
					},
				},
			},
		},
	}
	var tagged []taggedQuestion
	collectQuestions(result, "", &tagged)
	if len(tagged) != 1 {
		t.Fatalf("expected 1 question, got %d", len(tagged))
	}
	if len(tagged[0].Request.Questions) != 1 || tagged[0].Request.Questions[0].Question != "Which sheet?" {
		t.Fatalf("unexpected question: %+v", tagged[0])
	}
}

func TestMergeQuestionsPrefixesStep(t *testing.T) {
	tagged := []taggedQuestion{
		{Step: "extractor.actions", Request: agents.ClarificationRequest{Questions: []agents.ClarificationQuestion{{Id: "q1", Question: "Q?"}}}},
	}
	merged := mergeQuestions(tagged)
	if len(merged.Questions) != 1 || merged.Questions[0].Id != "extractor.actions::q1" {
		t.Fatalf("expected step-prefixed id, got %+v", merged.Questions)
	}
}

func TestAssignStableConversationIds(t *testing.T) {
	plan := map[string]interface{}{
		"func_steps": map[string]interface{}{
			"extractor": map[string]interface{}{
				"agent_name": "extractor",
				"tenant_id":  "t1",
				"func_steps": map[string]interface{}{
					"summarizer": map[string]interface{}{
						"agent_name": "summarizer",
						"tenant_id":  "t1",
					},
				},
			},
		},
	}
	assignStableConversationIds(plan, "conv123")

	steps := plan["func_steps"].(map[string]interface{})
	extractor := steps["extractor"].(map[string]interface{})
	if extractor["conversation_id"] != "conv123::extractor" {
		t.Fatalf("extractor conversation_id = %v", extractor["conversation_id"])
	}
	nested := extractor["func_steps"].(map[string]interface{})
	summarizer := nested["summarizer"].(map[string]interface{})
	if summarizer["conversation_id"] != "conv123::summarizer" {
		t.Fatalf("summarizer conversation_id = %v", summarizer["conversation_id"])
	}
}

func TestAssignStableConversationIdsPreservesExisting(t *testing.T) {
	plan := map[string]interface{}{
		"func_steps": map[string]interface{}{
			"extractor": map[string]interface{}{
				"agent_name":      "extractor",
				"conversation_id": "preset",
			},
		},
	}
	assignStableConversationIds(plan, "conv123")
	steps := plan["func_steps"].(map[string]interface{})
	extractor := steps["extractor"].(map[string]interface{})
	if extractor["conversation_id"] != "preset" {
		t.Fatalf("should preserve preset conversation_id, got %v", extractor["conversation_id"])
	}
}

func questionBody() map[string]interface{} {
	return map[string]interface{}{
		"actions": []interface{}{
			map[string]interface{}{
				"action_type": agents.ActionTypeQuestion,
				"action_name": "sentiment_analyzer",
				"action": map[string]interface{}{
					"questions": []interface{}{
						map[string]interface{}{"id": "q1", "question": "Which language?"},
					},
				},
			},
		},
	}
}

func TestDeriveJoinStep(t *testing.T) {
	plan := map[string]interface{}{
		"func_steps": map[string]interface{}{
			"sentiment_analyzer": map[string]interface{}{"agent_name": "sentiment_analyzer"},
			"topic_classifier": map[string]interface{}{
				"agent_name": "topic_classifier",
				"func_steps": map[string]interface{}{
					"report_generator": map[string]interface{}{"agent_name": "report_generator", "wait_for": "sentiment_analyzer"},
				},
			},
		},
	}
	if js := deriveJoinStep(plan); js != "report_generator" {
		t.Fatalf("expected join step report_generator, got %q", js)
	}
}

func TestBuildPendingResumeFromVarsParallel(t *testing.T) {
	plan := map[string]interface{}{
		"func_steps": map[string]interface{}{
			"sentiment_analyzer": map[string]interface{}{"agent_name": "sentiment_analyzer"},
			"topic_classifier":   map[string]interface{}{"agent_name": "topic_classifier"},
		},
	}
	resVars := map[string]*functions.TemplateVars{
		"sentiment_analyzer": {Body: questionBody()},
		"topic_classifier":   {Body: map[string]interface{}{"result": "done"}},
	}
	pr, merged, paused := buildPendingResumeFromVars(plan, resVars, "run1")
	if !paused {
		t.Fatal("expected paused")
	}
	if len(pr.PausedBranches) != 1 || pr.PausedBranches[0].StartStep != "sentiment_analyzer" {
		t.Fatalf("unexpected branches: %+v", pr.PausedBranches)
	}
	if pr.PausedBranches[0].EndStep != "sentiment_analyzer" {
		t.Fatalf("end step should equal start step, got %q", pr.PausedBranches[0].EndStep)
	}
	if len(merged.Questions) != 1 || merged.Questions[0].Id != "sentiment_analyzer::q1" {
		t.Fatalf("unexpected merged questions: %+v", merged.Questions)
	}
	if pr.ResVarsJSON == "" {
		t.Fatal("expected serialized res_vars")
	}
}

func TestVarsRoundTrip(t *testing.T) {
	in := map[string]*functions.TemplateVars{
		"a": {Body: map[string]interface{}{"x": "y"}},
	}
	s := marshalVars(in)
	out := unmarshalVars(s)
	body, ok := out["a"].Body.(map[string]interface{})
	if !ok || body["x"] != "y" {
		t.Fatalf("round trip failed: %+v", out)
	}
}

func TestResVarsToResult(t *testing.T) {
	resVars := map[string]*functions.TemplateVars{
		"step1": {Body: map[string]interface{}{"k": "v"}},
	}
	res := resVarsToResult(resVars)
	m, ok := res["step1"].(map[string]interface{})
	if !ok || m["k"] != "v" {
		t.Fatalf("unexpected result: %+v", res)
	}
}
