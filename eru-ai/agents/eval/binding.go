package eval

import (
	"fmt"
	agents "github.com/eru-tech/eru/eru-ai/agents"
	"sort"
	"strings"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	ruleset "github.com/eru-tech/eru/eru-ai/agents/ruleset"
)

// Binding assertions judge the page a run produced, not the calls it made.
//
// The dashboard scenario asserted that every bound query had been probed, and a
// run passed it while shipping a dashboard where nothing showed a number. Both
// of the faults that produced that page are invisible from the tool calls:
//
//   - every tile carried primary_value_path "0.iff_amt" beside
//     primary_value_field "iff_amt". The path is read against the value of the
//     field, which by then is the number 527000, so it resolved to nothing and
//     the tile rendered a dash behind a 200 response.
//   - the charts and grids carried query_result_path "result.0.Results". There
//     is no "result" key in an eru-ql answer.
//
// Then the retry produced a third: told not to bind a query it had not probed,
// the agent unbound the tiles instead - data_source "static" with no query -
// and the probe assertion still passed.
//
// These read the components themselves, which is the only place the difference
// between a working page and an empty one is visible.

// component is one component found anywhere in a produced page, flattened to
// the property bag a binding assertion cares about.
type component struct {
	Id         string
	Type       string
	Properties map[string]interface{}
}

// componentsIn finds every component in everything a run produced.
//
// A page reaches the reply in more than one shape - the answer action, a nested
// "pages" array, a save_page argument - and an assertion that knew only one of
// them would quietly pass whenever the shape changed. So this walks the whole
// trajectory for objects that look like a component: an "id", a "type", and a
// "properties" bag.
func componentsIn(t Trajectory) []component {
	var found []component
	seen := map[string]bool{}

	var walk func(interface{})
	walk = func(node interface{}) {
		switch typed := node.(type) {
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		case map[string]interface{}:
			id, _ := typed["id"].(string)
			kind, _ := typed["type"].(string)
			if id != "" && kind != "" {
				if bag, ok := flattenProperties(typed["properties"]); ok && !seen[id] {
					seen[id] = true
					found = append(found, component{Id: id, Type: kind, Properties: bag})
				}
			}
			for _, child := range typed {
				walk(child)
			}
		}
	}

	for _, action := range t.Actions {
		walk(action.Action)
	}
	for _, call := range t.ToolCalls {
		walk(call.Input)
		walk(call.Result)
	}
	return found
}

// flattenProperties collapses the responsive breakpoint bags into one, so an
// assertion does not have to know which breakpoint a value was set at. Later
// breakpoints win, which matches how the page resolves them.
func flattenProperties(raw interface{}) (map[string]interface{}, bool) {
	bag, ok := raw.(map[string]interface{})
	if !ok {
		return nil, false
	}
	out := map[string]interface{}{}
	nested := false
	for _, breakpoint := range []string{"base", "sm", "md", "lg", "xl", "2xl"} {
		values, ok := bag[breakpoint].(map[string]interface{})
		if !ok {
			continue
		}
		nested = true
		for key, value := range values {
			out[key] = value
		}
	}
	if !nested {
		return bag, true
	}
	return out, true
}

func text(bag map[string]interface{}, key string) string {
	value, _ := bag[key].(string)
	return strings.TrimSpace(value)
}

// PagesBindSoundly requires every component the run produced to be wired to its
// data in a way that can actually resolve.
//
// The constraints are NOT written here. They are the catalog's rule table, the
// same declarations the in-loop validator rejects a page with, read through the
// shared engine. This assertion used to re-implement two of them by hand and the
// copies were already a day out of step with each other; whichever drifted, the
// suite would have quietly stopped describing the product - passing a page the
// validator now rejects, or failing one it now allows.
//
// The division of labour: the validator catches it during the run and the agent
// gets a chance to fix it; this catches it afterwards and tells us the rule still
// holds. Same sentence, two moments.
//
// It reports every fault it finds rather than the first, because these arrive in
// batches - one wrong idea applied to eight tiles - and fixing them one run at a
// time is how an afternoon disappears.
func PagesBindSoundly() Expectation {
	return expectation{
		describe: "every component binds to data that can resolve",
		check: func(t Trajectory) string {
			components := componentsIn(t)
			if len(components) == 0 {
				return ""
			}
			subjects := make([]ruleset.Subject, 0, len(components))
			for _, c := range components {
				properties := make(map[string]interface{}, len(c.Properties)+1)
				for key, value := range c.Properties {
					properties[key] = value
				}
				// The rules select on "type", which lives on the component rather
				// than in its property bag.
				properties["type"] = c.Type
				subjects = append(subjects, ruleset.Subject{
					Path: c.Id, Name: c.Type, Properties: properties,
				})
			}

			var faults []string
			for _, finding := range ruleset.Check(catalog.GenericRules(), subjects) {
				faults = append(faults, fmt.Sprintf("%s: %s", finding.Path, finding.Message))
			}
			if len(faults) == 0 {
				return ""
			}
			sort.Strings(faults)
			return strings.Join(faults, "; ")
		},
	}
}

