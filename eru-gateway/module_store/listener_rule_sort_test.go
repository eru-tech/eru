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

func multiPathRule(name string, rank int64, host string, paths ...string) *module_model.ListenerRule {
	r := &module_model.ListenerRule{
		RuleName: name, RuleRank: rank, Hosts: []string{host},
		TargetHosts: []module_model.TargetHost{{Host: name + ".internal", Scheme: "https"}},
	}
	for _, p := range paths {
		r.Paths = append(r.Paths, module_model.PathStruct{MatchType: MatchTypePrefix, Path: p})
	}
	return r
}

func catchAll(host string) *module_model.ListenerRule {
	return &module_model.ListenerRule{
		RuleName: "catchall", RuleRank: 100, Hosts: []string{host},
		Paths:       []module_model.PathStruct{{MatchType: MatchTypePrefix, Path: "/"}},
		TargetHosts: []module_model.TargetHost{{Host: "catchall.internal", Scheme: "https"}},
	}
}

func match(t *testing.T, ms *ModuleStore, host string, path string) (string, error) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Host = host
	targetHost, _, _, _, err := ms.GetTargetGroupAuthorizer(context.Background(), r)
	return targetHost.Host, err
}

// Several paths on one rule are alternatives, not a chain where the last one wins.
func TestPathsMatchAsAlternatives(t *testing.T) {
	ms := &ModuleStore{ListenerRules: []*module_model.ListenerRule{
		multiPathRule("both", 1, "apiai.dev", "/.well-known/", "/oauth2/"),
		catchAll("apiai.dev"),
	}}

	for _, path := range []string{"/.well-known/oauth-protected-resource", "/oauth2/register"} {
		got, err := match(t, ms, "apiai.dev", path)
		if err != nil || got != "both.internal" {
			t.Errorf("%s should match the multi-path rule, got %s %v", path, got, err)
		}
	}
}

// A rule must not match a request to a different host just because its path matches.
func TestHostIsRequiredWhenSet(t *testing.T) {
	ms := &ModuleStore{ListenerRules: []*module_model.ListenerRule{
		multiPathRule("apiaioauth", 1, "apiai.dev", "/oauth2/"),
		catchAll("other.dev"),
	}}

	got, err := match(t, ms, "other.dev", "/oauth2/register")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if got != "catchall.internal" {
		t.Errorf("a path-only match on the wrong host must not win, got %s", got)
	}
}

// Every criterion a rule sets has to match, not just the last one listed.
func TestCriteriaAreCombined(t *testing.T) {
	strict := multiPathRule("strict", 1, "apiai.dev", "/oauth2/")
	strict.Methods = []string{http.MethodPost}
	ms := &ModuleStore{ListenerRules: []*module_model.ListenerRule{strict, catchAll("apiai.dev")}}

	// GET has the right host and path but the wrong method.
	got, _ := match(t, ms, "apiai.dev", "/oauth2/register")
	if got != "catchall.internal" {
		t.Errorf("method mismatch must reject the rule, got %s", got)
	}

	r := httptest.NewRequest(http.MethodPost, "/oauth2/register", nil)
	r.Host = "apiai.dev"
	targetHost, _, _, _, err := ms.GetTargetGroupAuthorizer(context.Background(), r)
	if err != nil || targetHost.Host != "strict.internal" {
		t.Errorf("all criteria matching must select the rule, got %s %v", targetHost.Host, err)
	}
}

func TestPathMatchTypeIsCaseInsensitive(t *testing.T) {
	lower := multiPathRule("lower", 1, "apiai.dev")
	lower.Paths = []module_model.PathStruct{{MatchType: "prefix", Path: "/oauth2/"}}
	ms := &ModuleStore{ListenerRules: []*module_model.ListenerRule{lower, catchAll("apiai.dev")}}

	if got, _ := match(t, ms, "apiai.dev", "/oauth2/register"); got != "lower.internal" {
		t.Errorf("a lower case match_type must not be silently ignored, got %s", got)
	}
}

func TestRuleWithNoCriteriaMatchesNothing(t *testing.T) {
	empty := &module_model.ListenerRule{RuleName: "empty", RuleRank: 1,
		TargetHosts: []module_model.TargetHost{{Host: "empty.internal", Scheme: "https"}}}
	ms := &ModuleStore{ListenerRules: []*module_model.ListenerRule{empty, catchAll("apiai.dev")}}

	if got, _ := match(t, ms, "apiai.dev", "/anything"); got != "catchall.internal" {
		t.Errorf("an empty rule must not swallow traffic, got %s", got)
	}
}

func TestAuthorizerExceptionMatchesPath(t *testing.T) {
	guarded := multiPathRule("guarded", 1, "apiai.dev", "/oauth2/")
	guarded.AuthorizerName = "sv_eru_mcp"
	guarded.AuthorizerException = []module_model.PathStruct{{MatchType: MatchTypePrefix, Path: "/oauth2/"}}
	ms := &ModuleStore{
		ListenerRules: []*module_model.ListenerRule{guarded},
		Authorizers:   map[string]module_model.Authorizer{"sv_eru_mcp": {AuthorizerName: "sv_eru_mcp"}},
	}

	r := httptest.NewRequest(http.MethodPost, "/oauth2/register", nil)
	r.Host = "apiai.dev"
	_, authorizer, _, _, err := ms.GetTargetGroupAuthorizer(context.Background(), r)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if authorizer.AuthorizerName != "" {
		t.Errorf("an excepted path must come back unguarded, got %q", authorizer.AuthorizerName)
	}
}
