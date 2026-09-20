package module_store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eru-tech/eru/eru-gateway/module_model"
)

func rule(name string, rank int64, path string) *module_model.ListenerRule {
	return &module_model.ListenerRule{
		RuleName:    name,
		RuleRank:    rank,
		Hosts:       []string{"apiai.dev.smartvalues.co.in"},
		Paths:       []module_model.PathStruct{{MatchType: MatchTypePrefix, Path: path}},
		TargetHosts: []module_model.TargetHost{{Host: name + ".internal", Scheme: "https"}},
	}
}

func ruleNames(rules []*module_model.ListenerRule) []string {
	var names []string
	for _, r := range rules {
		names = append(names, r.RuleName)
	}
	return names
}

func equal(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestSortListenerRulesByRank(t *testing.T) {
	ms := &ModuleStore{ListenerRules: []*module_model.ListenerRule{
		rule("catchall", 100, "/"),
		rule("wellknown", 1, "/.well-known/"),
		rule("mcp", 50, "/mcp"),
	}}

	ms.SortListenerRules(context.Background())

	if got := ruleNames(ms.ListenerRules); !equal(got, []string{"wellknown", "mcp", "catchall"}) {
		t.Errorf("expected lowest rank first, got %v", got)
	}
}

func TestSortListenerRulesKeepsTiesStable(t *testing.T) {
	ms := &ModuleStore{ListenerRules: []*module_model.ListenerRule{
		rule("first", 10, "/a"),
		rule("second", 10, "/b"),
		rule("third", 10, "/c"),
	}}

	ms.SortListenerRules(context.Background())

	// Equal ranks may appear in any order, but they must not shuffle on every save.
	if got := ruleNames(ms.ListenerRules); !equal(got, []string{"first", "second", "third"}) {
		t.Errorf("rules sharing a rank must keep their existing order, got %v", got)
	}
}

func TestSaveListenerRuleSortsOnInsert(t *testing.T) {
	fileStore := &ModuleFileStore{}
	fileStore.ListenerRules = []*module_model.ListenerRule{rule("catchall", 100, "/")}

	// A rule added later must still take precedence when its rank says so.
	if err := fileStore.SaveListenerRule(context.Background(), rule("wellknown", 1, "/.well-known/"), fileStore, false); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if got := ruleNames(fileStore.ListenerRules); !equal(got, []string{"wellknown", "catchall"}) {
		t.Errorf("a newly saved rule must be placed by rank, not appended, got %v", got)
	}
}

func TestLookupHonoursRank(t *testing.T) {
	ms := &ModuleStore{
		ListenerRules: []*module_model.ListenerRule{
			rule("catchall", 100, "/"),
			rule("wellknown", 1, "/.well-known/"),
		},
	}
	ms.SortListenerRules(context.Background())

	r := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	r.Host = "apiai.dev.smartvalues.co.in"

	targetHost, _, _, _, err := ms.GetTargetGroupAuthorizer(context.Background(), r)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if targetHost.Host != "wellknown.internal" {
		t.Errorf("the specific low-rank rule must win over the catch-all, got %s", targetHost.Host)
	}
}

func TestLookupFallsBackToCatchAll(t *testing.T) {
	ms := &ModuleStore{
		ListenerRules: []*module_model.ListenerRule{
			rule("catchall", 100, "/"),
			rule("wellknown", 1, "/.well-known/"),
		},
	}
	ms.SortListenerRules(context.Background())

	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Host = "apiai.dev.smartvalues.co.in"

	targetHost, _, _, _, err := ms.GetTargetGroupAuthorizer(context.Background(), r)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if targetHost.Host != "catchall.internal" {
		t.Errorf("a path the specific rule does not cover must fall through, got %s", targetHost.Host)
	}
}