// BindsEveryQuery requires each named query to be wired to something.
//
// "Do not bind a query you have not probed" has two solutions, and the cheap one
// is to bind nothing: a rejected dashboard came back with its KPI tiles switched
// to data_source "static", which satisfied the probe rule and left the user with
// seven tiles showing nothing at all.
func BindsEveryQuery(queries ...string) Expectation {
	return expectation{
		describe: fmt.Sprintf("binds a component to each of: %s", strings.Join(queries, ", ")),
		check: func(t Trajectory) string {
			components := componentsIn(t)
			if len(components) == 0 {
				return "the run produced no components to check"
			}
			bound := map[string]bool{}
			for _, c := range components {
				for _, key := range []string{"query", "query_name"} {
					if name := text(c.Properties, key); name != "" {
						bound[name] = true
					}
				}
			}
			var missing []string
			for _, name := range queries {
				if !bound[name] {
					missing = append(missing, name)
				}
			}
			if len(missing) == 0 {
				return ""
			}
			return fmt.Sprintf("nothing on the page reads %s", strings.Join(missing, ", "))
		},
	}
}

// GridsLabelTheirColumns requires a query-backed grid to head its columns with
// something a reader can understand.
//
// A query grid derives its columns from the first row and heads each one with
// the raw column name, so a dashboard built for senior management shipped with
// "an", "cl", "os_amt", "pn" and "avg_bal" as headings. The labels were not
// missing - the same run wrote "Anchor" and "Outstanding" onto the chart
// dimensions beside it - they just never reached the grid.
func GridsLabelTheirColumns() Expectation {
	return expectation{
		describe: "every query-backed grid labels its columns",
		check: func(t Trajectory) string {
			var bare []string
			for _, c := range componentsIn(t) {
				if c.Type != "grid" || text(c.Properties, "data_source") != "query" {
					continue
				}
				overrides, _ := c.Properties["column_overrides"].(map[string]interface{})
				labelled := 0
				for _, raw := range overrides {
					entry, ok := raw.(map[string]interface{})
					if !ok {
						continue
					}
					if label, _ := entry["label"].(string); strings.TrimSpace(label) != "" {
						labelled++
					}
				}
				if labelled == 0 {
					bare = append(bare, c.Id)
				}
			}
			if len(bare) == 0 {
				return ""
			}
			sort.Strings(bare)
			return fmt.Sprintf("these grids head their columns with the raw query column names: %s", strings.Join(bare, ", "))
		},
	}
}

// PagesAreWorthShipping requires the run to have handed over work the quality
// gate did not object to.
//
// This is deliberately NOT a second copy of the rubric. The rubric is the
// catalog's quality table and the gate applies it during the run; this asks only
// what the gate concluded. The distinction is the point: a fault the gate SAW
// and could not stop - because its one attempt was already spent - is a
// different failure from one the rubric never described, and only the first is
// visible here. A suite that re-derived the verdict would confuse the two and
// report the rubric as working precisely when it was being overruled.
func PagesAreWorthShipping() Expectation {
	return expectation{
		describe: "the quality gate had no objection to what was delivered",
		check: func(t Trajectory) string {
			if len(t.Quality) == 0 {
				return ""
			}
			final := t.Quality[len(t.Quality)-1]
			if !final.Rating.RetryWorthy() {
				return ""
			}
			return fmt.Sprintf("the page was delivered after the quality gate rated it %s on attempt %d and had no attempt left to improve it: %s",
				final.Rating, final.Attempt, final.Reason)
		},
	}
}

// TheQualityGateRan requires a verdict to exist at all.
//
// Worth asserting separately because the gate failing silently looks exactly
// like the gate being satisfied: no verdict, no objection, green suite. This is
// what tells the difference.
func TheQualityGateRan() Expectation {
	return expectation{
		describe: "the quality gate reached a verdict",
		check: func(t Trajectory) string {
			if len(t.Quality) == 0 {
				return "no quality verdict was recorded - the gate did not run, or the agent type does not implement one"
			}
			return ""
		},
	}
}

