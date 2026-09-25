package eval

import (
	"fmt"
	"sort"
	"strings"
)

// Sampling exists because one run of a stochastic process is not a measurement.
//
// Every number this suite produced before today came from a single sample of
// four fixtures, and decisions were taken on it. Two independent groups measured
// what that costs. HarnessX, evolving against its own single-sample score,
// peaked at 73.8% on GAIA and collapsed to 49.5% by round 15 without noticing
// until they plotted it. DarwinX cite Bjarnason et al. measuring 2.2-6.0 point
// pass@1 swings on SWE-bench Verified AT TEMPERATURE ZERO - "often the magnitude
// of a single accepted edit". A change smaller than the noise is indistinguishable
// from no change, and a suite that cannot tell them apart will confidently report
// either one.
//
// So a fixture is run k times and the result carries the spread. The point is
// not a better number; it is knowing when the number means nothing.

// Attempt is one sample of one fixture: what it scored, or why it never ran.
type Attempt struct {
	Result Result
	// Trajectory is what the run produced, kept so reports that read the RECORD
	// rather than the verdict - what the gate objected to, what the run cost -
	// can be derived afterwards instead of needing their own plumbing.
	Trajectory Trajectory
	// Infra is set when the attempt did not happen - the stack was down, the
	// request timed out, the reply would not decode. That is not the agent
	// performing badly, and scoring it as such would blame the agent for a
	// broken VPN. Excluded from the rate, counted and reported separately.
	Infra string
}

// SampledResult is one fixture over k attempts.
type SampledResult struct {
	Fixture string
	// Attempts in order, including the ones that never ran.
	Attempts []Attempt
	// Blocked is carried up from the attempts: a fixture that cannot be judged
	// cannot be judged however many times it is tried.
	Blocked string
}

// Scored is the number of attempts that actually produced a judgement.
func (s SampledResult) Scored() int {
	n := 0
	for _, a := range s.Attempts {
		if a.Infra == "" && a.Result.Blocked == "" {
			n++
		}
	}
	return n
}

// Passed is how many scored attempts held every expectation.
func (s SampledResult) Passed() int {
	n := 0
	for _, a := range s.Attempts {
		if a.Infra == "" && a.Result.Blocked == "" && a.Result.OK() {
			n++
		}
	}
	return n
}

// InfraFailures is how many attempts never ran.
func (s SampledResult) InfraFailures() int {
	n := 0
	for _, a := range s.Attempts {
		if a.Infra != "" {
			n++
		}
	}
	return n
}

// Rate is the fraction of scored attempts that passed, and whether there were
// any to divide by.
func (s SampledResult) Rate() (float64, bool) {
	scored := s.Scored()
	if scored == 0 {
		return 0, false
	}
	return float64(s.Passed()) / float64(scored), true
}

// OK says whether this fixture held.
//
// Every scored attempt must pass. A fixture that passes two times in three is
// NOT a pass - it is a fixture telling you something you did not know, and
// rounding it up to green throws that away. It is reported as FLAKY so the
// remediation is obvious: a consistent failure is a defect, an inconsistent one
// is either a real intermittency or an assertion that is too tight.
//
// Blocked and all-infra are not failures: nothing is known to be wrong.
func (s SampledResult) OK() bool {
	if s.Blocked != "" || s.Scored() == 0 {
		return true
	}
	return s.Passed() == s.Scored()
}

// expectationRates is the pass rate of each individual check across attempts, so
// a flaky fixture names the check that wobbled rather than just the scenario.
func (s SampledResult) expectationRates() []string {
	// Every scored attempt ran every check the fixture declares, so the
	// denominator is the number of scored attempts - not the number of attempts
	// since this check was first seen to fail. Counting from first failure
	// reported "failed 1/2" for a check that failed once in three, which
	// understates how reliable it actually was.
	scored := s.Scored()
	failures := map[string]int{}
	for _, a := range s.Attempts {
		if a.Infra != "" || a.Result.Blocked != "" {
			continue
		}
		failedHere := map[string]bool{}
		for _, f := range a.Result.Failures {
			failedHere[f.Expectation] = true
		}
		for check := range failedHere {
			failures[check]++
		}
	}
	order := make([]string, 0, len(failures))
	for check := range failures {
		order = append(order, check)
	}
	sort.Strings(order)
	out := make([]string, 0, len(order))
	for _, check := range order {
		out = append(out, fmt.Sprintf("%s — failed %d/%d", check, failures[check], scored))
	}
	return out
}

// reasons collects the distinct explanations a check gave, so a flaky failure
// shows whether it failed the same way twice or two different ways.
func (s SampledResult) reasons() []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range s.Attempts {
		for _, f := range a.Result.Failures {
			line := fmt.Sprintf("%s: %s", f.Expectation, f.Reason)
			if !seen[line] {
				seen[line] = true
				out = append(out, line)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (s SampledResult) String() string {
	if s.Blocked != "" {
		return fmt.Sprintf("%-38s BLOCKED - %s", s.Fixture, s.Blocked)
	}
	scored, passed, infra := s.Scored(), s.Passed(), s.InfraFailures()
	if scored == 0 {
		return fmt.Sprintf("%-38s BLOCKED - no attempt ran (%d infra failure(s)); the last said: %s",
			s.Fixture, infra, s.lastInfra())
	}

	suffix := ""
	if infra > 0 {
		suffix = fmt.Sprintf("  [%d infra failure(s) excluded]", infra)
	}

	var b strings.Builder
	switch {
	case passed == scored:
		fmt.Fprintf(&b, "%-38s PASS %d/%d%s", s.Fixture, passed, scored, suffix)
	case passed == 0:
		fmt.Fprintf(&b, "%-38s FAIL %d/%d%s", s.Fixture, passed, scored, suffix)
	default:
		fmt.Fprintf(&b, "%-38s FLAKY %d/%d — passed sometimes, which is not a pass%s", s.Fixture, passed, scored, suffix)
	}
	for _, rate := range s.expectationRates() {
		fmt.Fprintf(&b, "\n    · %s", rate)
	}
	for _, reason := range s.reasons() {
		fmt.Fprintf(&b, "\n    - %s", reason)
	}
	return b.String()
}

func (s SampledResult) lastInfra() string {
	for i := len(s.Attempts) - 1; i >= 0; i-- {
		if s.Attempts[i].Infra != "" {
			return s.Attempts[i].Infra
		}
	}
	return ""
}

// ReportSampled renders a sampled run and says whether it held.
func ReportSampled(results []SampledResult) (string, bool) {
	var b strings.Builder
	ok := true
	for _, result := range results {
		if !result.OK() {
			ok = false
		}
		fmt.Fprintln(&b, result.String())
	}
	return b.String(), ok
}
