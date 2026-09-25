package eval

import (
	"fmt"
	"sort"
	"strings"

	agents "github.com/eru-tech/eru/eru-ai/agents"
)

// Judging the judge.
//
// The quality gate decides whether an answer is good enough, spends an attempt
// on that decision, and records a verdict. Everything so far has taken those
// verdicts at face value. But a judge is a rubric applied by a machine, and a
// rubric can be wrong in two directions that look nothing alike from inside:
//
//   - it objects to answers that were fine, and every run pays an extra attempt
//     for nothing;
//   - it approves answers that were not, and the gate is decoration.
//
// Neither is visible in a pass rate. A suite where the gate fires constantly and
// a suite where it never fires both look like "the gate is on".
//
// This reads the verdicts a run recorded and reports what the gate actually did.
// It is a meta-eval in the honest, limited sense: it measures the gate's
// BEHAVIOUR, not its correctness. Correctness needs a human to say whether a
// page they were shown was worth sending back, and nothing here can stand in for
// that. What it can do is put the question in front of someone - a gate that
// objected to nine runs out of ten is making a claim about the agent that
// somebody should either believe or fix.

// JudgeReport is what the quality gate did across a set of runs.
type JudgeReport struct {
	Runs int
	// Judged is runs where the gate reached any verdict at all.
	Judged int
	// Objected is runs where it rated something poor.
	Objected int
	// Improved is runs where it objected and the next attempt rated good - the
	// gate earning its attempt.
	Improved int
	// Overruled is runs where it objected and the answer shipped anyway, because
	// the budget was gone. The gate saw a fault it could not stop.
	Overruled int
	// Reasons are the distinct objections, most frequent first.
	Reasons map[string]int
}

// ReportOnTheJudge reads the recorded verdicts of a set of runs.
func ReportOnTheJudge(trajectories []Trajectory) JudgeReport {
	report := JudgeReport{Runs: len(trajectories), Reasons: map[string]int{}}
	for _, traj := range trajectories {
		if len(traj.Quality) == 0 {
			continue
		}
		report.Judged++

		objected := false
		for _, verdict := range traj.Quality {
			if verdict.Rating.RetryWorthy() {
				objected = true
				if reason := firstLineOf(verdict.Reason); reason != "" {
					report.Reasons[reason]++
				}
			}
		}
		if !objected {
			continue
		}
		report.Objected++

		final := traj.Quality[len(traj.Quality)-1]
		switch {
		case final.Rating.RetryWorthy():
			// It objected, and the last word is still an objection: the answer
			// went out with the fault standing.
			report.Overruled++
		case final.Rating == agents.QualityGood:
			report.Improved++
		}
	}
	return report
}

// firstLineOf reduces a multi-line objection to its heading, so two reports of
// the same fault on different components count as one kind of objection.
func firstLineOf(reason string) string {
	for _, line := range strings.Split(reason, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if line != "" {
			if len(line) > 90 {
				line = line[:90] + "..."
			}
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// String renders the report, and says what the numbers would mean.
//
// The interpretation is included deliberately. A bare "objected 9/10" invites
// the reading "the gate is working hard", when it is at least as likely to mean
// the rubric is too strict and every run is paying for it.
func (r JudgeReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "quality gate over %d run(s): judged %d, objected %d, improved %d, overruled %d\n",
		r.Runs, r.Judged, r.Objected, r.Improved, r.Overruled)

	if r.Judged == 0 {
		b.WriteString("  the gate never ran - either no agent here implements one, or the runs carry no verdicts\n")
		return b.String()
	}
	switch {
	case r.Objected == 0:
		b.WriteString("  it never objected. Either the work is consistently good or the rubric is not reaching it;\n" +
			"  a gate that has never once fired is indistinguishable from one that is switched off\n")
	case r.Objected*2 > r.Judged:
		fmt.Fprintf(&b, "  it objected on more than half the runs, which is a claim about the rubric as much as the agent:\n"+
			"  either the agent is reliably missing something, or the bar is set where ordinary work fails it\n")
	}
	if r.Objected > 0 && r.Improved == 0 {
		b.WriteString("  it never once improved an answer it sent back. The retry is costing attempts and returning nothing,\n" +
			"  which is the case for narrowing the rubric or for repairing rather than regenerating\n")
	}
	if r.Overruled > 0 {
		fmt.Fprintf(&b, "  %d run(s) shipped with the objection standing - the gate saw a fault it had no attempt left to fix\n", r.Overruled)
	}

	if len(r.Reasons) > 0 {
		type reason struct {
			text string
			n    int
		}
		list := make([]reason, 0, len(r.Reasons))
		for text, n := range r.Reasons {
			list = append(list, reason{text, n})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].n != list[j].n {
				return list[i].n > list[j].n
			}
			return list[i].text < list[j].text
		})
		b.WriteString("  what it objected to:\n")
		for _, item := range list {
			fmt.Fprintf(&b, "    %2d x %s\n", item.n, item.text)
		}
	}
	return b.String()
}
