package eval

import (
	"fmt"
	"sort"
	"strings"
)

// Held-out fixtures: the set we report on is not the set we tuned against.
//
// # Why this is not paranoia
//
// Every fixture in this suite was written because something went wrong, and
// every one of them has since had the product changed to make it pass. That is
// the loop working. It is also, precisely, fitting the agent to four examples.
//
// The failure mode is not hypothetical and it is not slow. DGM's Appendix H has
// a variant that scored a perfect 2.0 by deleting the instrumentation its grader
// read; nothing that dramatic is needed here. It is enough to add one more
// sentence to a prompt for each fixture that fails, until the prompt is a list
// of four scenarios and the agent is excellent at exactly those and no better
// than before at anything else. The suite goes green, the product does not
// improve, and there is no signal anywhere that says so.
//
// A held-out set is the signal. It is measured and never optimised against, so
// the GAP between the two numbers is what tells you whether the last month of
// work generalised. Tune 5/5 and held-out 5/5 means the changes were real. Tune
// 5/5 and held-out 1/5 means they were memorised.
//
// # The discipline, which no code can enforce
//
// Nothing here can stop someone reading a held-out failure and patching the
// prompt to fix it. What the code does is make that a deliberate act rather than
// an accident:
//
//   - held-out fixtures do not run by default, so ordinary iteration never sees
//     them;
//   - the report keeps the two sets apart and prints the gap, so a widening gap
//     is visible rather than buried;
//   - and the rule is written here: IF YOU CHANGE THE PRODUCT BECAUSE A HELD-OUT
//     FIXTURE FAILED, MOVE THAT FIXTURE TO THE TUNE SET. It has done its job as
//     a held-out fixture and cannot do it twice. Write a new one to replace it.
//
// A held-out set that has been quietly optimised against is worse than none: it
// reports a generalisation that is not there, with the authority of a number.

// Tune returns the fixtures that may be iterated against.
func (s Suite) Tune() Suite {
	out := Suite{}
	for _, fixture := range s {
		if !fixture.HeldOut {
			out = append(out, fixture)
		}
	}
	return out
}

// Reported returns the held-out fixtures - measured, never tuned against.
func (s Suite) Reported() Suite {
	out := Suite{}
	for _, fixture := range s {
		if fixture.HeldOut {
			out = append(out, fixture)
		}
	}
	return out
}

// SetName is which half of the suite a run covers.
type SetName string

const (
	// SetTune is the default: the fixtures development iterates against.
	SetTune SetName = "tune"
	// SetHeldOut is the report set.
	SetHeldOut SetName = "held_out"
	// SetAll runs both, for a release measurement where the gap is the point.
	SetAll SetName = "all"
)

// Select returns the half of the suite a name asks for. An unknown name is the
// tune set: running fewer fixtures than intended is recoverable, and quietly
// burning the held-out set on a typo is not.
func (s Suite) Select(set SetName) Suite {
	switch set {
	case SetHeldOut:
		return s.Reported()
	case SetAll:
		return s
	default:
		return s.Tune()
	}
}

// ParseSet reads a set name, defaulting to the tune set.
func ParseSet(value string) SetName {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(SetHeldOut), "heldout", "held-out":
		return SetHeldOut
	case string(SetAll):
		return SetAll
	default:
		return SetTune
	}
}

// rate is passed/scored over a group, and whether anything was scored at all.
func rate(suite Suite, results []SampledResult) (passed, scored int, fixtures int) {
	inSet := map[string]bool{}
	for _, fixture := range suite {
		inSet[fixture.Name] = true
	}
	for _, result := range results {
		if !inSet[result.Fixture] {
			continue
		}
		fixtures++
		passed += result.Passed()
		scored += result.Scored()
	}
	return passed, scored, fixtures
}

