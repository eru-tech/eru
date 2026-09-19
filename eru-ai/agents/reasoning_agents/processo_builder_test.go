package reasoning_agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
)

func newBuilderAgent(t *testing.T, config string) *ProcessoBuilderAgent {
	t.Helper()
	if config == "" {
		config = `{"agent_type":"PROCESSO_BUILDER","agent_name":"processo_builder","model":"claude"}`
	}
	raw := json.RawMessage(config)
	agent := &ProcessoBuilderAgent{}
	if err := agent.MakeFromJson(context.Background(), &raw); err != nil {
		t.Fatal(err)
	}
	return agent
}

// The provider self-reference is what makes the generic loop reach this type's
// prompt, schema and validation. Forget it and the agent silently degrades to a
// plain reasoning agent.
func TestProcessoBuilderIsItsOwnProvider(t *testing.T) {
	agent := newBuilderAgent(t, "")
	if agent.ReasoningAgent.Agent.Provider == nil {
		t.Fatal("Provider was not set")
	}
	if _, ok := agent.ReasoningAgent.Agent.Provider.(*ProcessoBuilderAgent); !ok {
		t.Errorf("Provider is %T, want *ProcessoBuilderAgent", agent.ReasoningAgent.Agent.Provider)
	}
}

func TestProcessoBuilderPromptCarriesTheGeneratedCatalog(t *testing.T) {
	agent := newBuilderAgent(t, "")
	prompt := agent.GetSystemPrompt()

	if strings.Contains(prompt, "{{") {
		t.Error("the prompt still holds an unsubstituted placeholder")
	}
	// The catalog block, not a hand-written summary of it.
	for _, want := range []string{"dropdown_single_select", "open_status", "rcd_onr_xx", "get_field_spec"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not mention %s", want)
		}
	}
	// The instructions that stop the two irreversible mistakes.
	for _, want := range []string{"get_processo_context", "REPLACES", "f_name"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not warn about %s", want)
		}
	}
}

func TestProcessoBuilderAsksForTheLookupsItNeeds(t *testing.T) {
	agent := newBuilderAgent(t, "")
	requests := agent.InternalToolRequests()
	if len(requests) == 0 {
		t.Fatal("no internal tool requests")
	}
	for _, request := range requests {
		if request.Action == "" {
			t.Error("an internal tool request names no action")
		}
		if request.Why == "" {
			t.Errorf("%s has no Why, so a missing capability would be undiagnosable", request.Action)
		}
	}
}

// Without an eru-ql delegate the agent must still load and still offer what it
// can, rather than failing at construction.
func TestProcessoBuilderDegradesWithoutAnEruqlDelegate(t *testing.T) {
	agent := newBuilderAgent(t, "")
	extra := agent.ExtraTools(context.Background())

	if _, ok := extra[utility.FieldSpecToolName]; !ok {
		t.Error("get_field_spec needs no delegate and must always be offered")
	}
	if _, ok := extra[utility.ProcessoContextToolName]; ok {
		t.Error("get_processo_context cannot work without a delegate and must not be offered")
	}
}

// fakeEruqlDelegate stands in for the tenant's eru-ql tool.
type fakeEruqlDelegate struct {
	tools.Tool
}

func (f *fakeEruqlDelegate) GetActionsList() []tools.ActionInfo {
	return []tools.ActionInfo{{Name: "execute_query"}}
}

func TestProcessoBuilderOffersItsLookupsWhenAnEruqlToolIsResolved(t *testing.T) {
	agent := newBuilderAgent(t, "")
	agent.SetInternalTools(map[string]tools.Tooling{"execute_query": &fakeEruqlDelegate{}})

	extra := agent.ExtraTools(context.Background())
	for _, want := range []string{utility.FieldSpecToolName, utility.ProcessoContextToolName, utility.EntityMetadataToolName} {
		if _, ok := extra[want]; !ok {
			t.Errorf("%s was not offered", want)
		}
	}
}

func TestProcessoBuilderPlanningNoteKeepsAPlannerOffPerFieldSteps(t *testing.T) {
	note := (&ProcessoBuilderAgent{}).PlanningNote()
	if note == "" {
		t.Fatal("no planning note")
	}
	lower := strings.ToLower(note)
	if !strings.Contains(lower, "entity") || !strings.Contains(lower, "field") {
		t.Error("the note should tell a planner the unit of work is the entity, not the field")
	}
}

// ------------------------------------------------------------------ validation

