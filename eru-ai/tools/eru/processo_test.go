package eru

import (
	"reflect"
	"testing"

	tools "github.com/eru-tech/eru/eru-ai/tools"
)

func processoActionNames() map[string]bool {
	names := make(map[string]bool, len(processoToolActions))
	for _, action := range processoToolActions {
		names[action.ActionName] = true
	}
	return names
}

func TestProcessoExposesTheBorrowedQueryAndFuncActions(t *testing.T) {
	names := processoActionNames()
	for _, want := range []string{
		"execute_query", "list_queries", "get_query",
		"save_query", "save_query_default", "save_query_project",
		"remove_query", "remove_query_default", "remove_query_project",
		"list_funcs", "fetch_func", "execute_func", "run_func",
		"save_func", "save_func_default", "save_func_project",
		"remove_func", "remove_func_default", "remove_func_project",
	} {
		if !names[want] {
			t.Errorf("processo does not expose %s", want)
		}
	}
	// The tool's own actions are still there and are not tenant scoped.
	for _, want := range []string{ProcessoSaveEntity, ProcessoFetchPages, ProcessoFetchPage, ProcessoSavePage} {
		if !names[want] {
			t.Errorf("processo lost its own action %s", want)
		}
	}
	if names[ProcessoSaveEntity+"_project"] {
		t.Error("processo's own write actions were scoped, which changes their names")
	}
}

func TestBorrowedActionsKeepTheSchemaOfTheToolThatOwnsThem(t *testing.T) {
	owners := map[string][]tools.ToolAction{
		"save_query": eruqlBaseActions,
		"get_query":  eruqlBaseActions,
		"save_func":  erufunctionsBaseActions,
		"run_func":   erufunctionsBaseActions,
	}
	for name, ownerActions := range owners {
		var want, got *tools.ToolAction
		for i, action := range ownerActions {
			if action.ActionName == name {
				want = &ownerActions[i]
			}
		}
		for i, action := range processoToolActions {
			if processoActionScopes[action.ActionName].BaseName == name {
				got = &processoToolActions[i]
				break
			}
		}
		if want == nil || got == nil {
			t.Fatalf("%s missing: want=%v got=%v", name, want != nil, got != nil)
		}
		if !reflect.DeepEqual(want.GetParameters(), got.GetParameters()) {
			t.Errorf("%s parameters drifted from the tool that owns it", name)
		}
	}
}

func TestBorrowedWriteActionsMapBackToTheirScope(t *testing.T) {
	for _, tc := range []struct {
		action string
		base   string
		scope  tools.TenantScope
	}{
		{"save_query", "save_query", tools.ScopeTenant},
		{"save_query_default", "save_query", tools.ScopeDefaultTenant},
		{"remove_func_project", "remove_func", tools.ScopeProject},
		{"list_funcs", "list_funcs", tools.ScopeTenant},
	} {
		scoped, ok := processoActionScopes[tc.action]
		if !ok {
			t.Errorf("%s has no scope", tc.action)
			continue
		}
		if scoped.BaseName != tc.base || scoped.Scope != tc.scope {
			t.Errorf("%s = %+v, want %s/%s", tc.action, scoped, tc.base, tc.scope)
		}
	}
}

func TestTheProcessoProjectIsTheOneBorrowedActionsAddress(t *testing.T) {
	if (&ProcessoTool{}).projectIdSegment() != "processo" {
		t.Error("an unconfigured processo tool no longer defaults to the processo project")
	}
	if (&ProcessoTool{ProjectId: "other"}).projectIdSegment() != "other" {
		t.Error("a configured project id is not used")
	}
	// execute_query only runs the mandatory vars lookup when it is configured to.
	if (&ProcessoTool{}).eruqlDelegate().MandatoryVarsQuery != "" {
		t.Error("the delegate invented a mandatory vars query")
	}
	if (&ProcessoTool{MandatoryVarsQuery: "q", MandatoryVarsTransform: "json"}).eruqlDelegate().MandatoryVarsTransform != "json" {
		t.Error("the delegate did not carry the configured transform")
	}
}
