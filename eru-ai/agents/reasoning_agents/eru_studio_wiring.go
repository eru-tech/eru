package reasoning_agents

import (
	"fmt"
	"sort"
	"strings"

	"github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

// Wiring a filter to a query is two decisions the page has to get right together,
// and neither shows up as a broken component: the filter needs a value before the
// user touches it, and no two state-bound controls may share a name. Both produce
// a page that renders perfectly and answers with nothing.

// wiringIssues reports those two faults on a resolved page.
func wiringIssues(page map[string]interface{}) []catalog.Issue {
	components := flatComponents(page)
	issues := filtersWithoutDefault(components)
	return append(issues, duplicateStateFieldNames(components)...)
}

// filtersWithoutDefault finds controls feeding a query that hold nothing on the
// first render.
//
// A control the user has not touched holds "", and "" is sent to the query as an
// empty value - for a date that is a cast error, for a currency a match on
// nothing. The tile is then blank until every filter is set by hand, and nothing
// reports a problem.
func filtersWithoutDefault(components []map[string]interface{}) []catalog.Issue {
	wanted := stateKeysSentToQueries(components)
	if len(wanted) == 0 {
		return nil
	}

	var issues []catalog.Issue
	for _, component := range components {
		base := baseProps(component)
		if stringProp(base, "value_source") != "state" {
			continue
		}
		key := stringProp(base, "state_key")
		if key == "" || !wanted[key] {
			continue
		}
		if hasDefault(base) {
			continue
		}
		issues = append(issues, catalog.Issue{
			Path: fmt.Sprint("components.", stringProp(component, "id")),
			Code: catalog.CodeFilterWithoutDefault,
			Message: fmt.Sprint(
				"this control feeds a query through api_payload_fields (state:", key,
				") but has no default, so on first render it sends \"\" and the query returns nothing or fails. ",
				"Give it a starting value: a date takes default_value_mode (current_date | first_day | last_day | custom) ",
				"with default_value_offset_days for a relative bound; anything else takes default_value, or default_state_key to seed from state.",
			),
		})
	}
	return issues
}

// duplicateStateFieldNames finds state-bound controls that share a name.
//
// A state-bound control mirrors its value into page data under its `name`, so two
// of them with the same name write to one slot and each displays the other's
// value. The screen then shows a value the state variable behind it does not
// have - which is exactly what makes it hard to spot.
func duplicateStateFieldNames(components []map[string]interface{}) []catalog.Issue {
	owners := map[string][]string{}
	for _, component := range components {
		base := baseProps(component)
		if stringProp(base, "value_source") != "state" {
			continue
		}
		name := stringProp(base, "name")
		if name == "" {
			continue
		}
		owners[name] = append(owners[name], stringProp(component, "id"))
	}

	names := make([]string, 0, len(owners))
	for name, ids := range owners {
		if len(ids) > 1 {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var issues []catalog.Issue
	for _, name := range names {
		ids := owners[name]
		sort.Strings(ids)
		issues = append(issues, catalog.Issue{
			Path: fmt.Sprint("components.", ids[0]),
			Code: catalog.CodeDuplicateStateFieldName,
			Message: fmt.Sprint(
				strings.Join(ids, " and "), " are both bound to state and both use name \"", name,
				"\", so they share one page-data slot and each shows the other's value. ",
				"Give each its own name - two filters over the same entity field are still two different fields on the page.",
			),
		})
	}
	return issues
}

// stateKeysSentToQueries collects the page-state keys any call-query or
// call-function passes as payload.
func stateKeysSentToQueries(components []map[string]interface{}) map[string]bool {
	keys := map[string]bool{}
	for _, component := range components {
		events, _ := component["events"].([]interface{})
		for _, raw := range events {
			event, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			switch stringProp(event, "action") {
			case "call-query", "call-function":
			default:
				continue
			}
			fields, _ := event["api_payload_fields"].([]interface{})
			for _, entry := range fields {
				text, ok := entry.(string)
				if !ok {
					continue
				}
				if key := stateKeyOf(text); key != "" {
					keys[key] = true
				}
			}
		}
	}
	return keys
}

// stateKeyOf reads the state key out of one api_payload_fields entry, which is
// "<source>:<path>[=<name>]" in either order around the "=".
func stateKeyOf(entry string) string {
	spec := strings.TrimSpace(entry)
	if at := strings.LastIndex(spec, "="); at > 0 {
		before, after := strings.TrimSpace(spec[:at]), strings.TrimSpace(spec[at+1:])
		if strings.Contains(before, ":") {
			spec = before
		} else if strings.Contains(after, ":") {
			spec = after
		}
	}
	source, path, found := strings.Cut(spec, ":")
	if !found || strings.TrimSpace(source) != "state" {
		return ""
	}
	// Only the top-level variable is a control's state_key; a dotted path digs
	// into its value.
	key, _, _ := strings.Cut(strings.TrimSpace(path), ".")
	return key
}

func hasDefault(base map[string]interface{}) bool {
	for _, key := range []string{"default_value_mode", "default_value", "default_state_key", "default_app_state_key", "default_date_custom"} {
		if value, present := base[key]; present && value != nil && fmt.Sprint(value) != "" {
			return true
		}
	}
	return false
}

func flatComponents(page map[string]interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	var walk func(raw interface{})
	walk = func(raw interface{}) {
		list, ok := raw.([]interface{})
		if !ok {
			return
		}
		for _, item := range list {
			component, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			out = append(out, component)
			walk(component["children"])
		}
	}
	walk(page["components"])
	return out
}

func baseProps(component map[string]interface{}) map[string]interface{} {
	properties, _ := component["properties"].(map[string]interface{})
	base, _ := properties["base"].(map[string]interface{})
	if base == nil {
		return map[string]interface{}{}
	}
	return base
}

func stringProp(source map[string]interface{}, key string) string {
	value, _ := source[key].(string)
	return value
}

// introducedWiringIssues reports only what this edit brought in, on the same
// principle as introducedPageIssues: a page the user already had is not the
// agent's to be failed for.
func introducedWiringIssues(basePage, resolved map[string]interface{}) []catalog.Issue {
	issues := wiringIssues(resolved)
	if len(basePage) == 0 || len(issues) == 0 {
		return issues
	}
	preExisting := map[string]bool{}
	for _, issue := range wiringIssues(basePage) {
		preExisting[issue.Fingerprint()] = true
	}
	out := make([]catalog.Issue, 0, len(issues))
	for _, issue := range issues {
		if preExisting[issue.Fingerprint()] {
			continue
		}
		out = append(out, issue)
	}
	return out
}
