package orchestrator

import (
	"context"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	eru_models "github.com/eru-tech/eru/eru-models"
)

// The observed failure: the user picked "nested_needed" and the agent proceeded
// as if they had picked "grid_inline", saying "the user re-sent the same prompt
// without picking an option". The answer never reached the sub-agent.

// theReply is the payload the client actually sent, verbatim.
var theReply = agents.AgentMessage{
	Content: "How should I handle the dynamic/repeatable Addresses and Contact Details sections? " +
		"I understand — please flag this as needing a nested page so I can request the full multi-page build.",
	Params: map[string]interface{}{
		agents.ClarificationAnswersParamKey: []interface{}{
			map[string]interface{}{"question_id": "eru_studio::q1", "selected": []interface{}{"nested_needed"}},
		},
	},
}

// theBranch is how the orchestrator recorded the pause: the sub-agent asked "q1",
// which it published to the client as "eru_studio::q1".
var theBranch = PausedBranch{StartStep: "eru_studio", EndStep: "eru_studio", QuestionIds: []string{"eru_studio::q1"}}

func TestTheAnswerReachesTheStepThatAsked(t *testing.T) {
	routed := withBranchAnswers(theReply, theBranch)

	answers, ok := routed.ClarificationAnswers()
	if !ok {
		t.Fatal("the answer was dropped on the way to the step that asked")
	}
	if len(answers) != 1 {
		t.Fatalf("answers = %+v", answers)
	}
	// The sub-agent only knows "q1" - it never saw the prefix.
	if answers[0].QuestionId != "q1" {
		t.Errorf("question_id = %q, want %q so the sub-agent recognises its own question", answers[0].QuestionId, "q1")
	}
	if strings.Join(answers[0].Selected, ",") != "nested_needed" {
		t.Errorf("selected = %v", answers[0].Selected)
	}

	// The original message is untouched: the same message is reused per branch.
	original, _ := theReply.ClarificationAnswers()
	if original[0].QuestionId != "eru_studio::q1" {
		t.Error("routing mutated the caller's message")
	}
}

func TestAnswersDoNotLeakBetweenBranches(t *testing.T) {
	message := agents.AgentMessage{Params: map[string]interface{}{
		agents.ClarificationAnswersParamKey: []interface{}{
			map[string]interface{}{"question_id": "eru_studio::q1", "selected": []interface{}{"nested_needed"}},
			map[string]interface{}{"question_id": "sql_step::q1", "selected": []interface{}{"last_30_days"}},
		},
	}}

	studioBranch := PausedBranch{StartStep: "eru_studio", QuestionIds: []string{"eru_studio::q1"}}
	sqlBranch := PausedBranch{StartStep: "sql_step", QuestionIds: []string{"sql_step::q1"}}

	studioAnswers, _ := withBranchAnswers(message, studioBranch).ClarificationAnswers()
	if len(studioAnswers) != 1 || studioAnswers[0].Selected[0] != "nested_needed" {
		t.Errorf("the studio branch got %+v", studioAnswers)
	}
	sqlAnswers, _ := withBranchAnswers(message, sqlBranch).ClarificationAnswers()
	if len(sqlAnswers) != 1 || sqlAnswers[0].Selected[0] != "last_30_days" {
		t.Errorf("the sql branch got %+v", sqlAnswers)
	}
}

func TestABranchWithNoAnswerOfItsOwnGetsNone(t *testing.T) {
	other := PausedBranch{StartStep: "some_other_step", QuestionIds: []string{"some_other_step::q1"}}
	routed := withBranchAnswers(theReply, other)
	if _, ok := routed.ClarificationAnswers(); ok {
		t.Error("a branch was handed another step's answer")
	}
	// And the param is removed rather than left as an empty list.
	if _, present := routed.Params[agents.ClarificationAnswersParamKey]; present {
		t.Error("an empty answer list was left on the message")
	}
}

func TestAnUnprefixedAnswerStillRoutesWhenThereIsNothingToRouteBy(t *testing.T) {
	message := agents.AgentMessage{Params: map[string]interface{}{
		agents.ClarificationAnswersParamKey: []interface{}{
			map[string]interface{}{"question_id": "q1", "selected": []interface{}{"yes"}},
		},
	}}
	bare := PausedBranch{StartStep: "eru_studio"}
	answers, ok := withBranchAnswers(message, bare).ClarificationAnswers()
	if !ok || len(answers) != 1 || answers[0].QuestionId != "q1" {
		t.Errorf("an unprefixed answer was dropped: %+v", answers)
	}
}

func TestAMessageWithNoAnswersIsUnchanged(t *testing.T) {
	plain := agents.AgentMessage{Content: "build me a page"}
	if routed := withBranchAnswers(plain, theBranch); routed.Content != plain.Content || routed.Params != nil {
		t.Errorf("a plain message was altered: %+v", routed)
	}
}

