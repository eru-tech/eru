package agents

import (
	"context"
	"sync"
	"testing"
)

func TestStreamCallbackContextRoundtrip(t *testing.T) {
	ctx := context.Background()

	cb := GetStreamCallback(ctx)
	if cb != nil {
		t.Fatal("expected nil callback from bare context")
	}

	var received []StreamEvent
	testCb := StreamCallback(func(event StreamEvent) {
		received = append(received, event)
	})

	ctx = WithStreamCallback(ctx, testCb)
	cb = GetStreamCallback(ctx)
	if cb == nil {
		t.Fatal("expected callback from context")
	}

	cb(StreamEvent{Event: StreamEventTextDelta, Data: "hello"})
	cb(StreamEvent{Event: StreamEventDone, Data: "final"})

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if received[0].Event != StreamEventTextDelta {
		t.Errorf("expected text_delta, got %s", received[0].Event)
	}
	if received[1].Event != StreamEventDone {
		t.Errorf("expected done, got %s", received[1].Event)
	}
}

func TestStreamCallbackNilSafe(t *testing.T) {
	ctx := context.Background()
	cb := GetStreamCallback(ctx)
	if cb != nil {
		t.Fatal("expected nil callback")
	}
}

func TestStreamEventTypes(t *testing.T) {
	types := []string{
		StreamEventThinking,
		StreamEventToolUse,
		StreamEventToolResult,
		StreamEventTextDelta,
		StreamEventDone,
		StreamEventError,
	}
	seen := map[string]bool{}
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate event type: %s", typ)
		}
		seen[typ] = true
		if typ == "" {
			t.Error("event type should not be empty")
		}
	}
}

func TestStreamEventStruct(t *testing.T) {
	event := StreamEvent{
		Event:     StreamEventToolUse,
		Data:      map[string]string{"tool": "search"},
		Iteration: 3,
	}
	if event.Event != "tool_use" {
		t.Errorf("unexpected event: %s", event.Event)
	}
	if event.Iteration != 3 {
		t.Errorf("unexpected iteration: %d", event.Iteration)
	}
}

func TestStreamCallbackConcurrency(t *testing.T) {
	var mu sync.Mutex
	var received []StreamEvent

	testCb := StreamCallback(func(event StreamEvent) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, event)
	})

	ctx := WithStreamCallback(context.Background(), testCb)
	cb := GetStreamCallback(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cb(StreamEvent{Event: StreamEventTextDelta, Data: n})
		}(i)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 50 {
		t.Errorf("expected 50 events, got %d", len(received))
	}
}
