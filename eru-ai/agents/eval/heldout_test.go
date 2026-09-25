package eval

import (
	agents "github.com/eru-tech/eru/eru-ai/agents"
	"strings"
	"testing"
)

func splitSuite() Suite {
	return Suite{
		{Name: "tune_a"},
		{Name: "held_a", HeldOut: true},
		{Name: "tune_b"},
		{Name: "held_b", HeldOut: true},
	}
}

func TestTheSuiteSplitsInTwo(t *testing.T) {
	suite := splitSuite()
	if got := suite.Tune().Names(); len(got) != 2 || got[0] != "tune_a" {
		t.Errorf("tune set = %v", got)
	}
	if got := suite.Reported().Names(); len(got) != 2 || got[0] != "held_a" {
		t.Errorf("held-out set = %v", got)
	}
}

// Running fewer fixtures than intended is recoverable. Quietly burning the
// held-out set because someone typed the name wrong is not.
func TestAnUnknownSetNameIsTheTuneSet(t *testing.T) {
	for _, name := range []string{"", "heldout-please", "HELD", "everything", "tune"} {
		if got := ParseSet(name); got != SetTune {
			t.Errorf("ParseSet(%q) = %q, want tune", name, got)
		}
	}
	for _, name := range []string{"held_out", "heldout", "held-out", "HELD_OUT"} {
		if got := ParseSet(name); got != SetHeldOut {
			t.Errorf("ParseSet(%q) = %q, want held_out", name, got)
		}
	}
	if ParseSet("all") != SetAll {
		t.Error("all must select both")
	}
}

func TestSelectReturnsTheRightHalf(t *testing.T) {
	suite := splitSuite()
	if len(suite.Select(SetTune)) != 2 || len(suite.Select(SetHeldOut)) != 2 || len(suite.Select(SetAll)) != 4 {
		t.Error("Select does not partition the suite")
	}
}

func sampled(name string, passed, total int) SampledResult {
	result := SampledResult{Fixture: name}
	for i := 0; i < total; i++ {
		attempt := Attempt{Result: Result{Fixture: name}}
		if i >= passed {
			attempt.Result.Failures = []Failure{{Expectation: "x", Reason: "y"}}
		}
		result.Attempts = append(result.Attempts, attempt)
	}
	return result
}

// The gap is the only number in the report that is not already in the
// per-fixture lines, and it is the one worth reading.
func TestAWideGapIsCalledWhatItIs(t *testing.T) {
	suite := splitSuite()
	report, _ := ReportSets(suite, []SampledResult{
		sampled("tune_a", 5, 5), sampled("tune_b", 5, 5),
		sampled("held_a", 1, 5), sampled("held_b", 0, 5),
	})
	if !strings.Contains(report, "tune     10/10 (100%)") {
		t.Errorf("tune rate missing:\n%s", report)
	}
	if !strings.Contains(report, "held-out 1/10 (10%)") {
		t.Errorf("held-out rate missing:\n%s", report)
	}
	if !strings.Contains(report, "being fitted") {
		t.Errorf("a 90 point gap must say the tune set is being fitted:\n%s", report)
	}
}

func TestAgreeingSetsSayTheRateCanBeBelieved(t *testing.T) {
	report, _ := ReportSets(splitSuite(), []SampledResult{
		sampled("tune_a", 4, 5), sampled("tune_b", 4, 5),
		sampled("held_a", 4, 5), sampled("held_b", 4, 5),
	})
	if !strings.Contains(report, "can be believed") {
		t.Errorf("equal rates must be reported as agreement:\n%s", report)
	}
}

// A held-out set that is easier than the tune set measures nothing; that is a
// fault in the split, not a result, and it must not read as good news.
func TestAnEasierHeldOutSetIsReportedAsABrokenSplit(t *testing.T) {
	report, _ := ReportSets(splitSuite(), []SampledResult{
		sampled("tune_a", 1, 5), sampled("tune_b", 1, 5),
		sampled("held_a", 5, 5), sampled("held_b", 5, 5),
	})
	if !strings.Contains(report, "needs rethinking") {
		t.Errorf("an inverted gap must be flagged as a broken split:\n%s", report)
	}
}

