package orchestrator

import (
	"context"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/tools"
	functions "github.com/eru-tech/eru/eru-functions/functions"
)

func TestResolvedAnswersReachTheSubAgentOnResume(t *testing.T) {
	// The user is never shown a question the orchestrator answered, so the only
	// way the sub-agent gets that answer is if the checkpoint carries it.
	msg := agents.AgentMessage{Params: map[string]interface{}{}}
	msg = withResolvedAnswers(msg, []agents.ClarificationAnswer{
		{QuestionId: "eru_studio::q1", FreeText: "fn"},
	})

	answers, ok := msg.ClarificationAnswers()
	if !ok || len(answers) != 1 {
		t.Fatalf("resolved answer did not survive onto the message: %v", answers)
	}
	if answers[0].FreeText != "fn" {
		t.Fatalf("answer = %q", answers[0].FreeText)
	}
}

func TestTheUsersAnswerBeatsTheOrchestratorsGuess(t *testing.T) {
	// A partly self-resolved run asks the user the rest. If the user also speaks
	// to a question we answered, they are right and we are not.
	msg := agents.AgentMessage{Params: map[string]interface{}{
		agents.ClarificationAnswersParamKey: []agents.ClarificationAnswer{
			{QuestionId: "q1", FreeText: "the user's own answer"},
		},
	}}
	msg = withResolvedAnswers(msg, []agents.ClarificationAnswer{
		{QuestionId: "q1", FreeText: "what we assumed"},
		{QuestionId: "q2", FreeText: "sn"},
	})

	answers, _ := msg.ClarificationAnswers()
	if len(answers) != 2 {
		t.Fatalf("expected the user's answer plus the one they were not asked, got %d", len(answers))
	}
	byId := map[string]string{}
	for _, a := range answers {
		byId[a.QuestionId] = a.FreeText
	}
	if byId["q1"] != "the user's own answer" {
		t.Fatalf("the orchestrator overrode the user: q1 = %q", byId["q1"])
	}
	if byId["q2"] != "sn" {
		t.Fatalf("q2 = %q", byId["q2"])
	}
}

func TestWithResolvedAnswersLeavesAMessageAloneWhenNothingWasResolved(t *testing.T) {
	msg := agents.AgentMessage{Params: map[string]interface{}{"code": "x"}}
	out := withResolvedAnswers(msg, nil)
	if _, ok := out.ClarificationAnswers(); ok {
		t.Fatal("invented an answers param out of nothing")
	}
}

func TestAnAnswerWithNoSourceIsNotAnAnswer(t *testing.T) {
	// The resolver is allowed to answer questions of fact from evidence it can
	// cite. A claim with no citation is a guess, and a guess must go to the user.
	r := resolvedAnswer{QuestionId: "q1", Answer: "fn", Source: "entity metadata for scf"}
	line := r.assumptionLine("Which entity holds the transactions?")
	for _, want := range []string{"Which entity holds the transactions?", "fn", "entity metadata for scf"} {
		if !strings.Contains(line, want) {
			t.Fatalf("assumption line %q does not mention %q", line, want)
		}
	}
}

func TestAssumptionsAreSaidOutLoud(t *testing.T) {
	result := map[string]interface{}{"response": "Built the page."}
	noteAssumptions(result, []string{"- Which field? — assumed **fn** (entity metadata)"})

	got, _ := result["response"].(string)
	if !strings.Contains(got, "Built the page.") {
		t.Fatal("the answer itself was lost")
	}
	if !strings.Contains(got, "without asking you") || !strings.Contains(got, "fn") {
		t.Fatalf("the user cannot see what was assumed:\n%s", got)
	}
}

func TestNothingIsAddedWhenNothingWasAssumed(t *testing.T) {
	result := map[string]interface{}{"response": "Built the page."}
	noteAssumptions(result, nil)
	if got, _ := result["response"].(string); got != "Built the page." {
		t.Fatalf("response was altered with no assumptions to report: %q", got)
	}
}

func TestQuestionsAreRenderedWithTheirIdsAndOptions(t *testing.T) {
	out := renderQuestions([]agents.ClarificationQuestion{{
		Id:       "q1",
		Question: "Chart or table?",
		Options:  []agents.QuestionOption{{Value: "c", Label: "Chart"}, {Value: "t", Label: "Table"}},
	}})
	for _, want := range []string{"q1", "Chart or table?", "Chart", "Table"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered questions missing %q:\n%s", want, out)
		}
	}
}

func TestTheResolverIsToldNotToGuessAtIntent(t *testing.T) {
	// The whole safety of this mechanism rests on the resolver refusing
	// preference questions. If that instruction ever goes, the orchestrator
	// starts quietly deciding what the user wanted.
	for _, want := range []string{"question of FACT", "WANTS", "not certain", "preference, not a fact"} {
		if !strings.Contains(resolverSystemPrompt, want) {
			t.Fatalf("the resolver prompt no longer says %q, so it may answer questions only the user can answer", want)
		}
	}
}

// resolverStubModel stands in for the model so the resolution path can be
// exercised end to end: what it returns is what a resolver would have decided.
type resolverStubModel struct {
	models.Model
	content string
	prompt  string
}

func (m *resolverStubModel) RunToolLoop(ctx context.Context, chatRequest models.ChatRequest, toolsMap map[string]tools.Tooling, agentPrompt models.AgentPrompt, maxIterations int, thinkingBudget int, toolExecutor models.ToolExecutor) (models.Message, []models.StepTrace, error) {
	m.prompt = agentPrompt.String()
	if len(chatRequest.Messages) > 0 {
		m.prompt += "\n" + chatRequest.Messages[0].Content
	}
	return models.Message{
		Content:      m.content,
		TerminalTool: models.TerminalToolStructuredOutput,
	}, nil, nil
}

