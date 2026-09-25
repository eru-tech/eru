package eval

import (
	"context"
	"os"
	"strconv"
	"testing"
)

// A live run writes to a real workspace and takes minutes, so it never happens
// by accident: `go test ./...` skips it, and it runs only when asked for.
//
//	ERU_EVAL_LIVE=1 \
//	ERU_EVAL_TENANT=<org_process_id> \
//	ERU_EVAL_CLAIMS='<claims header>' \
//	ERU_EVAL_RECORD=./local-testdata/trajectories \
//	ERU_EVAL_SAMPLES=3 \
//	go test ./agents/eval -run TestLive -v -timeout 90m
func TestLiveScenarios(t *testing.T) {
	if os.Getenv("ERU_EVAL_LIVE") != "1" {
		t.Skip("set ERU_EVAL_LIVE=1 to run the scenarios against a live stack")
	}
	runner, err := RunnerFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	// The tune set by default. The held-out fixtures are measured deliberately
	// or not at all: a suite that runs them on every iteration is a suite whose
	// held-out set has been tuned against by the end of the week.
	all := Scenarios()
	suite := all.Select(ParseSet(os.Getenv("ERU_EVAL_SET")))
	if only := os.Getenv("ERU_EVAL_ONLY"); only != "" {
		fixture, found := all.ByName(only)
		if !found {
			t.Fatalf("no scenario named %q; have %v", only, all.Names())
		}
		suite = Suite{fixture}
	}
	if len(suite) == 0 {
		t.Skipf("no fixtures selected; the suite holds\n%s", SetsOf(all))
	}

	results, err := runner.RunAllSampled(context.Background(), suite, recordDir(), samples())
	if err != nil {
		t.Fatal(err)
	}
	report, ok := ReportSets(suite, results)
	t.Log("\n" + report)
	if !ok {
		t.Error("one or more scenarios failed - see the report above")
	}
}

// samples is a global OVERRIDE, for investigating one flake without editing a
// fixture. The normal place to say how many attempts a scenario needs is
// Fixture.Samples, because the right number depends on what the scenario costs.
// Zero means "let each fixture decide".
func samples() int {
	n, err := strconv.Atoi(os.Getenv("ERU_EVAL_SAMPLES"))
	if err != nil || n < 1 {
		return 0
	}
	return n
}

// Recorded runs are scored offline, which is how a change to an expectation gets
// tested against real trajectories without paying for them again.
func TestRecordedScenarios(t *testing.T) {
	dir := recordDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no recorded trajectories in %s", dir)
	}
	results, err := ScoreRecorded(dir, Scenarios())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Skip("no recordings matched a scenario name")
	}
	report, ok := Report(results)
	t.Log("\n" + report)
	if !ok {
		t.Error("a recorded run no longer satisfies its scenario")
	}
}

// recordDir is where live runs are written and replays are read from, so the two
// cannot drift apart again.
func recordDir() string {
	if dir := os.Getenv("ERU_EVAL_RECORD"); dir != "" {
		return dir
	}
	return DefaultRecordDir
}
