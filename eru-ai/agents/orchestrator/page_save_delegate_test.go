package orchestrator

import (
	"context"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	tools "github.com/eru-tech/eru/eru-ai/tools"
)

// The message has to name what the orchestrator holds. Told only that no tool
// offers save_page, a reader goes looking for a bug in the save; the tool was
// simply not in this agent's configuration.
func TestTheMissingDelegateNamesWhatIsAttached(t *testing.T) {
	oa := &OrchestratorAgent{}
	oa.AgentTools = []agents.AgentTools{
		{ToolName: "eruql_processo"},
		{ToolName: "eru-functions"},
	}
	report := oa.savePages(nil, []map[string]interface{}{{"page_id": "p1"}}, "processo", "t1")
	if len(report) != 1 {
		t.Fatalf("expected one line, got %v", report)
	}
	for _, want := range []string{"save_page", "eruql_processo", "eru-functions"} {
		if !strings.Contains(report[0], want) {
			t.Errorf("%q should appear in %q", want, report[0])
		}
	}
}

// A tool named in the configuration but never loaded is a different fault from
// one that was never configured, and the message must tell them apart.
func TestAnUnloadedToolIsMarked(t *testing.T) {
	oa := &OrchestratorAgent{}
	oa.AgentTools = []agents.AgentTools{{ToolName: "processo"}}
	if got := oa.attachedToolNames(); !strings.Contains(got, "not loaded") {
		t.Fatalf("an unhydrated tool must be marked: %s", got)
	}
}

func TestNoToolsAtAllIsSaidPlainly(t *testing.T) {
	oa := &OrchestratorAgent{}
	if got := oa.attachedToolNames(); got != "no tools at all" {
		t.Fatalf("got %q", got)
	}
}

// fakeSaveTool stands in for the tenant's processo tool.
type fakeSaveTool struct {
	tools.Tooling
	called map[string]interface{}
}

func (f *fakeSaveTool) Execute(ctx context.Context, projectId string, tenantId string, action string,
	params map[string]interface{}) (map[string]interface{}, bool, error) {
	f.called = params
	return map[string]interface{}{"ok": true}, false, nil
}

// An orchestrator attaches no tools, so the delegate has to come from the
// internal ones. Before this it found "no tools at all" and the page was
// reported as handled while nothing was written.
func TestTheSaveDelegateComesFromInternalTools(t *testing.T) {
	oa := &OrchestratorAgent{}
	save := &fakeSaveTool{}
	oa.SetInternalTools(map[string]tools.Tooling{"save_page": save})

	if oa.savePageDelegate(context.Background()) == nil {
		t.Fatal("an internal save_page tool must serve as the delegate")
	}
}

// The tool listing has to show internal tools too, or the failure message
// claims the orchestrator has nothing when it has what it needs.
func TestInternalToolsAppearInTheListing(t *testing.T) {
	oa := &OrchestratorAgent{}
	oa.SetInternalTools(map[string]tools.Tooling{"save_page": &fakeSaveTool{}})
	got := oa.attachedToolNames()
	if !strings.Contains(got, "save_page (internal)") {
		t.Fatalf("internal tools must be listed: %s", got)
	}
}

// Requesting the action by name keeps this tenant-agnostic: whatever the tenant
// called the tool, save_page is save_page.
func TestTheOrchestratorAsksForWhatThePageSaveNeeds(t *testing.T) {
	oa := &OrchestratorAgent{}
	wanted := map[string]bool{}
	for _, r := range oa.InternalToolRequests() {
		if r.Why == "" {
			t.Errorf("%s must say why, so a missing action is diagnosable", r.Action)
		}
		wanted[r.Action] = true
	}
	for _, action := range []string{"save_page", "get_processo_context"} {
		if !wanted[action] {
			t.Errorf("%s must be requested", action)
		}
	}
}