// The other half: the step has to carry the answer in its request body, or the
// routing above has nothing to deliver through.
func TestAPlanThatCannotDeliverAnAnswerIsInvalid(t *testing.T) {
	asker := agents.DiscoveredAgent{
		AgentName:             "eru_studio",
		SupportsClarification: true,
		InputSchema:           agents.AgentInputSchema(map[string]eru_models.JSONSchema{"code": {Type: "string", Description: "page"}}, nil),
	}

	// The plan the orchestrator produced: no clarification_answers anywhere.
	dropped := planWithTransform("eru_studio", `{{stringify (dict "content" "Build a business card page.")}}`)
	issues := validatePlan(context.Background(), dropped, []agents.DiscoveredAgent{asker}, nil, codeContext{})
	joined := ""
	for _, issue := range issues {
		joined += issue.Err + " "
	}
	if !strings.Contains(joined, agents.ClarificationAnswersParamKey) {
		t.Fatalf("a plan that cannot deliver an answer was accepted: %v", issues)
	}
	if !strings.Contains(joined, "guesses") {
		t.Errorf("the error does not say what goes wrong: %v", issues)
	}

	forwarded := planWithTransform("eru_studio",
		`{"content": "Build a business card page.", "params": {"clarification_answers": {{stringify .Vars.Body.params.clarification_answers}}}}`)
	if issues := validatePlan(context.Background(), forwarded, []agents.DiscoveredAgent{asker}, nil, codeContext{}); len(issues) > 0 {
		t.Errorf("a plan that forwards the answer was rejected: %v", issues)
	}
}

func TestAnAgentThatNeverAsksIsNotBurdened(t *testing.T) {
	quiet := agents.DiscoveredAgent{
		AgentName:             "processo_generate_sql",
		SupportsClarification: false,
		InputSchema:           agents.AgentInputSchema(nil, nil),
	}
	plan := planWithTransform("processo_generate_sql", `{{stringify (dict "content" .Vars.Body.content)}}`)
	if issues := validatePlan(context.Background(), plan, []agents.DiscoveredAgent{quiet}, nil, codeContext{}); len(issues) > 0 {
		t.Errorf("an agent that cannot ask was required to forward answers: %v", issues)
	}
}

// And the last hop: the sub-agent has to show the model the question next to the
// answer, or the model reads a bare id and guesses at the option.
func TestThePromptShowsTheQuestionWithTheAnswer(t *testing.T) {
	request := agents.ClarificationRequest{
		Questions: []agents.ClarificationQuestion{{
			Id:       "q1",
			Question: "How should I handle the dynamic/repeatable Addresses and Contact Details sections?",
			Options: []agents.QuestionOption{
				{Value: "grid_inline", Label: "Use an inline editable Data Grid"},
				{Value: "nested_needed", Label: "Flag this as needing a nested page"},
			},
		}},
	}

	// Even if the prefix survives to the sub-agent, the question text must stay
	// attached to the answer.
	for _, id := range []string{"q1", "eru_studio::q1"} {
		text := agents.FormatAnswersForModel(request, []agents.ClarificationAnswer{
			{QuestionId: id, Selected: []string{"nested_needed"}},
		})
		if !strings.Contains(text, "How should I handle") {
			t.Errorf("id %q lost the question text, leaving the model to guess:\n%s", id, text)
		}
		if !strings.Contains(text, "nested_needed") {
			t.Errorf("id %q lost the answer:\n%s", id, text)
		}
	}
}

// theStudioAgent is eru_studio as discovery describes it: it asks questions, and
// it declares the params it reads - a list that never mentioned
// clarification_answers.
func theStudioAgent() agents.DiscoveredAgent {
	return agents.DiscoveredAgent{
		AgentName:             "eru_studio",
		SupportsClarification: true,
		InputSchema: agents.AgentInputSchema(map[string]eru_models.JSONSchema{
			"code":                {Type: "string", Description: "the current page"},
			"output_mode":         {Type: "string", Description: "full | patch | auto"},
			"inline_nested_pages": {Type: "boolean", Description: "inline mounted pages"},
			"base_revision":       {Type: "string"},
			"scope":               {Type: "string"},
		}, nil),
	}
}

// The deadlock seen in production: the clarification rule demands the step
// forward params.clarification_answers, and the closed-list check calls that same
// key silently discarded. Two repair attempts burned, then the planner gave up
// and pasted the answer into content, where it rendered as
// "Clarification answers: null" and the agent guessed an option.
func TestForwardingTheAnswerSatisfiesEveryRuleAtOnce(t *testing.T) {
	// The form the planner actually emits - stringify over a dict, which is the
	// form whose params keys the validator can read.
	plan := planWithTransform("eru_studio",
		`{{stringify (dict "content" "Build a business card page." "params" (dict "clarification_answers" .Vars.Body.params.clarification_answers))}}`)

	issues := validatePlan(context.Background(), plan, []agents.DiscoveredAgent{theStudioAgent()}, nil, codeContext{})
	for _, issue := range issues {
		t.Errorf("a plan that forwards the answer was rejected: %s", issue.Err)
	}
}

func TestTheAnswerKeyIsPartOfTheContractOfEveryAgentThatAsks(t *testing.T) {
	asker := theStudioAgent()
	if !containsString(asker.ParamKeys(), agents.ClarificationAnswersParamKey) {
		t.Errorf("an agent that asks questions does not list %s among the params it reads: %v",
			agents.ClarificationAnswersParamKey, asker.ParamKeys())
	}
	// The keys it declares itself are untouched.
	for _, declared := range []string{"code", "output_mode", "scope"} {
		if !containsString(asker.ParamKeys(), declared) {
			t.Errorf("declared param %q was lost", declared)
		}
	}
	// The schema shown to the planner agrees with the list it is validated against.
	if _, described := asker.ParamsSchema().Properties[agents.ClarificationAnswersParamKey]; !described {
		t.Error("the params schema shown to the planner omits the key the validator requires")
	}

	// An agent that cannot ask is not given the key.
	quiet := agents.DiscoveredAgent{
		AgentName:   "processo_generate_sql",
		InputSchema: agents.AgentInputSchema(map[string]eru_models.JSONSchema{"code": {Type: "string"}}, nil),
	}
	if containsString(quiet.ParamKeys(), agents.ClarificationAnswersParamKey) {
		t.Error("an agent that never asks was told it reads clarification answers")
	}
}
