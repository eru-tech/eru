package eval

import (
	"fmt"
	"sort"
	"strings"
)

// Fixture is one scenario: a prompt, and what a good run of it looks like.
//
// The assertions are code and are written once (expect.go). A fixture is the
// data that says which of them apply to this prompt, which is why adding a
// scenario is a few lines rather than a new test.
type Fixture struct {
	Name   string
	Agent  string
	Prompt string
	// HeldOut keeps this fixture out of the set development iterates against.
	// It is measured and never tuned against; see heldout.go for why, and for
	// the one rule that cannot be enforced in code.
	HeldOut bool
	Expect  []Expectation
	// ExpectOutcome asserts the workspace afterwards rather than the run.
	//
	// Prefer these. An action assertion is state-dependent: "creates three
	// entities" is false on a second run against the same workspace, where the
	// right behaviour is to extend what is already there. An outcome holds
	// either way.
	ExpectOutcome []OutcomeExpectation
	// ExpectOutcomeFor is ExpectOutcome for a fixture whose target differs per
	// attempt. It is handed the same token {{sample}} resolves to, so the
	// assertion names the thing that attempt actually wrote.
	//
	// Needed because a per-attempt target and an outcome assertion are the two
	// halves of the same fix: the target varies so attempts do not collide, and
	// the assertion is an outcome so a correct no-op on a second run still
	// passes. An assertion naming a fixed field would check attempt 1's work
	// five times.
	ExpectOutcomeFor func(sample string) []OutcomeExpectation
	// NeedsToolVisibility marks a fixture whose assertions read the agent's tool
	// calls. Some agents report none: the page agent runs its own plan/build loop
	// and neither its traces nor its metrics mention the tools it used, though
	// the service log shows them plainly. Against such a reply the assertions
	// cannot be evaluated - and reporting "run_query was never called" when it
	// demonstrably ran is worse than reporting nothing, because it sends the
	// reader after a defect that is not there.
	NeedsToolVisibility bool
	// RequiresAbsent names topics that must NOT already be in the workspace for
	// this fixture to mean anything. A scenario about building something cannot
	// be judged against a workspace where it is already built: the right
	// behaviour there is to do nothing, which is indistinguishable from failing.
	// Reported as BLOCKED, so it is visible rather than quietly green.
	RequiresAbsent []string
	// RequiresPresent names entities the workspace must already hold. A scenario
	// that says "on the existing entity X" is judging what the agent does with
	// X, and against a workspace without it the agent's correct move is to ask -
	// which scores as a failure on one fixture and, far worse, as a PASS on its
	// paired negative.
	//
	// Both attachment fixtures hit this the day the "test" entity was deleted:
	// attachment_with_storage_writes failed 5/5 while the agent was behaving
	// perfectly, and attachment_without_storage_asks passed 3/3 for entirely the
	// wrong reason - it asserts "asked and did not write", and a missing entity
	// produces that whatever the storage rule does. A green that does not depend
	// on the behaviour under test is worse than a red.
	RequiresPresent []string
	// Samples is how many times to run this scenario before believing the
	// result. Zero means "use the suite default".
	//
	// It belongs to the fixture rather than to a flag because the right number
	// is a property of the scenario, not of the run: attachment_without_storage
	// answers in twelve seconds and can afford five samples, while the dashboard
	// build takes ten minutes and one is all anybody will wait for. A single
	// global number is either too slow to use or too small to mean anything.
	//
	// It does NOT belong on the Agent config: the agent has no idea it is being
	// evaluated, and putting test settings there would mix how it behaves with
	// how we measure it.
	Samples int
	// Why records what this scenario is protecting against, so a future reader
	// knows whether a failure matters or the fixture has gone stale.
	Why string
}