// Running one half is the normal case, and a gap computed against an empty set
// would be a number with no meaning.
func TestOneHalfAloneReportsNoGap(t *testing.T) {
	report, _ := ReportSets(splitSuite(), []SampledResult{sampled("tune_a", 5, 5)})
	if strings.Contains(report, "\ngap") {
		t.Errorf("no gap should be computed from one half:\n%s", report)
	}
	if !strings.Contains(report, "Tune set only - no held-out fixtures") {
		t.Errorf("the report must name the set that ran and the one that did not:\n%s", report)
	}

	// And the other way round, which is the case that was reported backwards.
	heldOnly, _ := ReportSets(splitSuite(), []SampledResult{sampled("held_a", 5, 5)})
	if !strings.Contains(heldOnly, "Held-out set only - no tune fixtures") {
		t.Errorf("a held-out-only run must not be described as tune-only:\n%s", heldOnly)
	}
}

// The real suite has to actually be split, or the mechanism is decoration.
func TestTheRealSuiteHasBothSets(t *testing.T) {
	suite := Scenarios()
	if len(suite.Tune()) == 0 {
		t.Error("no tune fixtures")
	}
	if len(suite.Reported()) == 0 {
		t.Fatal("no held-out fixtures - the gap can never be measured")
	}
	// Every fixture must state why it exists, held out or not.
	for _, fixture := range suite.Reported() {
		if strings.TrimSpace(fixture.Why) == "" {
			t.Errorf("held-out fixture %q does not say what it is for", fixture.Name)
		}
	}
}

// Both derived reports are printed with the suite, because a report nobody runs
// is a report nobody reads.
func TestTheSuiteReportCarriesTheDerivedReadings(t *testing.T) {
	withRun := func(name string, traj Trajectory) SampledResult {
		return SampledResult{Fixture: name, Attempts: []Attempt{{Result: Result{Fixture: name}, Trajectory: traj}}}
	}
	suite := Suite{{Name: "tune_a"}, {Name: "held_a", HeldOut: true}}
	report, _ := ReportSets(suite, []SampledResult{
		withRun("tune_a", Trajectory{
			Quality:   []agents.QualityVerdict{{Rating: agents.QualityPoor, Reason: "no chart title", Attempt: 1, Enforced: true}, {Rating: agents.QualityGood, Attempt: 2}},
			ToolCalls: []ToolCall{{Name: "run_query", Input: map[string]interface{}{"q": "a"}}, {Name: "run_query", Input: map[string]interface{}{"q": "a"}}},
		}),
		withRun("held_a", Trajectory{}),
	})
	if !strings.Contains(report, "quality gate over") {
		t.Errorf("the judge report must be printed:\n%s", report)
	}
	if !strings.Contains(report, "effort across the suite") {
		t.Errorf("the effort report must be printed:\n%s", report)
	}
	if !strings.Contains(report, "already answered") {
		t.Errorf("the repeated call must be surfaced:\n%s", report)
	}
}

// The common case adds no noise: nothing judged and nothing called prints
// nothing.
func TestTheDerivedReadingsStaySilentWhenEmpty(t *testing.T) {
	suite := Suite{{Name: "a"}, {Name: "b", HeldOut: true}}
	report, _ := ReportSets(suite, []SampledResult{
		{Fixture: "a", Attempts: []Attempt{{Result: Result{Fixture: "a"}}}},
		{Fixture: "b", Attempts: []Attempt{{Result: Result{Fixture: "b"}}}},
	})
	if strings.Contains(report, "quality gate over") || strings.Contains(report, "effort across") {
		t.Errorf("nothing to say means nothing printed:\n%s", report)
	}
}
