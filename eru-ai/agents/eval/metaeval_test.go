package eval

import (
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
)

func judged(verdicts ...agents.QualityVerdict) Trajectory {
	return Trajectory{Quality: verdicts}
}

func poor(reason string, attempt int, enforced bool) agents.QualityVerdict {
	return agents.QualityVerdict{Rating: agents.QualityPoor, Reason: reason, Attempt: attempt, Enforced: enforced}
}

func good(attempt int) agents.QualityVerdict {
	return agents.QualityVerdict{Rating: agents.QualityGood, Attempt: attempt}
}

// The gate earning its attempt: objected, and the next answer was better.
func TestAnImprovedRunIsCountedAsImproved(t *testing.T) {
	r := ReportOnTheJudge([]Trajectory{judged(poor("no chart title", 1, true), good(2))})
	if r.Objected != 1 || r.Improved != 1 || r.Overruled != 0 {
		t.Errorf("%+v", r)
	}
}

// The gate seeing a fault it could not stop is a different outcome and must not
// be counted as a win.
func TestAnOverruledRunIsCountedSeparately(t *testing.T) {
	r := ReportOnTheJudge([]Trajectory{judged(poor("no chart title", 1, true), poor("still no chart title", 2, false))})
	if r.Improved != 0 || r.Overruled != 1 {
		t.Errorf("%+v", r)
	}
	if !strings.Contains(r.String(), "objection standing") {
		t.Errorf("the report must surface it:\n%s", r)
	}
}

// A gate that has never once fired is indistinguishable from one switched off,
// and the report has to say so rather than read as good news.
func TestAGateThatNeverObjectsIsQuestioned(t *testing.T) {
	r := ReportOnTheJudge([]Trajectory{judged(good(1)), judged(good(1)), judged(good(1))})
	if r.Objected != 0 || r.Judged != 3 {
		t.Errorf("%+v", r)
	}
	out := r.String()
	if !strings.Contains(out, "switched off") {
		t.Errorf("silence must be questioned, not celebrated:\n%s", out)
	}
}

// Objecting constantly is a claim about the rubric as much as about the agent.
func TestObjectingOnMostRunsIsFlagged(t *testing.T) {
	r := ReportOnTheJudge([]Trajectory{
		judged(poor("a", 1, true), good(2)),
		judged(poor("a", 1, true), good(2)),
		judged(good(1)),
	})
	if !strings.Contains(r.String(), "the bar is set where ordinary work fails it") {
		t.Errorf("a high objection rate must be questioned:\n%s", r)
	}
}

// A retry that never improves anything is costing attempts for nothing.
func TestARetryThatNeverHelpsIsCalledOut(t *testing.T) {
	r := ReportOnTheJudge([]Trajectory{
		judged(poor("a", 1, true), poor("a", 2, false)),
		judged(poor("b", 1, true), poor("b", 2, false)),
	})
	if !strings.Contains(r.String(), "never once improved") {
		t.Errorf("a useless retry must be named:\n%s", r)
	}
}

// The same fault on different components is one kind of objection, or the
// tally is a list of component ids.
func TestObjectionsAreGroupedByKind(t *testing.T) {
	r := ReportOnTheJudge([]Trajectory{
		judged(poor("- line_chart has no title\n  page \"a\", chart \"x\"", 1, true), good(2)),
		judged(poor("- line_chart has no title\n  page \"b\", chart \"y\"", 1, true), good(2)),
	})
	if len(r.Reasons) != 1 {
		t.Fatalf("one kind of objection, not two: %+v", r.Reasons)
	}
	for text, n := range r.Reasons {
		if n != 2 {
			t.Errorf("%q counted %d", text, n)
		}
		if strings.Contains(text, "page") {
			t.Errorf("the per-instance detail must be dropped: %q", text)
		}
	}
}

// Runs with no verdict are not evidence about the gate either way.
func TestUnjudgedRunsAreNotCounted(t *testing.T) {
	r := ReportOnTheJudge([]Trajectory{{}, {}, judged(good(1))})
	if r.Runs != 3 || r.Judged != 1 {
		t.Errorf("%+v", r)
	}
	if !strings.Contains(ReportOnTheJudge([]Trajectory{{}, {}}).String(), "never ran") {
		t.Error("a gate that produced no verdicts at all must say so")
	}
}