// PromptFor is this fixture's prompt for attempt n (1-based).
//
// A write scenario repeated k times poisons its own precondition: the first
// attempt creates the thing, and every later one meets it already there. That is
// not a flake and not a defect - the agent correctly asks before changing a
// field's datatype - but it means attempts 2..k are testing a different scenario
// from attempt 1, and the fixture reports FLAKY while everything behaves.
//
// "{{sample}}" in a prompt is replaced by a per-attempt LETTER - a, b, c - so
// each attempt writes to its own target. Letters rather than digits because a
// field name may not contain one: substituting the attempt number produced
// "test_scan_1", which the agent rightly refused, turning a 3/5 flake into a
// 5/5 failure of the harness's own making.
//
// Stable rather than random on purpose: attempt 3 always writes test_scan_c, so
// a second run of the suite reuses the same handful of names instead of
// accumulating a new field every time.
//
// The alternative is a fresh workspace per attempt, which is what the larger
// harnesses do. Until we have that cheaply, this isolates the behaviour under
// test at the cost of a few fixture-owned fields.
func (f Fixture) PromptFor(attempt int) string {
	if !strings.Contains(f.Prompt, "{{sample}}") {
		return f.Prompt
	}
	return strings.ReplaceAll(f.Prompt, "{{sample}}", sampleToken(attempt))
}

// sampleToken is a, b, ... z, aa, ab - a name-safe stand-in for an attempt
// number.
func sampleToken(attempt int) string {
	if attempt < 1 {
		attempt = 1
	}
	n := attempt - 1
	token := ""
	for {
		token = string(rune('a'+n%26)) + token
		n = n/26 - 1
		if n < 0 {
			break
		}
	}
	return token
}

// OutcomeExpectations is what to assert about the workspace after attempt n.
func (f Fixture) OutcomeExpectations(attempt int) []OutcomeExpectation {
	if f.ExpectOutcomeFor != nil {
		return append(f.ExpectOutcome, f.ExpectOutcomeFor(sampleToken(attempt))...)
	}
	return f.ExpectOutcome
}

// WantsOutcome reports whether this fixture asserts anything about the workspace.
func (f Fixture) WantsOutcome() bool {
	return len(f.ExpectOutcome) > 0 || f.ExpectOutcomeFor != nil
}

// SampleCount is how many attempts this fixture wants, given the suite default.
// An explicit override beats both, for a one-off investigation of a flake.
func (f Fixture) SampleCount(suiteDefault, override int) int {
	if override > 0 {
		return override
	}
	if f.Samples > 0 {
		return f.Samples
	}
	if suiteDefault > 0 {
		return suiteDefault
	}
	return 1
}

// Failure is one expectation that did not hold.
type Failure struct {
	Expectation string
	Reason      string
}

// Result is how one fixture scored.
type Result struct {
	Fixture  string
	Passed   int
	Failures []Failure
	// Blocked says why the fixture could not be judged. A blocked fixture is
	// neither a pass nor a failure: it is a gap in what we can observe, and
	// calling it either would be a lie.
	Blocked string
}

func (r Result) OK() bool { return len(r.Failures) == 0 }

