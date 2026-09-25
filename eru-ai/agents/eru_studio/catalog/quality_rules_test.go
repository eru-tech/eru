package catalog

import (
	"strings"
	"testing"

	ruleset "github.com/eru-tech/eru/eru-ai/agents/ruleset"
)

// The safety property the whole severity split exists for: a quality rule must
// never be able to reject a page.
//
// If one leaks into the conformance table, a dashboard starts failing because a
// chart has no title - which is worse than shipping the untitled chart, and
// worse still because it would look like a validator bug rather than a
// misfiled rule.
func TestNoQualityRuleCanRejectAPage(t *testing.T) {
	for _, rule := range rules {
		if rule.Severity != SeverityError {
			t.Errorf("rule %q is in the conformance table with severity %q; a rule that cannot reject belongs in qualityRules", rule.Code, rule.Severity)
		}
	}
	for _, rule := range qualityRules {
		if rule.Severity != SeverityQuality {
			t.Errorf("rule %q is in the quality table but is marked %q, so a caller filtering on severity would enforce it", rule.Code, rule.Severity)
		}
	}
}

// The validator reads one table. A page breaking every quality rule must pass
// it without a single issue.
func TestTheValidatorIgnoresTheQualityTable(t *testing.T) {
	v := &validator{c: Get()}
	v.checkRules(ScopeBase, "p", "grid", map[string]interface{}{"data_source": "query", "query": "q"})
	v.checkRules(ScopeBase, "p", "tile", map[string]interface{}{"title": "nothing behind me"})
	v.checkRules(ScopeBase, "p", "bar_chart", map[string]interface{}{"query": "q"})
	v.checkRules(ScopeEveryBreakpoint, "p", "grid", map[string]interface{}{"data_source": "query", "query": "q"})
	if len(v.issues) != 0 {
		t.Errorf("the validator must raise nothing for a merely unpolished page: %+v", v.issues)
	}
}

// And the eval reads the same conformance table, so it cannot start failing
// runs over taste either.
func TestGenericRulesCarriesOnlyConformance(t *testing.T) {
	for _, rule := range GenericRules() {
		if rule.Severity != ruleset.SeverityError {
			t.Errorf("GenericRules leaked a %q rule: %s", rule.Severity, rule.Code)
		}
	}
	quality := GenericQualityRules()
	if len(quality) != len(qualityRules) {
		t.Fatalf("GenericQualityRules dropped rules: %d of %d", len(quality), len(qualityRules))
	}
	for _, rule := range quality {
		if rule.Severity != ruleset.SeverityQuality {
			t.Errorf("GenericQualityRules leaked a %q rule: %s", rule.Severity, rule.Code)
		}
	}
}

// Component becomes a Match, or a rule about grids would be applied to tiles.
func TestQualityRulesKeepTheirComponentMatch(t *testing.T) {
	for _, rule := range GenericQualityRules() {
		if rule.Code == string(CodeQualityRawColumnHeadings) {
			if rule.Match["type"] != "grid" {
				t.Errorf("the column-heading rule must match grids only, got %+v", rule.Match)
			}
			return
		}
	}
	t.Error("the column-heading rule is not in the projected table at all")
}

// A gate that only ever speaks after the fact pays for a second model call to
// say what the first could have been told for free.
func TestTheQualityRulesAreStatedInThePrompt(t *testing.T) {
	prompt := RulesPrompt()
	if !strings.Contains(prompt, "WHAT MAKES A PAGE GOOD RATHER THAN MERELY VALID") {
		t.Fatalf("the quality section is missing from the prompt:\n%s", prompt)
	}
	for _, want := range []string{"column_overrides", "secondary_is_currency DEFAULTS TO TRUE", "unbound tile", "Every chart"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt must state %q:\n%s", want, prompt)
		}
	}
	// The two sections must stay distinguishable: one rejects, one does not,
	// and a model told they are the same thing will treat both as optional.
	hard := strings.Index(prompt, "PROPERTIES THAT REQUIRE OTHER PROPERTIES")
	soft := strings.Index(prompt, "WHAT MAKES A PAGE GOOD")
	if hard < 0 || soft < hard {
		t.Error("the conformance rules must come first, under their own heading")
	}
	if !strings.Contains(prompt, "rejected and sent back to you") {
		t.Error("the conformance section must still say it rejects")
	}
	if !strings.Contains(prompt, "does not make the page wrong") {
		t.Error("the quality section must say it does not")
	}
}

// Every quality rule has to say what to do about it before the fact, or the
// model only ever learns by being sent back.
func TestEveryQualityRuleReachesTheModelSomehow(t *testing.T) {
	byCode := map[Code]bool{}
	for _, rule := range qualityRules {
		if strings.TrimSpace(rule.Message) == "" {
			t.Errorf("rule %q has no message, so the gate would send back an empty reason", rule.Code)
		}
		byCode[rule.Code] = byCode[rule.Code] || strings.TrimSpace(rule.Guidance) != ""
	}
	for code, stated := range byCode {
		if !stated {
			t.Errorf("no rule with code %q states its guidance, so the prompt never mentions it", code)
		}
	}
}
