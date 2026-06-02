package agents

import (
	"strings"
	"testing"
)

func sampleQuestionRequest() ClarificationRequest {
	return ClarificationRequest{
		Prompt: "Need a bit more detail",
		Questions: []ClarificationQuestion{
			{
				Id:       "q1",
				Question: "Which output format?",
				Options:  []QuestionOption{{Value: "json", Label: "JSON"}, {Value: "csv", Label: "CSV"}},
			},
		},
	}
}

func TestToActionAndParseRoundTrip(t *testing.T) {
	req := sampleQuestionRequest()
	action := req.ToAction("agentA")
	if action.ActionType != ActionTypeQuestion {
		t.Fatalf("expected action_type %q, got %q", ActionTypeQuestion, action.ActionType)
	}
	parsed, err := ParseClarificationRequest(action.Action)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(parsed.Questions) != 1 || parsed.Questions[0].Id != "q1" {
		t.Fatalf("round trip lost data: %+v", parsed)
	}
}

func TestPendingQuestionFindsLastAssistantQuestion(t *testing.T) {
	conv := &Conversation{
		Messages: []AgentMessage{
			{Role: "user", Content: "do something"},
			{Role: "assistant", Actions: []AgentOutputAction{sampleQuestionRequest().ToAction("agentA")}},
		},
	}
	msg, action, ok := PendingQuestion(conv)
	if !ok {
		t.Fatal("expected a pending question")
	}
	if action.ActionType != ActionTypeQuestion || msg.Role != "assistant" {
		t.Fatalf("unexpected pending question: %+v", action)
	}
}

func TestPendingQuestionStopsAtUserTurn(t *testing.T) {
	conv := &Conversation{
		Messages: []AgentMessage{
			{Role: "assistant", Actions: []AgentOutputAction{sampleQuestionRequest().ToAction("agentA")}},
			{Role: "user", Content: "answered"},
		},
	}
	if _, _, ok := PendingQuestion(conv); ok {
		t.Fatal("answered question should not be pending")
	}
}

func TestClarificationAnswersFromParams(t *testing.T) {
	msg := AgentMessage{
		Params: map[string]interface{}{
			ClarificationAnswersParamKey: []map[string]interface{}{
				{"question_id": "q1", "selected": []string{"json"}},
			},
		},
	}
	answers, ok := msg.ClarificationAnswers()
	if !ok || len(answers) != 1 || answers[0].QuestionId != "q1" {
		t.Fatalf("failed to parse answers: %+v ok=%v", answers, ok)
	}
}

func TestFormatAnswersForModelIncludesQuestionAndAnswer(t *testing.T) {
	req := sampleQuestionRequest()
	answers := []ClarificationAnswer{{QuestionId: "q1", Selected: []string{"json"}}}
	out := FormatAnswersForModel(req, answers)
	if !strings.Contains(out, "Which output format?") || !strings.Contains(out, "json") {
		t.Fatalf("formatted text missing content: %q", out)
	}
}
