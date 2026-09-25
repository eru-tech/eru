package eval

import (
	"strings"
	"testing"

	"github.com/eru-tech/eru/eru-ai/agents"
)

func withVerdicts(verdicts ...agents.QualityVerdict) Trajectory {
	return FromRun("build it", agents.AgentMessage{
		Metrics: &agents.ExecutionMetrics{Quality: verdicts},
	})
}

// A fault the gate saw and could not stop is a different failure from one the
// rubric never described, and this is the only place the first is visible.
func TestShippingBelowTheBarIsReported(t *testing.T) {
	reason := PagesAreWorthShipping().Check(withVerdicts(
		agents.QualityVerdict{Rating: agents.QualityPoor, Reason: "raw column headings", Attempt: 1, Enforced: true},
		agents.QualityVerdict{Rating: agents.QualityPoor, Reason: "still raw column headings", Attempt: 2},
	))
	if reason == "" {
		t.Fatal("a page delivered after an unenforceable poor verdict must be reported")
	}
	if !strings.Contains(reason, "still raw column headings") {
		t.Errorf("the reason must carry the judge's own words: %s", reason)
	}
}

// The gate doing its job is a pass, not a failure: the first attempt was poor,
// the second was not, and the user got the better page.
func TestAFixedPagePasses(t *testing.T) {
	reason := PagesAreWorthShipping().Check(withVerdicts(
		agents.QualityVerdict{Rating: agents.QualityPoor, Reason: "raw column headings", Attempt: 1, Enforced: true},
		agents.QualityVerdict{Rating: agents.QualityGood, Attempt: 2},
	))
	if reason != "" {
		t.Errorf("the gate working is not a failure: %s", reason)
	}
}

func TestAcceptableIsNotAFailure(t *testing.T) {
	if reason := PagesAreWorthShipping().Check(withVerdicts(
		agents.QualityVerdict{Rating: agents.QualityAcceptable, Reason: "tile order", Attempt: 1},
	)); reason != "" {
		t.Errorf("acceptable ships: %s", reason)
	}
}

// An agent type with no judge must not fail this assertion; that is what
// TheQualityGateRan is for.
func TestNoVerdictIsNotAnObjection(t *testing.T) {
	if reason := PagesAreWorthShipping().Check(withVerdicts()); reason != "" {
		t.Errorf("nothing judged is nothing to object to: %s", reason)
	}
}

// A gate that failed silently looks exactly like a gate that was satisfied.
func TestAGateThatNeverRanIsCaught(t *testing.T) {
	if reason := TheQualityGateRan().Check(withVerdicts()); reason == "" {
		t.Error("no verdict at all must be reported by this assertion")
	}
	if reason := TheQualityGateRan().Check(withVerdicts(
		agents.QualityVerdict{Rating: agents.QualityGood, Attempt: 1},
	)); reason != "" {
		t.Errorf("a verdict is a verdict: %s", reason)
	}
}

// The verdicts have to survive the reply-to-trajectory reduction, or every
// assertion above is checking an empty slice.
func TestVerdictsSurviveIntoTheTrajectory(t *testing.T) {
	traj := withVerdicts(
		agents.QualityVerdict{Rating: agents.QualityPoor, Reason: "r", Attempt: 1, Enforced: true},
	)
	if len(traj.Quality) != 1 {
		t.Fatalf("verdicts lost on the way into the trajectory: %+v", traj.Quality)
	}
	if !traj.Quality[0].Enforced || traj.Quality[0].Attempt != 1 {
		t.Errorf("the verdict arrived altered: %+v", traj.Quality[0])
	}
}
