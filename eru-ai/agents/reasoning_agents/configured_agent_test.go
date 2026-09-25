package reasoning_agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eru-tech/eru/eru-ai/agents"
	ruleset "github.com/eru-tech/eru/eru-ai/agents/ruleset"
	"github.com/eru-tech/eru/eru-ai/tools"
)

// An agent built entirely through configuration - no provider, no Go, nothing
// this repository wrote for it - must get the same inner loop eru_studio gets.
//
// This is the claim the framework rests on, so it is tested as a claim: the
// agent below is exactly what a product user can create, and it is driven
// through the real Execute against a scripted model.

func configuredAgent(t *testing.T, retryCount int, rules []ruleset.RuleSet, answers ...string) (agents.AgentMessage, *scriptedModel, error) {
	t.Helper()
	model := &scriptedModel{answers: answers}
	ra := &ReasoningAgent{}
	ra.AgentName = "invoice_writer"
	ra.RetryCount = retryCount
	ra.Model = model
	ra.SystemPrompt = "You write invoices."
	ra.ValidationRules = rules
	// No SetProvider: there is no Go behind this agent at all.
	out, err := ra.Execute(context.Background(), agents.AgentMessage{Content: "bill acme for 3 days"}, "", "p", "t")
	return out, model, err
}

func invoiceRules() []ruleset.RuleSet {
	min := 1.0
	return []ruleset.RuleSet{{
		Subjects: "line_items",
		NameKey:  "description",
		Rules: []ruleset.Rule{
			{Property: "description", Code: "line_unlabelled",
				Message:  "{subject} has no {property} - a line the customer cannot read is a line they will query",
				Guidance: "Every line item needs a description."},
			{Kind: ruleset.KindRange, Property: "quantity", Min: &min,
				Code: "bad_quantity", Message: "{subject} has quantity {value}, which must be {field}",
				Guidance: "Quantities are at least 1."},
		},
	}}
}

func TestAConfigOnlyAgentRejectsAndCorrects(t *testing.T) {
	bad := `{"line_items":[{"description":"Consulting","quantity":3},{"quantity":0}]}`
	good := `{"line_items":[{"description":"Consulting","quantity":3},{"description":"Travel","quantity":1}]}`

	out, model, err := configuredAgent(t, 2, invoiceRules(), bad, good)
	if err != nil {
		t.Fatal(err)
	}
	if model.calls != 2 {
		t.Fatalf("the bad answer must be sent back, so two calls; got %d", model.calls)
	}
	// The rejection has to carry BOTH faults and say where they are.
	feedback := model.prompts[1]
	for _, want := range []string{"line_unlabelled", "quantity"} {
		if !strings.Contains(feedback, want) && !strings.Contains(feedback, "no description") {
			t.Errorf("the retry must name the fault; missing %q in:\n%s", want, feedback)
		}
	}
	if !strings.Contains(feedback, "line_items[1]") {
		t.Errorf("the retry must locate the fault:\n%s", feedback)
	}
	if out.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", out.RetryCount)
	}
}

func TestAConfigOnlyAgentFailsWhenItCannotBeCorrected(t *testing.T) {
	bad := `{"line_items":[{"quantity":0}]}`
	if _, _, err := configuredAgent(t, 1, invoiceRules(), bad, bad); err == nil {
		t.Fatal("an answer that never satisfies the declared rules must fail the run")
	}
}

// A declared quality rule must drive the gate: one retry, then ship.
func TestAConfigOnlyAgentGetsAQualityGate(t *testing.T) {
	rules := []ruleset.RuleSet{{
		Rules: []ruleset.Rule{
			{Property: "total", Code: "no_total", Message: "the invoice has no {property}"},
			{Property: "payment_terms", Severity: ruleset.SeverityQuality,
				Code: "no_terms", Message: "the invoice has no {property}, so the customer does not know when to pay",
				Guidance: "State payment terms."},
		},
	}}
	plain := `{"total":100}`
	better := `{"total":100,"payment_terms":"30 days"}`

	out, model, err := configuredAgent(t, 2, rules, plain, better)
	if err != nil {
		t.Fatal(err)
	}
	if model.calls != 2 {
		t.Fatalf("the quality rule must send it back once; %d calls", model.calls)
	}
	if !strings.Contains(model.prompts[1], "not a rejection") {
		t.Errorf("the quality retry prompt must be used, not the validation one:\n%s", model.prompts[1])
	}
	if out.Metrics == nil || len(out.Metrics.Quality) == 0 {
		t.Fatal("the verdict must be recorded like any other")
	}
	if !out.Metrics.Quality[0].Enforced || out.Metrics.Quality[0].Rating != agents.QualityPoor {
		t.Errorf("first verdict: %+v", out.Metrics.Quality[0])
	}
	if final := out.Metrics.Quality[len(out.Metrics.Quality)-1]; final.Rating != agents.QualityGood {
		t.Errorf("the improved answer must rate good: %+v", final)
	}
}

