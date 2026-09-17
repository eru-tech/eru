package catalog

import (
	"strings"
)

// The tabs component decides which of its children is a tab panel by ID: a child
// counts as the panel for a tab only when its id begins "<tabs component id>-tab-".
// Anything else stays in children[] and is never rendered, and the runtime then
// creates its own empty panels to fill the gap.
//
// The agent had no way to know that - the contract was in the component's code
// and nowhere in its instructions - so it named the panels after their contents
// (tab_invoices, tab_finv_content). The result was a tabs component with two
// tabs and four children: two orphans holding all the content, and two
// auto-created empties, which are the ones the user sees.
//
// One panel per tab, each carrying the prefix. Both halves are checked, because
// either one alone still produces empty tabs.

// TabPanelPrefix is the id prefix a tabs component's panels must carry.
func TabPanelPrefix(tabsComponentId string) string {
	return tabsComponentId + "-tab-"
}

// tabTitles reads the tab list, which the panel may hold as a comma-separated
// string or as an array.
func tabTitles(value interface{}) []string {
	var parts []string
	switch typed := value.(type) {
	case string:
		parts = strings.Split(typed, ",")
	case []interface{}:
		for _, item := range typed {
			if text, ok := item.(string); ok {
				parts = append(parts, text)
			}
		}
	default:
		return nil
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// checkTabs holds a tabs component to its panel contract.
func (v *validator) checkTabs(label string, component map[string]interface{}, properties map[string]interface{}) {
	id, _ := component["id"].(string)
	if id == "" {
		return
	}
	titles := tabTitles(properties["tabs"])
	if len(titles) == 0 {
		return
	}

	rawChildren, present := component["children"]
	if !present {
		// A patch that does not restate children is not rewriting them.
		if v.partial {
			return
		}
		v.add(label, CodeTabsPanelCount,
			"this has %d tab(s) but no children. Each tab needs its own child container, in tab order, with id %q, %q ... - without them the tabs render empty",
			len(titles), TabPanelPrefix(id)+"0", TabPanelPrefix(id)+"1")
		return
	}
	children, ok := rawChildren.([]interface{})
	if !ok {
		return
	}

	prefix := TabPanelPrefix(id)
	panels := 0
	for _, raw := range children {
		child, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		childId, _ := child["id"].(string)
		if strings.HasPrefix(childId, prefix) {
			panels++
			continue
		}
		v.add(label, CodeTabsChildNotAPanel,
			"child %q is not a tab panel: a tabs component recognises a panel only by its id, which must start with %q (so %q for the first tab, %q for the second). "+
				"A child named anything else is never rendered, and the runtime adds an empty panel in its place - which is why the tab looks empty",
			childId, prefix, prefix+"0", prefix+"1")
	}

	if panels != len(titles) {
		v.add(label, CodeTabsPanelCount,
			"this has %d tab(s) but %d panel(s) with the %q prefix. There must be exactly one panel per tab, in the same order as the titles",
			len(titles), panels, prefix)
	}
}