func (r Result) String() string {
	if r.Blocked != "" {
		return fmt.Sprintf("%-34s BLOCKED - %s", r.Fixture, r.Blocked)
	}
	if r.OK() {
		return fmt.Sprintf("%-34s PASS (%d checks)", r.Fixture, r.Passed)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-34s FAIL (%d passed, %d failed)\n", r.Fixture, r.Passed, len(r.Failures))
	for _, failure := range r.Failures {
		fmt.Fprintf(&b, "    - %s: %s\n", failure.Expectation, failure.Reason)
	}
	return strings.TrimRight(b.String(), "\n")
}

// Score runs every trajectory expectation.
func (f Fixture) Score(traj Trajectory) Result {
	result := Result{Fixture: f.Name}
	if f.NeedsToolVisibility && !traj.reportsTools() {
		result.Blocked = "this agent reports no tool calls in its traces or metrics, so its tool use cannot be checked from the reply"
		return result
	}
	for _, expectation := range f.Expect {
		if reason := expectation.Check(traj); reason != "" {
			result.Failures = append(result.Failures, Failure{Expectation: expectation.Describe(), Reason: reason})
			continue
		}
		result.Passed++
	}
	return result
}

// CheckPreconditions says why this scenario cannot mean anything against the
// workspace as it stands, or "" if it can. Checked before the run, not after:
// a scenario that cannot be judged should not cost ten minutes first.
func (f Fixture) CheckPreconditions(before Outcome) string {
	// An unread workspace is not an empty one.
	//
	// before is the zero Outcome when ERU_EVAL_ORG_ID and ERU_EVAL_PROCESS_ID
	// were not set, or when the read failed - and a precondition evaluated
	// against it reported "this scenario acts on test, which the workspace does
	// not have" for an entity that was sitting right there. Two fixtures were
	// blocked for a day on a message that named the wrong problem entirely, and
	// the message was so specific that nobody thought to doubt it.
	//
	// This is the same distinction the evidence rules and the query ledger both
	// turn on - never looked is not the same as looked and found nothing - and
	// it was wrong here while being right in three other places.
	if len(f.RequiresPresent) > 0 && !before.Read {
		return "the workspace could not be read, so this scenario's preconditions could not be checked - " +
			"set ERU_EVAL_ORG_ID and ERU_EVAL_PROCESS_ID. This is NOT a statement about whether the entities exist"
	}
	if present := topicsPresent(before, f.RequiresAbsent); len(present) > 0 {
		return fmt.Sprintf(
			"the workspace already holds %s, so there is nothing for this scenario to build - run it against a workspace without them",
			strings.Join(present, ", "))
	}
	if missing := entitiesMissing(before, f.RequiresPresent); len(missing) > 0 {
		return fmt.Sprintf(
			"this scenario acts on %s, which the workspace does not have - whatever the agent did, it was not the thing being tested",
			strings.Join(missing, ", "))
	}
	return ""
}

// ScoreOutcome adds the workspace assertions to a result already scored.
func (f Fixture) ScoreOutcome(result Result, before, after Outcome) Result {
	return f.ScoreOutcomeFor(result, before, after, 1)
}

// ScoreOutcomeFor is ScoreOutcome for a numbered attempt.
func (f Fixture) ScoreOutcomeFor(result Result, before, after Outcome, attempt int) Result {
	if reason := f.CheckPreconditions(before); reason != "" {
		result.Blocked = reason
		return result
	}
	for _, expectation := range f.OutcomeExpectations(attempt) {
		if reason := expectation.Check(before, after); reason != "" {
			result.Failures = append(result.Failures, Failure{Expectation: expectation.Describe, Reason: reason})
			continue
		}
		result.Passed++
	}
	return result
}

// Suite is a set of fixtures.
type Suite []Fixture

// ByName finds one fixture, for running a single scenario.
func (s Suite) ByName(name string) (Fixture, bool) {
	for _, fixture := range s {
		if fixture.Name == name {
			return fixture, true
		}
	}
	return Fixture{}, false
}

// Names lists the scenarios, sorted.
func (s Suite) Names() []string {
	out := make([]string, 0, len(s))
	for _, fixture := range s {
		out = append(out, fixture.Name)
	}
	sort.Strings(out)
	return out
}

// Report renders a whole run, and says whether it held.
func Report(results []Result) (string, bool) {
	var b strings.Builder
	ok := true
	for _, result := range results {
		// Blocked is not failure: it does not turn a suite red, because nothing
		// is known to be wrong. It is loud in the report so it does not become
		// a permanent quiet gap.
		if !result.OK() && result.Blocked == "" {
			ok = false
		}
		fmt.Fprintln(&b, result.String())
	}
	return b.String(), ok
}

// entitiesMissing is which of these named entities the workspace does not have.
// Exact, unlike topicsPresent: a scenario naming "test" means that entity, not
// anything whose name contains it.
func entitiesMissing(outcome Outcome, names []string) []string {
	var missing []string
	for _, name := range names {
		if _, ok := outcome.Has(name); !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

// topicsPresent is which of these topics the workspace already covers.
func topicsPresent(outcome Outcome, topics []string) []string {
	var present []string
	for _, topic := range topics {
		want := normalise(topic)
		for _, state := range outcome.Entities {
			if strings.Contains(normalise(state.Name), want) || strings.Contains(normalise(state.DisplayName), want) {
				present = append(present, topic)
				break
			}
		}
	}
	return present
}