// ReportSets renders a run with the two halves kept apart, and states the gap.
//
// The gap is the only number here that is not already in the per-fixture lines,
// and it is the one worth reading: it is the difference between "the agent got
// better" and "the suite got easier".
func ReportSets(suite Suite, results []SampledResult) (string, bool) {
	body, ok := ReportSampled(results)

	tunePassed, tuneScored, tuneCount := rate(suite.Tune(), results)
	heldPassed, heldScored, heldCount := rate(suite.Reported(), results)

	var b strings.Builder
	b.WriteString(body)

	// Nothing to compare: one half was not run. Say which, rather than printing
	// a gap computed against zero.
	if tuneCount == 0 || heldCount == 0 {
		ran, missing := "Tune", "held-out"
		if tuneCount == 0 {
			ran, missing = "Held-out", "tune"
		}
		fmt.Fprintf(&b, "\n%s set only - no %s fixtures in this run, so the two rates cannot be compared\n", ran, missing)
		return b.String(), ok
	}

	fmt.Fprintf(&b, "\ntune     %s across %d fixture(s)\n", ratio(tunePassed, tuneScored), tuneCount)
	fmt.Fprintf(&b, "held-out %s across %d fixture(s)\n", ratio(heldPassed, heldScored), heldCount)
	fmt.Fprintf(&b, "%s\n", gapNote(tunePassed, tuneScored, heldPassed, heldScored))
	b.WriteString(derivedReports(results))
	return b.String(), ok
}

// derivedReports are the two readings taken from the record rather than the
// verdict: what the quality gate did, and what the runs cost.
//
// Printed with the suite because a report nobody runs is a report nobody reads.
// Both stay silent when they have nothing to say - a gate that never ran and a
// suite with no tool calls each print nothing, so the common case adds no noise.
func derivedReports(results []SampledResult) string {
	var trajectories []Trajectory
	for _, result := range results {
		for _, attempt := range result.Attempts {
			if attempt.Infra == "" {
				trajectories = append(trajectories, attempt.Trajectory)
			}
		}
	}
	if len(trajectories) == 0 {
		return ""
	}

	var b strings.Builder
	if judge := ReportOnTheJudge(trajectories); judge.Judged > 0 {
		b.WriteString("\n" + judge.String())
	}

	total := Effort{Repeats: map[string]int{}, PerAction: map[string]int{}}
	for _, traj := range trajectories {
		one := EffortOf(traj)
		total.Calls += one.Calls
		total.Distinct += one.Distinct
		for name, n := range one.PerAction {
			total.PerAction[name] += n
		}
		for name, n := range one.Repeats {
			total.Repeats[name] += n
		}
	}
	if rendered := total.String(); rendered != "" {
		b.WriteString("\neffort across the suite: " + rendered)
	}
	return b.String()
}

func ratio(passed, scored int) string {
	if scored == 0 {
		return "no scored attempts"
	}
	return fmt.Sprintf("%d/%d (%d%%)", passed, scored, passed*100/scored)
}

// gapNote says what the difference between the two rates means, in words,
// because a reader who has to work out the sign of a subtraction will not.
func gapNote(tunePassed, tuneScored, heldPassed, heldScored int) string {
	if tuneScored == 0 || heldScored == 0 {
		return "gap     unknown - one half produced no scored attempts"
	}
	tune := tunePassed * 100 / tuneScored
	held := heldPassed * 100 / heldScored
	switch difference := tune - held; {
	case difference >= 40:
		return fmt.Sprintf("gap      %d points - the tune set is being fitted; treat its rate as meaningless", difference)
	case difference >= 15:
		return fmt.Sprintf("gap      %d points - the improvements are not fully generalising", difference)
	case difference <= -15:
		return fmt.Sprintf("gap      %d points - the held-out set is EASIER than the tune set, which means the two are not comparable; the split needs rethinking", difference)
	default:
		return fmt.Sprintf("gap      %d points - the two sets agree, so the tune rate can be believed", difference)
	}
}

// SetsOf reports which fixtures are in which half, for a report header.
func SetsOf(suite Suite) string {
	tune, held := suite.Tune().Names(), suite.Reported().Names()
	sort.Strings(tune)
	sort.Strings(held)
	return fmt.Sprintf("tune: %s\nheld-out: %s", strings.Join(tune, ", "), strings.Join(held, ", "))
}
