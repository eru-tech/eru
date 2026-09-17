package catalog

import (
	"strings"
	"testing"
)

func ruleComponent(componentType string, base map[string]interface{}) map[string]interface{} {
	return component("c1", componentType, map[string]interface{}{
		"properties": map[string]interface{}{"base": base},
	})
}

func TestRuleFiresWhenItsConditionHolds(t *testing.T) {
	issues := Get().ValidatePage(page(ruleComponent("grid", map[string]interface{}{"view_mode": "board"})))
	found := false
	for _, issue := range issues {
		if issue.Code == CodeMountBoardCardUnset {
			found = true
			if issue.ComponentId != "c1" {
				t.Errorf("issue should name the component, got %q", issue.ComponentId)
			}
		}
	}
	if !found {
		t.Errorf("a board grid with no card page was accepted:\n%s", issueText(issues))
	}
}

func TestRuleStaysQuietWhenItsConditionDoesNot(t *testing.T) {
	for _, issue := range Get().ValidatePage(page(ruleComponent("grid", map[string]interface{}{"view_mode": "table"}))) {
		if issue.Code == CodeMountBoardCardUnset {
			t.Errorf("a table grid was asked for a card page: %s", issue.Message)
		}
	}
}

func TestRuleIsSatisfied(t *testing.T) {
	for _, issue := range Get().ValidatePage(page(ruleComponent("grid", map[string]interface{}{
		"view_mode": "board", "card_page_id": "card_pg",
	}))) {
		if issue.Code == CodeMountBoardCardUnset {
			t.Errorf("a board WITH a card page was reported: %s", issue.Message)
		}
	}
}

// No static check can know what an expression resolves to, and refusing it would
// ban a page that is perfectly good at runtime.
func TestRuleAcceptsARuntimeExpression(t *testing.T) {
	for _, issue := range Get().ValidatePage(page(ruleComponent("grid", map[string]interface{}{
		"view_mode": "board", "card_page_id": "@app.card_page",
	}))) {
		if issue.Code == CodeMountBoardCardUnset {
			t.Errorf("an expression-valued card page was reported: %s", issue.Message)
		}
	}
}

// The whole point of the table: the sentence the model reads and the check it is
// judged on come from the same entry.
func TestEveryRuleReachesThePrompt(t *testing.T) {
	prompt := RulesPrompt()
	for _, rule := range Rules() {
		if rule.Guidance == "" {
			t.Errorf("rule %s/%s has no guidance, so the model is judged on something it was never told", rule.Component, rule.Requires)
			continue
		}
		if !strings.Contains(prompt, rule.Guidance) {
			t.Errorf("rule %s/%s is enforced but missing from the prompt", rule.Component, rule.Requires)
		}
		if !strings.Contains(prompt, rule.Component) {
			t.Errorf("prompt does not mention %q", rule.Component)
		}
	}
}

func TestEveryRuleHasACodeAndAMessage(t *testing.T) {
	for _, rule := range Rules() {
		if rule.Code == "" {
			t.Errorf("rule %s/%s has no code, so it cannot be compared across runs", rule.Component, rule.Requires)
		}
		if rule.Message == "" {
			t.Errorf("rule %s/%s has no message, so a rejection tells the model nothing", rule.Component, rule.Requires)
		}
		if _, known := Get().Component(rule.Component); !known {
			t.Errorf("rule names component %q, which is not in the library", rule.Component)
		}
	}
}

// A rule that names a property the component does not have never fires. It is
// invisible: the tests pass, the prompt still promises the constraint, and
// nothing enforces it. This is the check that catches a rule written from
// memory rather than from the library.
func TestEveryRulePointsAtRealProperties(t *testing.T) {
	c := Get()
	for _, rule := range Rules() {
		if rule.When != "" {
			if _, known := c.Property(rule.Component, rule.When); !known {
				t.Errorf("rule condition %s.%s is not a property of %s, so the rule can never arm", rule.Component, rule.When, rule.Component)
			}
		}
		if _, known := c.Property(rule.Component, rule.Requires); !known {
			t.Errorf("rule requires %s.%s, which is not a property of %s", rule.Component, rule.Requires, rule.Component)
		}
		for _, value := range rule.Equals {
			property, known := c.Property(rule.Component, rule.When)
			if !known {
				continue
			}
			options := property.EnumValues()
			if len(options) == 0 {
				continue
			}
			matched := false
			for _, option := range options {
				if strings.EqualFold(option, value) {
					matched = true
				}
			}
			if !matched {
				t.Errorf("rule arms on %s.%s = %q, which is not one of %v", rule.Component, rule.When, value, options)
			}
		}
	}
}