func resolverFixture(content string) (*OrchestratorAgent, *resolverStubModel) {
	stub := &resolverStubModel{content: content}
	oa := &OrchestratorAgent{}
	oa.Model = stub
	return oa, stub
}

func TestAFactualQuestionIsAnsweredWithoutTheUser(t *testing.T) {
	oa, _ := resolverFixture(`{"resolutions":[
		{"question_id":"q1","answered":true,"answer":"fn","source":"entity metadata for scf"}
	]}`)

	req := agents.ClarificationRequest{Questions: []agents.ClarificationQuestion{
		{Id: "q1", Question: "Which entity holds the facilities?"},
	}}
	out := oa.resolveClarifications(context.Background(), req, nil, nil, "processo", "t1")

	if len(out.Answers) != 1 || out.Answers[0].FreeText != "fn" {
		t.Fatalf("the question was not answered: %+v", out.Answers)
	}
	if len(out.Remaining.Questions) != 0 {
		t.Fatalf("a question that was answered was still put to the user: %+v", out.Remaining.Questions)
	}
	if len(out.Assumptions) != 1 || !strings.Contains(out.Assumptions[0], "entity metadata for scf") {
		t.Fatalf("the assumption is not reportable: %v", out.Assumptions)
	}
}

func TestAQuestionOfIntentStillGoesToTheUser(t *testing.T) {
	oa, _ := resolverFixture(`{"resolutions":[
		{"question_id":"q1","answered":false,"reason":"this is the user's preference"}
	]}`)

	req := agents.ClarificationRequest{Questions: []agents.ClarificationQuestion{
		{Id: "q1", Question: "Chart or table?"},
	}}
	out := oa.resolveClarifications(context.Background(), req, nil, nil, "processo", "t1")

	if len(out.Answers) != 0 {
		t.Fatalf("the orchestrator decided what the user wanted: %+v", out.Answers)
	}
	if len(out.Remaining.Questions) != 1 {
		t.Fatal("the question was not passed on to the user")
	}
}

func TestAnAnswerWithoutASourceIsTreatedAsAGuess(t *testing.T) {
	// The model claiming "answered" is not enough. Without something a person
	// could check, it is a guess, and guesses go to the user.
	oa, _ := resolverFixture(`{"resolutions":[
		{"question_id":"q1","answered":true,"answer":"fn","source":""},
		{"question_id":"q2","answered":true,"answer":"","source":"somewhere"}
	]}`)

	req := agents.ClarificationRequest{Questions: []agents.ClarificationQuestion{
		{Id: "q1", Question: "Which entity?"},
		{Id: "q2", Question: "Which field?"},
	}}
	out := oa.resolveClarifications(context.Background(), req, nil, nil, "processo", "t1")

	if len(out.Answers) != 0 {
		t.Fatalf("an unsourced or empty answer was accepted: %+v", out.Answers)
	}
	if len(out.Remaining.Questions) != 2 {
		t.Fatalf("both questions should have gone to the user, got %d", len(out.Remaining.Questions))
	}
}

func TestAPartlyResolvedRunOnlyAsksWhatIsLeft(t *testing.T) {
	oa, _ := resolverFixture(`{"resolutions":[
		{"question_id":"q1","answered":true,"answer":"fn","source":"step lookup_entities"},
		{"question_id":"q2","answered":false,"reason":"preference"}
	]}`)

	req := agents.ClarificationRequest{Questions: []agents.ClarificationQuestion{
		{Id: "q1", Question: "Which entity?"},
		{Id: "q2", Question: "Chart or table?"},
	}}
	out := oa.resolveClarifications(context.Background(), req, nil, nil, "processo", "t1")

	if len(out.Answers) != 1 {
		t.Fatalf("expected one self-answer, got %d", len(out.Answers))
	}
	if len(out.Remaining.Questions) != 1 || out.Remaining.Questions[0].Id != "q2" {
		t.Fatalf("the user should be asked only q2, got %+v", out.Remaining.Questions)
	}
}

func TestAnUnreadableResolutionFallsBackToAskingTheUser(t *testing.T) {
	oa, _ := resolverFixture(`not json at all`)
	req := agents.ClarificationRequest{Questions: []agents.ClarificationQuestion{{Id: "q1", Question: "Which entity?"}}}
	out := oa.resolveClarifications(context.Background(), req, nil, nil, "processo", "t1")

	if len(out.Answers) != 0 || len(out.Remaining.Questions) != 1 {
		t.Fatal("a broken resolution must degrade to asking, never to guessing")
	}
}

func TestTheResolverSeesWhatTheRunAlreadyKnows(t *testing.T) {
	oa, stub := resolverFixture(`{"resolutions":[]}`)
	resVars := map[string]*functions.TemplateVars{
		"lookup_entities": {Body: map[string]interface{}{"entities": []string{"fl", "fn", "sn"}}},
	}
	req := agents.ClarificationRequest{Questions: []agents.ClarificationQuestion{{Id: "q1", Question: "Which entity?"}}}
	oa.resolveClarifications(context.Background(), req, resVars, nil, "processo", "t1")

	if !strings.Contains(stub.prompt, "lookup_entities") {
		t.Fatalf("the resolver was not shown this run's own step results:\n%s", stub.prompt)
	}
	if !strings.Contains(stub.prompt, "Which entity?") {
		t.Fatal("the resolver was not shown the question")
	}
}
