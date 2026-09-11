package reasoning_agents

import (
	"context"
	"sort"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
)

// stubTool stands in for a configured tenant tool offering one action.
type stubTool struct {
	tools.Tool
	action string
}

func (s *stubTool) GetActionsList() []tools.ActionInfo {
	return []tools.ActionInfo{{Name: s.action}}
}

func (s *stubTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	return map[string]interface{}{"ok": true}, false, nil
}

func toolNames(offered map[string]tools.Tooling) []string {
	names := make([]string, 0, len(offered))
	for name := range offered {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// The agent must declare the lookups it needs by ACTION, not by tool name -
// whatever a tenant called its eru-ql or processo tool, the actions are the same.
func TestInternalToolRequestsAreByActionAndExplained(t *testing.T) {
	requests := (&EruStudioAgent{}).InternalToolRequests()
	wanted := map[string]bool{"execute_query": false, "fetch_pages": false, "fetch_page": false}
	for _, request := range requests {
		if _, expected := wanted[request.Action]; !expected {
			t.Errorf("unexpected internal tool request %q", request.Action)
			continue
		}
		wanted[request.Action] = true
		if request.Why == "" {
			t.Errorf("%s is requested without saying why - a missing tool would be undiagnosable", request.Action)
		}
	}
	for action, found := range wanted {
		if !found {
			t.Errorf("the agent does not ask for %q", action)
		}
	}
}

func TestNothingHasToBeAttachedForTheLookupsToAppear(t *testing.T) {
	agent := &EruStudioAgent{}
	// No AgentTools at all: this is the default configuration.
	agent.SetInternalTools(map[string]tools.Tooling{
		"execute_query": &stubTool{action: "execute_query"},
		"fetch_pages":   &stubTool{action: "fetch_pages"},
		"fetch_page":    &stubTool{action: "fetch_page"},
	})

	ctx := studio.WithPageScope(context.Background(), studio.PageScope{OrgId: "org_1", ProcessId: "proc_1"})
	offered := agent.ExtraTools(ctx)

	want := []string{utility.ComponentSpecToolName, utility.EntityMetadataToolName, utility.GetPageToolName, utility.ListPagesToolName}
	if got := toolNames(offered); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("offered %v, want %v", got, want)
	}

	// The page tools carry the scope, so the model is never asked for org/process.
	list, ok := offered[utility.ListPagesToolName].(*utility.PageLibraryTool)
	if !ok {
		t.Fatal("list_pages is not a page library tool")
	}
	if list.OrgId != "org_1" || list.ProcessId != "proc_1" {
		t.Errorf("scope = %s/%s", list.OrgId, list.ProcessId)
	}
	if list.ListDelegate == nil {
		t.Error("list_pages has no delegate")
	}
	get, _ := offered[utility.GetPageToolName].(*utility.PageLibraryTool)
	if get == nil || get.GetDelegate == nil {
		t.Error("get_page has no delegate")
	}
}

func TestTheComponentLookupIsAlwaysThereAndTheRestDegrade(t *testing.T) {
	// A tenant with no eru-ql and no processo tool: the agent still works, it
	// just cannot look entities or pages up.
	agent := &EruStudioAgent{}
	agent.SetInternalTools(nil)

	offered := agent.ExtraTools(context.Background())
	if got := toolNames(offered); strings.Join(got, ",") != utility.ComponentSpecToolName {
		t.Fatalf("offered %v, want only the component library lookup", got)
	}

	// And the prompt does not advertise what is not there.
	prompt := agent.GetSystemPrompt()
	if strings.Contains(prompt, utility.EntityMetadataToolName) {
		t.Error("the prompt mentions get_entity_metadata when the tool is absent")
	}
	if strings.Contains(prompt, "WHEN THE USER POINTS AT ANOTHER PAGE") {
		t.Error("the prompt teaches page references when the lookups are absent")
	}
}

func TestOnlyWhatTheTenantHasIsOffered(t *testing.T) {
	// Pages resolvable, entity metadata not.
	agent := &EruStudioAgent{}
	agent.SetInternalTools(map[string]tools.Tooling{"fetch_pages": &stubTool{action: "fetch_pages"}})

	offered := agent.ExtraTools(context.Background())
	if _, has := offered[utility.EntityMetadataToolName]; has {
		t.Error("entity metadata was offered without a query tool")
	}
	if _, has := offered[utility.ListPagesToolName]; !has {
		t.Error("list_pages was not offered although fetch_pages resolved")
	}
	// fetch_page did not resolve, so reading a page is not offered.
	if _, has := offered[utility.GetPageToolName]; has {
		t.Error("get_page was offered without fetch_page")
	}

	prompt := agent.GetSystemPrompt()
	if !strings.Contains(prompt, "WHEN THE USER POINTS AT ANOTHER PAGE") {
		t.Error("the page-reference guidance is missing although the lookup exists")
	}
	if strings.Contains(prompt, "FIELD NAMES AND LABELS COME FROM") {
		t.Error("the metadata guidance appears without the metadata tool")
	}
}

func TestAnAttachedQueryToolStillWins(t *testing.T) {
	// An owner who points the agent at a specific eru-ql tool keeps that control.
	attached := &stubTool{action: "execute_query"}
	internal := &stubTool{action: "execute_query"}
	agent := &EruStudioAgent{}
	agent.AgentTools = []agents.AgentTools{{ToolName: "eruql_custom", Tool: attached}}
	agent.SetInternalTools(map[string]tools.Tooling{"execute_query": internal})

	if got := agent.entityMetadataDelegate(context.Background()); got != attached {
		t.Error("the attached eru-ql tool did not take precedence over the resolved one")
	}
}

func TestPageScopeDefaultsToTheTenantAndCanBeOverridden(t *testing.T) {
	agent := &EruStudioAgent{}
	if scope := agent.pageScopeFor("tenant_1"); scope.OrgId != "tenant_1" || scope.ProcessId != "tenant_1" {
		t.Errorf("default scope = %+v, want the tenant for both", scope)
	}

	agent.PageOrgId = "org_9"
	agent.PageProcessId = "proc_9"
	if scope := agent.pageScopeFor("tenant_1"); scope.OrgId != "org_9" || scope.ProcessId != "proc_9" {
		t.Errorf("configured scope = %+v", scope)
	}

	// A half-configured agent still falls back for the missing half.
	half := &EruStudioAgent{PageOrgId: "org_9"}
	if scope := half.pageScopeFor("tenant_1"); scope.OrgId != "org_9" || scope.ProcessId != "tenant_1" {
		t.Errorf("half-configured scope = %+v", scope)
	}
}

func TestPageGuidanceTellsTheModelWhatNotToDo(t *testing.T) {
	agent := &EruStudioAgent{}
	agent.SetInternalTools(map[string]tools.Tooling{"fetch_page": &stubTool{action: "fetch_page"}})
	prompt := agent.GetSystemPrompt()

	for _, want := range []string{
		"list_pages",
		"get_page",
		"Never reuse its component ids",
		"not your answer",
		"entity metadata",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the page guidance does not cover %q", want)
		}
	}
}
