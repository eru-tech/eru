package eval

import (
	"fmt"
	"strings"
)

// Expectation is one assertion over a run. It returns the reason it failed, or
// an empty string when it held.
//
// Every expectation here is agent-agnostic: it reads the trajectory, never the
// prose the model wrote about itself. That is the point. Each defect this suite
// exists to catch was a case where the report and the run disagreed - a page
// that claimed "confirmed run_query results" with no such call in the log, a
// field reported created that was never written, one reported failed that was.
type Expectation interface {
	Describe() string
	Check(Trajectory) string
}

type expectation struct {
	describe string
	check    func(Trajectory) string
}

func (e expectation) Describe() string          { return e.describe }
func (e expectation) Check(t Trajectory) string { return e.check(t) }

// Called requires a tool to have been used at least once.
func Called(tool string) Expectation {
	return expectation{
		describe: fmt.Sprintf("calls %s", tool),
		check: func(t Trajectory) string {
			if t.CallCount(tool) > 0 {
				return ""
			}
			return fmt.Sprintf("%s was never called (called: %s)", tool, listOf(t.Names()))
		},
	}
}

// NeverCalled requires a tool to have been left alone. The shape of "it should
// have asked instead of writing".
func NeverCalled(tool string) Expectation {
	return expectation{
		describe: fmt.Sprintf("never calls %s", tool),
		check: func(t Trajectory) string {
			if n := t.CallCount(tool); n > 0 {
				return fmt.Sprintf("%s was called %d time(s) and should not have been", tool, n)
			}
			return ""
		},
	}
}

// CalledTimes pins an exact count, for the one-step-per-entity kind of rule.
func CalledTimes(tool string, want int) Expectation {
	return expectation{
		describe: fmt.Sprintf("calls %s exactly %d time(s)", tool, want),
		check: func(t Trajectory) string {
			if got := t.CallCount(tool); got != want {
				return fmt.Sprintf("%s was called %d time(s), want %d", tool, got, want)
			}
			return ""
		},
	}
}

// CalledWith requires at least one call whose input carries this value at key.
// Argument correctness, not just tool correctness.
func CalledWith(tool, key string, want interface{}) Expectation {
	return expectation{
		describe: fmt.Sprintf("calls %s with %s=%v", tool, key, want),
		check: func(t Trajectory) string {
			calls := t.Calls(tool)
			if len(calls) == 0 {
				return fmt.Sprintf("%s was never called, so %s=%v could not be checked", tool, key, want)
			}
			var seen []string
			for _, call := range calls {
				got := lookup(call.Input, key)
				if fmt.Sprint(got) == fmt.Sprint(want) {
					return ""
				}
				seen = append(seen, fmt.Sprint(got))
			}
			return fmt.Sprintf("no %s call had %s=%v (saw %s)", tool, key, want, listOf(seen))
		},
	}
}

// CalledBefore fixes an order between two tools: look it up, then bind to it.
func CalledBefore(first, second string) Expectation {
	return expectation{
		describe: fmt.Sprintf("calls %s before %s", first, second),
		check: func(t Trajectory) string {
			a, b := t.firstIndex(first), t.firstIndex(second)
			if b < 0 {
				return ""
			}
			if a < 0 {
				return fmt.Sprintf("%s ran without %s ever being called", second, first)
			}
			if a > b {
				return fmt.Sprintf("%s ran before %s", second, first)
			}
			return ""
		},
	}
}

// Asked requires the run to have stopped and put a question to the user.
func Asked() Expectation {
	return expectation{
		describe: "asks the user",
		check: func(t Trajectory) string {
			if t.asked() {
				return ""
			}
			return "the run finished without asking anything"
		},
	}
}

// DidNotAsk is its opposite, for a prompt that carries everything needed.
func DidNotAsk() Expectation {
	return expectation{
		describe: "does not ask the user",
		check: func(t Trajectory) string {
			if t.asked() {
				return "the run stopped to ask, though the prompt left nothing open"
			}
			return ""
		},
	}
}

