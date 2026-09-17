package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// Rule is a constraint that one property places on another: set this, and that
// becomes required.
//
// These used to live in three places at once - a sentence in the component
// description, a paragraph in the system prompt, and a hand-written check in the
// mount validator - and only the last of them was enforced. Wording drifted from
// behaviour, and the model was being told something slightly different from what
// it would be judged on. A rule here is the single statement of the constraint:
// the validator raises it and the prompt is generated from it, so the two cannot
// disagree.
type Rule struct {
	// Component is the type the rule applies to.
	Component string
	// When is the property that arms the rule, and Equals the values that arm
	// it. An empty When makes the rule unconditional.
	When   string
	Equals []string
	// Requires is the property that must then carry a non-empty value.
	Requires string
	Code     Code
	// Message is what the model is told when the rule is broken. It has to read
	// as an instruction: it is the only thing the model gets to act on.
	Message string
	// Guidance is the same rule stated before the fact, for the system prompt.
	Guidance string
}

// rules is the table. Adding one here adds it to the validator and to the
// prompt at the same time; there is nowhere else to put it.
var rules = []Rule{
	{
		Component: "grid",
		When:      "view_mode",
		Equals:    []string{"board"},
		Requires:  "card_page_id",
		Code:      CodeMountBoardCardUnset,
		Message: "grid is in board view but has no \"card_page_id\", so every record draws with the built-in card, which cannot be designed. " +
			"Author the card as a small page - the fields that belong on a card, not the whole record - emit it in \"pages\" and set card_page_id to its id",
		Guidance: "A grid with view_mode \"board\" MUST have card_page_id pointing at a page you author and emit in \"pages\". " +
			"A board draws every record as a card, so the card is the design; left blank it falls back to a built-in card the user cannot change.",
	},
	{
		Component: "page_ref",
		Requires:  "page",
		Code:      CodeMountPageRefUnset,
		Message:   "page_ref has no \"page\" - it will render an empty panel. Emit the page it should mount in \"pages\" and set \"page\" to that page's id",
		Guidance: "A page_ref MUST have \"page\" set to the id of a page that exists or that you emit in \"pages\". " +
			"A page_ref with no page renders an empty panel, which looks to the user like nothing happened.",
	},
}

// Rules is the table, for callers that render it.
func Rules() []Rule { return rules }

// armed reports whether the rule's condition holds for this component.
func (r Rule) armed(properties map[string]interface{}) bool {
	if r.When == "" {
		return true
	}
	value, _ := properties[r.When].(string)
	value = strings.TrimSpace(value)
	for _, candidate := range r.Equals {
		if strings.EqualFold(value, candidate) {
			return true
		}
	}
	return false
}

// checkRules raises an issue for every rule this component arms and then fails.
//
// A property written as a runtime expression satisfies the rule: no static check
// can know what it resolves to, and refusing it would ban a legitimate page.
func (v *validator) checkRules(label, componentType string, properties map[string]interface{}) {
	for _, rule := range rules {
		if rule.Component != componentType || !rule.armed(properties) {
			continue
		}
		value, _ := properties[rule.Requires].(string)
		if strings.TrimSpace(value) != "" {
			continue
		}
		if _, present := properties[rule.Requires]; present && properties[rule.Requires] != nil {
			// A non-string value is someone else's complaint to make.
			if _, isString := properties[rule.Requires].(string); !isString {
				continue
			}
		}
		v.add(label, rule.Code, "%s", rule.Message)
	}
}

// RulesPrompt renders the table for the system prompt, so what the model is told
// and what it is judged on are the same sentences.
func RulesPrompt() string {
	byComponent := map[string][]Rule{}
	names := []string{}
	for _, rule := range rules {
		if _, seen := byComponent[rule.Component]; !seen {
			names = append(names, rule.Component)
		}
		byComponent[rule.Component] = append(byComponent[rule.Component], rule)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("PROPERTIES THAT REQUIRE OTHER PROPERTIES\n")
	b.WriteString("These are checked. An answer that breaks one is rejected and sent back to you.\n\n")
	for _, name := range names {
		fmt.Fprintf(&b, "%s:\n", name)
		for _, rule := range byComponent[name] {
			fmt.Fprintf(&b, "- %s\n", rule.Guidance)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
