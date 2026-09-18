package functions

import (
	"context"
	"os"
	"testing"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func TestMain(m *testing.M) {
	logs.LogInit("test", "eru-functions-test")
	os.Exit(m.Run())
}

func TestDependencyIsDispatchedBeforeTheStepWaitingOnIt(t *testing.T) {
	steps := map[string]*FuncStep{
		"eru_studio": {WaitFor: "processo"},
		"processo":   {},
	}
	for i := 0; i < 50; i++ {
		ordered := orderedFuncStepKeys(steps)
		if len(ordered) != 2 {
			t.Fatalf("expected both steps, got %v", ordered)
		}
		if ordered[0] != "processo" {
			t.Fatalf("the awaited step must be dispatched first, got %v", ordered)
		}
	}
}

func TestOrderIsStableWhenNothingWaits(t *testing.T) {
	steps := map[string]*FuncStep{"b": {}, "a": {}, "c": {}}
	first := orderedFuncStepKeys(steps)
	for i := 0; i < 20; i++ {
		if got := orderedFuncStepKeys(steps); got[0] != first[0] || got[1] != first[1] || got[2] != first[2] {
			t.Fatalf("order is not stable: %v then %v", first, got)
		}
	}
}

func TestACycleStillDispatchesEveryStep(t *testing.T) {
	steps := map[string]*FuncStep{"a": {WaitFor: "b"}, "b": {WaitFor: "a"}}
	ordered := orderedFuncStepKeys(steps)
	if len(ordered) != 2 {
		t.Fatalf("a cycle must not drop steps, got %v", ordered)
	}
}

func TestWaitForAStepOutsideThisMapDoesNotHoldUpDispatch(t *testing.T) {
	steps := map[string]*FuncStep{"only": {WaitFor: "somewhere_else"}}
	if ordered := orderedFuncStepKeys(steps); len(ordered) != 1 || ordered[0] != "only" {
		t.Fatalf("got %v", ordered)
	}
}

func TestWaitForStepGivesUpWhenTheRequestIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(InitStepSync(context.Background()))
	done := make(chan struct{})
	go func() {
		WaitForStep(ctx, "never_runs")
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForStep ignored cancellation and blocked")
	}
}

func TestWaitForStepReturnsWhatTheStepSignalled(t *testing.T) {
	ctx := InitStepSync(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		signalStep(ctx, "ready", map[string]FuncTemplateVars{"ready": {}}, nil)
	}()
	res, _ := WaitForStep(ctx, "ready")
	if _, ok := res["ready"]; !ok {
		t.Fatalf("the awaited step's vars did not come back: %v", res)
	}
}