// ProducesComponent requires the answer to contain a component of a given type
// whose properties match.
//
// It exists to stop a fixture passing by not doing the thing. Asked for a board
// and given a table, every other assertion here still holds - the table binds
// soundly, labels its columns and probes its queries - and the run scores green
// for building the wrong component correctly.
func ProducesComponent(kind string, with map[string]string) Expectation {
	described := kind
	if len(with) > 0 {
		parts := make([]string, 0, len(with))
		for key, value := range with {
			parts = append(parts, fmt.Sprintf("%s=%q", key, value))
		}
		sort.Strings(parts)
		described = fmt.Sprintf("%s with %s", kind, strings.Join(parts, " "))
	}
	return expectation{
		describe: "the answer contains a " + described,
		check: func(t Trajectory) string {
			components := componentsIn(t)
			if len(components) == 0 {
				return "the answer carried no components at all"
			}
			var ofKind int
			for _, c := range components {
				if !strings.EqualFold(c.Type, kind) {
					continue
				}
				ofKind++
				matched := true
				for key, want := range with {
					got, _ := c.Properties[key].(string)
					if !strings.EqualFold(strings.TrimSpace(got), want) {
						matched = false
						break
					}
				}
				if matched {
					return ""
				}
			}
			if ofKind == 0 {
				return fmt.Sprintf("no %s anywhere in the answer", kind)
			}
			return fmt.Sprintf("%d %s(s), none of them %s", ofKind, kind, described)
		},
	}
}

// NoComponentSets requires that no component gives a property one of the named
// values.
//
// The shape of the faults it catches: a query that does not exist, bound anyway
// because the name sounded right; an entity bound to its physical table rather
// than to the entity. Both render as an empty component behind a 200, which is
// why they need asserting rather than looking at.
func NoComponentSets(property string, values ...string) Expectation {
	banned := map[string]bool{}
	for _, value := range values {
		banned[strings.ToLower(strings.TrimSpace(value))] = true
	}
	return expectation{
		describe: fmt.Sprintf("no component sets %s to %s", property, strings.Join(values, " or ")),
		check: func(t Trajectory) string {
			var faults []string
			for _, c := range componentsIn(t) {
				got, _ := c.Properties[property].(string)
				if banned[strings.ToLower(strings.TrimSpace(got))] {
					faults = append(faults, fmt.Sprintf("%s %q has %s = %q", c.Type, c.Id, property, strings.TrimSpace(got)))
				}
			}
			if len(faults) == 0 {
				return ""
			}
			sort.Strings(faults)
			return strings.Join(faults, "; ")
		},
	}
}

// StoppedBecause requires the run to have ended for a particular reason.
//
// Until the loop named its exits, "it finished", "it gave up" and "it was cut
// off by a limit" all arrived as the same green tick with an answer attached,
// and a suite could assert what an agent produced but never that it got to the
// end properly. A scenario that passes every content assertion while stopping on
// a token limit is not a passing scenario.
func StoppedBecause(want agents.StopReason) Expectation {
	return expectation{
		describe: fmt.Sprintf("the run ended with %s", want),
		check: func(t Trajectory) string {
			// An unknown reason is not a wrong one. Recordings made before the
			// loop named its exits carry none, and failing them would turn a
			// change in the harness into a suite full of red that says nothing
			// about the product. The cost is that this assertion passes
			// vacuously on such a run - acceptable only because every live reply
			// now carries a reason, so the vacuum closes as recordings age out.
			if t.StopReason == "" {
				return ""
			}
			if t.StopReason == want {
				return ""
			}
			return fmt.Sprintf("the run ended with %s, not %s", t.StopReason, want)
		},
	}
}

// RanToCompletion is the common case: the agent finished on its own terms,
// rather than being cut short by a budget or falling back to an earlier answer.
func RanToCompletion() Expectation {
	return expectation{
		describe: "the agent finished rather than being cut short",
		check: func(t Trajectory) string {
			switch t.StopReason {
			case "", agents.StopEndTurn:
				return ""
			case agents.StopAskedUser:
				return "the agent paused to ask a question instead of finishing"
			default:
				return fmt.Sprintf("the run was cut short: %s", t.StopReason)
			}
		},
	}
}