func TestProcessoBuilderAcceptsAnHonestReport(t *testing.T) {
	agent := newBuilderAgent(t, "")
	err := agent.ValidateOutput(context.Background(), map[string]interface{}{
		"summary":  "Created deals with two fields.",
		"entities": []interface{}{map[string]interface{}{"name": "deals", "action": "created"}},
		"fields": []interface{}{
			map[string]interface{}{"entity_name": "deals", "name": "deal_value", "datatype": "currency", "action": "created"},
			map[string]interface{}{"entity_name": "deals", "name": "stage", "datatype": "status", "action": "created"},
		},
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// The field-name pattern is enforced only by the editor - the backend accepts
// anything - so this validator is the only thing standing between the model and
// a field the UI cannot edit.
func TestProcessoBuilderRejectsAFieldNameTheEditorWouldRefuse(t *testing.T) {
	agent := newBuilderAgent(t, "")
	err := agent.ValidateOutput(context.Background(), map[string]interface{}{
		"summary": "done",
		"fields": []interface{}{
			map[string]interface{}{"entity_name": "deals", "name": "dealValue", "datatype": "number", "action": "created"},
		},
	})
	if err == nil {
		t.Fatal("camelCase field name should be rejected")
	}
	if !strings.Contains(err.Error(), "dealValue") {
		t.Errorf("the error should name the offending field: %v", err)
	}
}

func TestProcessoBuilderRejectsAReservedName(t *testing.T) {
	agent := newBuilderAgent(t, "")
	err := agent.ValidateOutput(context.Background(), map[string]interface{}{
		"summary": "done",
		"fields": []interface{}{
			map[string]interface{}{"entity_name": "deals", "name": "rcd_onr_xx", "datatype": "textbox", "action": "updated"},
		},
	})
	if err == nil {
		t.Fatal("writing a system field should be rejected")
	}
}

func TestProcessoBuilderRejectsAnImpossibleDatatype(t *testing.T) {
	agent := newBuilderAgent(t, "")
	err := agent.ValidateOutput(context.Background(), map[string]interface{}{
		"summary": "done",
		"fields": []interface{}{
			map[string]interface{}{"entity_name": "deals", "name": "stage", "datatype": "kanban", "action": "created"},
		},
	})
	if err == nil {
		t.Fatal("an invented datatype should be rejected")
	}
}

// A report that says a write failed is an honest report, not a second failure.
func TestProcessoBuilderDoesNotPunishAReportedFailure(t *testing.T) {
	agent := newBuilderAgent(t, "")
	err := agent.ValidateOutput(context.Background(), map[string]interface{}{
		"summary": "The status field could not be created.",
		"fields": []interface{}{
			map[string]interface{}{"entity_name": "deals", "name": "badName", "datatype": "nonsense", "action": "failed", "note": "processo rejected it"},
		},
	})
	if err != nil {
		t.Errorf("a reported failure should not itself fail validation: %v", err)
	}
}

func TestProcessoBuilderOutputSchemaRequiresASummary(t *testing.T) {
	agent := newBuilderAgent(t, "")
	schema := agent.GetOutputSchema(context.Background())
	if len(schema.Required) == 0 || schema.Required[0] != "summary" {
		t.Errorf("summary should be required, got %v", schema.Required)
	}
	for _, want := range []string{"entities", "fields"} {
		if _, ok := schema.Properties[want]; !ok {
			t.Errorf("the output schema has no %s", want)
		}
	}
}

func TestProcessoBuilderInputSchemaFollowsTheAgentEnvelope(t *testing.T) {
	agent := newBuilderAgent(t, "")
	schema := agent.GetInputSchema(context.Background())
	for _, want := range []string{agents.AgentInputContentKey, agents.AgentInputParamsKey, agents.AgentInputFilesKey} {
		if _, ok := schema.Properties[want]; !ok {
			t.Errorf("the input schema has no %s", want)
		}
	}
}

// ---------------------------------------------- confirming the write landed

// metadataDelegate answers the read-back with a fixed entity/field set.
type metadataDelegate struct {
	tools.Tool
	fields []string
	calls  int
	err    error
}

func (m *metadataDelegate) GetActionsList() []tools.ActionInfo {
	return []tools.ActionInfo{{Name: "execute_query"}}
}

func (m *metadataDelegate) Execute(ctx context.Context, projectId, tenantId, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	m.calls++
	if m.err != nil {
		return nil, false, m.err
	}
	rows := make([]interface{}, 0, len(m.fields))
	for _, f := range m.fields {
		rows = append(rows, map[string]interface{}{"entity_name": "test", "field_name": f, "data_type": "textbox"})
	}
	return map[string]interface{}{"fetch_entity_table_column_metadata": map[string]interface{}{"rows": rows}}, false, nil
}

func builderWithMetadata(t *testing.T, fields []string) (*ProcessoBuilderAgent, *metadataDelegate) {
	t.Helper()
	agent := newBuilderAgent(t, "")
	delegate := &metadataDelegate{fields: fields}
	agent.SetInternalTools(map[string]tools.Tooling{"execute_query": delegate})
	return agent, delegate
}

func createdField(name string) map[string]interface{} {
	return map[string]interface{}{
		"summary": "done",
		"fields": []interface{}{
			map[string]interface{}{"entity_name": "test", "name": name, "datatype": "textbox", "action": "created"},
		},
	}
}

// A processo save can answer without an error and still write nothing, so the
// answer is checked against the model rather than taken on trust.
func TestProcessoBuilderRejectsAWriteThatDidNotLand(t *testing.T) {
	agent, delegate := builderWithMetadata(t, []string{"fn", "ln"})
	ctx := withBuilderScope(context.Background(), "processo", "tenant-1")

	err := agent.ValidateOutput(ctx, createdField("test_note"))
	if err == nil {
		t.Fatal("a field that is not in the model should be rejected")
	}
	if !strings.Contains(err.Error(), "test.test_note") {
		t.Errorf("the error should name the missing field: %v", err)
	}
	if delegate.calls != 1 {
		t.Errorf("the model should be read back exactly once, got %d", delegate.calls)
	}
}

func TestProcessoBuilderAcceptsAWriteThatLanded(t *testing.T) {
	agent, _ := builderWithMetadata(t, []string{"fn", "ln", "test_note"})
	ctx := withBuilderScope(context.Background(), "processo", "tenant-1")

	if err := agent.ValidateOutput(ctx, createdField("test_note")); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// The read-back reads the tenant of the request, never one the model chose.
func TestProcessoBuilderReadsBackTheRequestTenant(t *testing.T) {
	agent := newBuilderAgent(t, "")
	var gotTenant, gotProject string
	probe := &scopeProbe{onExecute: func(p, tn string) { gotProject, gotTenant = p, tn }}
	agent.SetInternalTools(map[string]tools.Tooling{"execute_query": probe})

	ctx := withBuilderScope(context.Background(), "processo", "tenant-42")
	_ = agent.ValidateOutput(ctx, createdField("test_note"))

	if gotTenant != "tenant-42" || gotProject != "processo" {
		t.Errorf("read back with project=%q tenant=%q, want processo/tenant-42", gotProject, gotTenant)
	}
}

type scopeProbe struct {
	tools.Tool
	onExecute func(projectId, tenantId string)
}

func (s *scopeProbe) GetActionsList() []tools.ActionInfo {
	return []tools.ActionInfo{{Name: "execute_query"}}
}

func (s *scopeProbe) Execute(ctx context.Context, projectId, tenantId, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	s.onExecute(projectId, tenantId)
	return map[string]interface{}{}, false, nil
}

// A read-back that cannot run must not turn a possibly-correct answer into a
// failure, and neither must a missing scope.
func TestProcessoBuilderDoesNotFailWhenItCannotConfirm(t *testing.T) {
	agent, _ := builderWithMetadata(t, nil)
	// No scope on the context.
	if err := agent.ValidateOutput(context.Background(), createdField("test_note")); err != nil {
		t.Errorf("no scope should skip confirmation, got %v", err)
	}

	broken := newBuilderAgent(t, "")
	broken.SetInternalTools(map[string]tools.Tooling{"execute_query": &metadataDelegate{err: errors.New("eru-ql down")}})
	ctx := withBuilderScope(context.Background(), "processo", "tenant-1")
	if err := broken.ValidateOutput(ctx, createdField("test_note")); err != nil {
		t.Errorf("a failed read-back should not fail the answer, got %v", err)
	}
}

// A field the answer already reports as failed is an honest report; confirming
// it would turn one failure into two.
func TestProcessoBuilderSkipsConfirmationForReportedFailures(t *testing.T) {
	agent, delegate := builderWithMetadata(t, []string{"fn"})
	ctx := withBuilderScope(context.Background(), "processo", "tenant-1")

	err := agent.ValidateOutput(ctx, map[string]interface{}{
		"summary": "could not save",
		"fields": []interface{}{
			map[string]interface{}{"entity_name": "test", "name": "test_note", "datatype": "textbox", "action": "failed", "note": "tool returned an empty result"},
		},
	})
	if err != nil {
		t.Errorf("a reported failure should pass, got %v", err)
	}
	if delegate.calls != 0 {
		t.Errorf("nothing was claimed written, so nothing should be read back; got %d calls", delegate.calls)
	}
}
