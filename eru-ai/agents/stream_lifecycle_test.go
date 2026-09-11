package agents

import (
	"context"
	"testing"
	"time"
)

func collector() (context.Context, *[]StreamEvent) {
	captured := &[]StreamEvent{}
	ctx := WithStreamCallback(context.Background(), func(event StreamEvent) {
		*captured = append(*captured, event)
	})
	return ctx, captured
}

func TestEmitDoesNothingWithoutAStream(t *testing.T) {
	// Every lifecycle emitter runs on non-streamed requests too, so they must be
	// silent rather than panic.
	Emit(context.Background(), StreamEvent{Event: StreamEventRunStarted})
	EmitStepStarted(context.Background(), StepGenerate, 1)
	EmitStepFinished(context.Background(), StepGenerate, 1, OutcomeSuccess, time.Now(), "", "")
}

func TestEmitStampsTheRunIdentity(t *testing.T) {
	ctx, captured := collector()
	ctx = WithRunIdentity(ctx, RunIdentity{RunId: "run_1", ThreadId: "conv_1"})

	Emit(ctx, StreamEvent{Event: StreamEventAgentStarted})
	if len(*captured) != 1 {
		t.Fatalf("expected one event, got %d", len(*captured))
	}
	event := (*captured)[0]
	if event.RunId != "run_1" || event.ThreadId != "conv_1" {
		t.Errorf("the event was not identified: %+v", event)
	}
}

func TestIdentifyNeverRelabelsARelayedEvent(t *testing.T) {
	// A sub-agent's event travels up through pods that each have a run identity
	// of their own; the one that produced it owns it.
	original := StreamEvent{Event: StreamEventThinking, RunId: "inner", ThreadId: "inner_thread"}
	relayed := original.Identify(RunIdentity{RunId: "outer", ThreadId: "outer_thread"})
	if relayed.RunId != "inner" || relayed.ThreadId != "inner_thread" {
		t.Errorf("a relaying hop overwrote the run identity: %+v", relayed)
	}

	unset := StreamEvent{Event: StreamEventThinking}.Identify(RunIdentity{RunId: "outer", ThreadId: "outer_thread"})
	if unset.RunId != "outer" || unset.ThreadId != "outer_thread" {
		t.Errorf("an unidentified event was not stamped: %+v", unset)
	}
}

func TestStepEventsCarryTheirPhaseAndOutcome(t *testing.T) {
	ctx, captured := collector()

	started := time.Now().Add(-25 * time.Millisecond)
	EmitStepStarted(ctx, StepValidate, 2)
	EmitStepFinished(ctx, StepValidate, 2, OutcomeRetry, started, "unknown component type", CodeOutputValidation)

	if len(*captured) != 2 {
		t.Fatalf("expected two events, got %d", len(*captured))
	}

	begin, ok := (*captured)[0].Data.(StepPayload)
	if !ok {
		t.Fatalf("step_started data is %T", (*captured)[0].Data)
	}
	if (*captured)[0].Event != StreamEventStepStarted || begin.Step != StepValidate || begin.Attempt != 2 {
		t.Errorf("step_started = %+v / %+v", (*captured)[0], begin)
	}

	end, ok := (*captured)[1].Data.(StepPayload)
	if !ok {
		t.Fatalf("step_finished data is %T", (*captured)[1].Data)
	}
	if (*captured)[1].Event != StreamEventStepFinished {
		t.Errorf("event = %q", (*captured)[1].Event)
	}
	if end.Outcome != OutcomeRetry || end.Code != CodeOutputValidation {
		t.Errorf("outcome = %q, code = %q", end.Outcome, end.Code)
	}
	if end.Detail != "unknown component type" {
		t.Errorf("detail = %q", end.Detail)
	}
	if end.DurationMs < 20 {
		t.Errorf("duration_ms = %d, expected the elapsed time since the step started", end.DurationMs)
	}
}

func TestLifecycleEventNamesAreStable(t *testing.T) {
	// Clients switch on these strings; renaming one is a breaking change and
	// should have to be done here, deliberately.
	for name, want := range map[string]string{
		StreamEventRunStarted:    "run_started",
		StreamEventAgentStarted:  "agent_started",
		StreamEventAgentFinished: "agent_finished",
		StreamEventStepStarted:   "step_started",
		StreamEventStepFinished:  "step_finished",
		StreamEventPageComponent: "page_component",
		StreamEventDone:          "done",
		StreamEventError:         "error",
		StreamEventQuestion:      "question",
	} {
		if name != want {
			t.Errorf("event name %q should be %q", name, want)
		}
	}
}
