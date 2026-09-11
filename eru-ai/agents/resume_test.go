package agents

import (
	"strings"
	"testing"

	models "github.com/eru-tech/eru/eru-ai/models"
)

func TestBuildResumeContextKeepsToolWorkAndSkipsTheQuestion(t *testing.T) {
	traces := []models.StepTrace{
		{
			Iteration: 1,
			Thinking:  "I need to know which components exist",
			ToolName:  "get_component_spec",
			ToolInput: map[string]interface{}{"types": []interface{}{"grid", "button"}},
			ToolResult: map[string]interface{}{
				"spec": "=== grid (Data Grid) ===",
			},
		},
		{
			Iteration: 2,
			ToolName:  models.TerminalToolAskUser,
			ToolInput: map[string]interface{}{"questions": []interface{}{}},
		},
	}

	resume := BuildResumeContext(traces)
	if resume == "" {
		t.Fatal("a run that called a tool produced no resume context")
	}
	for _, want := range []string{"get_component_spec", "grid", "=== grid (Data Grid) ==="} {
		if !strings.Contains(resume, want) {
			t.Errorf("the resume context does not carry %q:\n%s", want, resume)
		}
	}
	if strings.Contains(resume, models.TerminalToolAskUser) {
		t.Error("the resume context replays the question itself, which the answer already covers")
	}
}

func TestBuildResumeContextIsEmptyWhenThereWasNoWork(t *testing.T) {
	if got := BuildResumeContext(nil); got != "" {
		t.Errorf("no traces produced %q", got)
	}
	// Thinking alone is not work to resume: it is not a lookup the agent would
	// otherwise repeat, and it is the part most likely to be stale after an answer.
	thinkingOnly := []models.StepTrace{{Iteration: 1, Thinking: "hmm"}}
	if got := BuildResumeContext(thinkingOnly); got != "" {
		t.Errorf("thinking-only traces produced %q", got)
	}
}

func TestBuildResumeContextIsStableAcrossRuns(t *testing.T) {
	traces := []models.StepTrace{{
		ToolName:   "lookup",
		ToolInput:  map[string]interface{}{"z": 1, "a": 2, "m": 3},
		ToolResult: map[string]interface{}{"y": "b", "b": "y"},
	}}
	first := BuildResumeContext(traces)
	for i := 0; i < 20; i++ {
		if BuildResumeContext(traces) != first {
			t.Fatal("the resume context changed between identical runs - map ordering leaked into the prompt")
		}
	}
}

func TestBuildResumeContextIsCapped(t *testing.T) {
	huge := strings.Repeat("x", maxResumeContextChars*3)
	traces := []models.StepTrace{{
		ToolName:   "lookup",
		ToolResult: map[string]interface{}{"spec": huge},
	}}
	resume := BuildResumeContext(traces)
	if len(resume) > maxResumeContextChars+200 {
		t.Errorf("the resume context is %d chars - a single tool result can crowd out the answer", len(resume))
	}
	if !strings.Contains(resume, "truncated") {
		t.Error("a truncated resume context does not say so")
	}
}

func TestResumeContextRoundTripsThroughAQuestionAction(t *testing.T) {
	request := ClarificationRequest{
		Prompt: "Which layout?",
		Questions: []ClarificationQuestion{
			{Id: "q1", Question: "One column or two?", Options: []QuestionOption{{Value: "one", Label: "One"}, {Value: "two", Label: "Two"}}},
		},
	}
	action := request.ToAction("eru_studio")
	AttachResumeContext(action.Action, "--- WORK ALREADY DONE ---\nTool: get_component_spec\n")

	if got := ResumeContextFrom(action.Action); !strings.Contains(got, "get_component_spec") {
		t.Errorf("the resume context did not survive the action: %q", got)
	}

	// The extra key must not disturb reading the questions back.
	parsed, err := ParseClarificationRequest(action.Action)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Questions) != 1 || parsed.Questions[0].Id != "q1" {
		t.Errorf("the clarification request did not parse alongside the resume context: %+v", parsed)
	}
}

func TestAttachResumeContextIgnoresNothingToSay(t *testing.T) {
	action := map[string]interface{}{}
	AttachResumeContext(action, "")
	AttachResumeContext(action, "   ")
	if len(action) != 0 {
		t.Errorf("an empty resume context was stored: %v", action)
	}
	AttachResumeContext(nil, "something") // must not panic
	if got := ResumeContextFrom(nil); got != "" {
		t.Errorf("a nil action produced %q", got)
	}
}

func TestPendingQuestionFindsTheQuestionToResume(t *testing.T) {
	request := ClarificationRequest{Questions: []ClarificationQuestion{{Id: "q1", Question: "Which one?"}}}
	action := request.ToAction("eru_studio")
	AttachResumeContext(action.Action, "prior work")

	conversation := &Conversation{Messages: []AgentMessage{
		{Role: "user", Content: "build me a page"},
		{Role: "assistant", Actions: []AgentOutputAction{action}},
	}}

	_, found, ok := PendingQuestion(conversation)
	if !ok {
		t.Fatal("the pending question was not found")
	}
	if got := ResumeContextFrom(found.Action); got != "prior work" {
		t.Errorf("resume context = %q", got)
	}

	// A later user message means that question has already been answered.
	conversation.Messages = append(conversation.Messages, AgentMessage{Role: "user", Content: "two columns"})
	if _, _, ok := PendingQuestion(conversation); ok {
		t.Error("a question that the user has already answered still reads as pending")
	}
}