// ReportsNo fails when the answer contains a value at key - the shape of "no
// field may be reported as failed".
func ReportsNo(key string, value interface{}) Expectation {
	return expectation{
		describe: fmt.Sprintf("reports no %s=%v", key, value),
		check: func(t Trajectory) string {
			hits := valuesAt(t.answer(), key)
			for _, got := range hits {
				if fmt.Sprint(got) == fmt.Sprint(value) {
					return fmt.Sprintf("the answer reports %s=%v", key, value)
				}
			}
			return ""
		},
	}
}

// ReportsCount requires the answer to carry n entries at key, each matching
// value: three entities created, say.
func ReportsCount(key string, value interface{}, want int) Expectation {
	return expectation{
		describe: fmt.Sprintf("reports %d × %s=%v", want, key, value),
		check: func(t Trajectory) string {
			got := 0
			for _, v := range valuesAt(t.answer(), key) {
				if fmt.Sprint(v) == fmt.Sprint(value) {
					got++
				}
			}
			if got != want {
				return fmt.Sprintf("the answer reports %d × %s=%v, want %d", got, key, value, want)
			}
			return ""
		},
	}
}

// WithinIterations is a budget. A run that needs twenty turns to do a two-step
// job has not failed a validator, but it has told us something.
func WithinIterations(max int) Expectation {
	return expectation{
		describe: fmt.Sprintf("finishes within %d iterations", max),
		check: func(t Trajectory) string {
			if t.Iterations > max {
				return fmt.Sprintf("took %d iterations, budget %d", t.Iterations, max)
			}
			return ""
		},
	}
}

// EveryBoundQueryWasProbed is the generic form of the dashboard defect: every
// query a component reads from must have been run first, because a query's
// response shape can only be known by running it.
func EveryBoundQueryWasProbed(probeTool, bindingKey string) Expectation {
	return expectation{
		describe: fmt.Sprintf("probes every query it binds, via %s", probeTool),
		check: func(t Trajectory) string {
			probed := map[string]bool{}
			for _, call := range t.Calls(probeTool) {
				if name, ok := lookup(call.Input, bindingKey).(string); ok && name != "" {
					probed[name] = true
				}
			}
			var missing []string
			seen := map[string]bool{}
			for _, name := range boundQueries(t.answer()) {
				if probed[name] || seen[name] {
					continue
				}
				seen[name] = true
				missing = append(missing, name)
			}
			if len(missing) > 0 {
				return fmt.Sprintf("bound %s without running %s first", listOf(missing), probeTool)
			}
			return ""
		},
	}
}

func listOf(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

// IfCalledWith is CalledWith without requiring the call.
//
// "If you wrote, you wrote the right thing" is a different claim from "you
// wrote", and only the first is state-independent. The attachment scenario
// needs both halves and they belong in different places: whether the field ends
// up correct is an OUTCOME assertion, true whether this run wrote it or found it
// already right; whether a write that did happen carried the storage the user
// named is this one.
//
// Asserting the call itself failed the moment the fixture was run twice - the
// field was already there, the correct behaviour became doing nothing, and an
// action assertion cannot tell that from failing.
func IfCalledWith(tool, key string, want interface{}) Expectation {
	return expectation{
		describe: fmt.Sprintf("any %s call carries %s=%v", tool, key, want),
		check: func(t Trajectory) string {
			calls := t.Calls(tool)
			if len(calls) == 0 {
				return ""
			}
			var seen []string
			for _, call := range calls {
				got := lookup(call.Input, key)
				if fmt.Sprint(got) == fmt.Sprint(want) {
					return ""
				}
				seen = append(seen, fmt.Sprint(got))
			}
			return fmt.Sprintf("%s was called but no call had %s=%v (saw %s)", tool, key, want, listOf(seen))
		},
	}
}
