package eval

import (
	"fmt"
	"sort"
	"strings"
)

// What a run actually did, derived from the record rather than described.
//
// Everything else here asks whether the answer was right. This asks what it
// cost, which is a different question and the one nobody has been able to
// answer: how many tool calls, how many of them repeats, how much of the work
// was the agent looking things up versus changing them.
//
// It matters because the failure it exposes is invisible to every correctness
// check. An agent that calls the same lookup eleven times and produces a perfect
// answer passes every assertion in this package. It is also four times slower
// and four times dearer than it needs to be, and nothing would ever have said
// so.
//
// Derived, not recorded separately: the tool record already holds every call
// with its arguments, so this is a reading of evidence that was already there.

// Effort is what one run spent.
type Effort struct {
	Calls int
	// Distinct is calls with distinct (action, arguments). The gap between this
	// and Calls is repeated work.
	Distinct int
	// Repeats is the per-action count of identical calls made more than once.
	Repeats map[string]int
	// PerAction is the call count by action, most used first when rendered.
	PerAction map[string]int
	Failed    int
}

// Wasted is how many calls asked a question that had already been answered.
func (e Effort) Wasted() int { return e.Calls - e.Distinct }

// EffortOf reads a trajectory's tool record.
func EffortOf(t Trajectory) Effort {
	effort := Effort{Repeats: map[string]int{}, PerAction: map[string]int{}}
	seen := map[string]int{}
	for _, call := range t.ToolCalls {
		name := strings.TrimSpace(call.Name)
		if name == "" {
			continue
		}
		effort.Calls++
		effort.PerAction[name]++
		key := name + "|" + canonicalArgs(call.Input)
		seen[key]++
		if seen[key] == 1 {
			effort.Distinct++
		} else {
			effort.Repeats[name]++
		}
	}
	return effort
}

// canonicalArgs renders arguments so two identical calls compare equal. Cheap
// and approximate on purpose: this is a cost report, not a correctness check,
// and over-counting a repeat costs a line in a report rather than a wrong
// verdict.
func canonicalArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, args[key]))
	}
	return strings.Join(parts, "&")
}

// String renders one run's effort, and stays quiet when there is nothing to say.
func (e Effort) String() string {
	if e.Calls == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d tool call(s), %d distinct", e.Calls, e.Distinct)
	if wasted := e.Wasted(); wasted > 0 {
		fmt.Fprintf(&b, ", %d repeated", wasted)
	}
	b.WriteString("\n")

	type entry struct {
		name string
		n    int
	}
	list := make([]entry, 0, len(e.PerAction))
	for name, n := range e.PerAction {
		list = append(list, entry{name, n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].name < list[j].name
	})
	for _, item := range list {
		line := fmt.Sprintf("    %3d x %s", item.n, item.name)
		if repeated := e.Repeats[item.name]; repeated > 0 {
			line += fmt.Sprintf("  (%d of them asking something already answered)", repeated)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}