// A quality rule must never fail a request, however many attempts it costs.
func TestAConfiguredQualityRuleNeverFailsTheRun(t *testing.T) {
	rules := []ruleset.RuleSet{{Rules: []ruleset.Rule{
		{Property: "payment_terms", Severity: ruleset.SeverityQuality,
			Code: "no_terms", Message: "no {property}"},
	}}}
	plain := `{"total":100}`
	out, model, err := configuredAgent(t, 3, rules, plain, plain)
	if err != nil {
		t.Fatalf("taste must not fail a request: %v", err)
	}
	if model.calls != 2 {
		t.Errorf("one extra attempt and no more, got %d", model.calls)
	}
	final := out.Metrics.Quality[len(out.Metrics.Quality)-1]
	if final.Rating != agents.QualityPoor || final.Enforced {
		t.Errorf("the last verdict must be recorded as poor and unenforced: %+v", final)
	}
}

// An agent with no declared rules must behave exactly as it did before any of
// this existed.
func TestAnAgentWithNoRulesIsUnaffected(t *testing.T) {
	out, model, err := configuredAgent(t, 2, nil, `{"anything":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if model.calls != 1 {
		t.Errorf("no rules means no retries, got %d calls", model.calls)
	}
	if out.Metrics != nil && len(out.Metrics.Quality) != 0 {
		t.Errorf("and no verdict: %+v", out.Metrics.Quality)
	}
}

// The model has to be TOLD, or the first attempt is a guess and the retry pays
// for what the prompt could have said free.
func TestTheDeclaredRulesReachTheSystemPrompt(t *testing.T) {
	rules := invoiceRules()
	rules[0].Rules = append(rules[0].Rules, ruleset.Rule{
		Property: "notes", Severity: ruleset.SeverityQuality, Code: "c",
		Message: "m", Guidance: "Notes read better in full sentences.",
	})
	agent := &ReasoningAgent{}
	agent.ValidationRules = rules
	prompt := agent.ConfiguredRulesPrompt()

	for _, want := range []string{
		"RULES YOUR ANSWER MUST SATISFY",
		"Every line item needs a description.",
		"Quantities are at least 1.",
		"WHAT MAKES AN ANSWER GOOD RATHER THAN MERELY VALID",
		"Notes read better in full sentences.",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt must state %q:\n%s", want, prompt)
		}
	}
	// The two must stay distinguishable: one rejects, one does not.
	if strings.Index(prompt, "RULES YOUR ANSWER") > strings.Index(prompt, "WHAT MAKES AN ANSWER GOOD") {
		t.Error("the rejecting rules must come first")
	}
}

// The check the user called the most valuable in the system, now available to an
// agent nobody wrote Go for: "is this something you actually looked up, or did
// you make it up?"
//
// A recording tool is needed because evidence is collected at the tool boundary,
// which is the only place that sees both the arguments and the result.

type lookupTool struct {
	tools.Tool
	byName map[string]map[string]interface{}
	fail   bool
}

func (t *lookupTool) Execute(ctx context.Context, projectId, tenantId, action string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	if t.fail {
		return nil, false, errors.New("lookup service unavailable")
	}
	name, _ := params["entity_name"].(string)
	result, ok := t.byName[name]
	if !ok {
		return map[string]interface{}{"fields": []interface{}{}}, false, nil
	}
	return result, false, nil
}

func evidenceAgent(t *testing.T, tool tools.Tooling, callFirst bool, answers ...string) (agents.AgentMessage, *scriptedModel, error) {
	t.Helper()
	model := &scriptedModel{answers: answers, callTool: callFirst, toolAction: "get_entity_metadata",
		toolArgs: map[string]interface{}{"entity_name": "invoices"}}
	ra := &ReasoningAgent{}
	ra.AgentName = "form_builder"
	ra.RetryCount = 2
	ra.Model = model
	ra.AgentTools = []agents.AgentTools{{ToolName: "metadata", ToolKey: "get_entity_metadata", Tool: tool}}
	ra.Evidence = []agents.EvidenceRule{{
		Name: "known_fields", Action: "get_entity_metadata", FromResult: "fields.name",
	}}
	ra.ValidationRules = []ruleset.RuleSet{{
		Subjects: "fields",
		NameKey:  "identifier",
		Rules: []ruleset.Rule{{
			Kind: ruleset.KindFromEvidence, Property: "identifier", Evidence: "known_fields",
			Code:    "invented_field",
			Message: "{subject} binds to {value}, which get_entity_metadata never returned. Available: {field}",
			Guidance: "Bind only to fields the metadata lookup returned. A field name from memory renders an empty " +
				"input and saves nothing.",
		}},
	}}
	out, err := ra.Execute(context.Background(), agents.AgentMessage{Content: "build an invoice form"}, "", "p", "t")
	return out, model, err
}

func metadataTool() *lookupTool {
	return &lookupTool{byName: map[string]map[string]interface{}{
		"invoices": {"fields": []interface{}{
			map[string]interface{}{"name": "inv_no"},
			map[string]interface{}{"name": "inv_dt"},
			map[string]interface{}{"name": "amt"},
		}},
	}}
}

func TestAConfiguredAgentIsCaughtInventingAField(t *testing.T) {
	invented := `{"fields":[{"identifier":"inv_no"},{"identifier":"invoice_number"}]}`
	corrected := `{"fields":[{"identifier":"inv_no"},{"identifier":"inv_dt"}]}`

	out, model, err := evidenceAgent(t, metadataTool(), true, invented, corrected)
	if err != nil {
		t.Fatal(err)
	}
	if model.calls != 2 {
		t.Fatalf("the invented field must be sent back; %d model calls", model.calls)
	}
	feedback := model.prompts[1]
	if !strings.Contains(feedback, "invoice_number") {
		t.Errorf("the rejection must name the invented field:\n%s", feedback)
	}
	// Saying what IS available is the difference between a fix and another guess.
	if !strings.Contains(feedback, "inv_dt") {
		t.Errorf("the rejection must list what the lookup did return:\n%s", feedback)
	}
	if out.RetryCount != 1 {
		t.Errorf("RetryCount = %d, want 1", out.RetryCount)
	}
}

// The lesson that took a live run to learn, carried into the generic version:
// a lookup that never happened must not fault anything.
func TestNothingIsFaultedWhenTheLookupNeverHappened(t *testing.T) {
	invented := `{"fields":[{"identifier":"invented_entirely"}]}`
	_, model, err := evidenceAgent(t, metadataTool(), false, invented)
	if err != nil {
		t.Fatalf("an agent that never called the lookup cannot be held to it: %v", err)
	}
	if model.calls != 1 {
		t.Errorf("no evidence collected means no rule to break, got %d calls", model.calls)
	}
}

// A lookup that RAN and returned nothing is the tool working, and every binding
// after it is unsupported. This is the case the eru_studio version got wrong.
func TestALookupThatReturnedNothingStillEnforces(t *testing.T) {
	empty := &lookupTool{byName: map[string]map[string]interface{}{}}
	invented := `{"fields":[{"identifier":"anything"}]}`
	if _, _, err := evidenceAgent(t, empty, true, invented, invented); err == nil {
		t.Fatal("an empty result is evidence that the field does not exist")
	}
}

// A broken tool is not evidence of anything, and must not authorise or forbid.
func TestABrokenLookupDoesNotEnforce(t *testing.T) {
	broken := &lookupTool{fail: true}
	invented := `{"fields":[{"identifier":"anything"}]}`
	if _, _, err := evidenceAgent(t, broken, true, invented); err != nil {
		t.Errorf("a failed lookup must not be held against the agent: %v", err)
	}
}
