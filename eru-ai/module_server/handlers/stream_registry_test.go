package handlers

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
)

// TestStreamSinkSingleWriterUnderConcurrentProducers is the case the design
// exists for: the local agent loop plus many sub-agent callback goroutines all
// producing at once, while exactly one goroutine writes the connection. Run under
// -race, any write escaping to a second goroutine trips the detector.
func TestStreamSinkSingleWriterUnderConcurrentProducers(t *testing.T) {
	sink := newStreamSink()

	var written []agents.StreamEvent
	var writerGoroutines sync.Map
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		sink.drain(func(event agents.StreamEvent) {
			// Deliberately unsynchronised: only ever safe because drain is the
			// single owner. -race fails here if that stops being true.
			written = append(written, event)
			writerGoroutines.Store(currentGoroutineId(), true)
		})
	}()

	const producers = 16
	const perProducer = 20
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				sink.send(agents.StreamEvent{Event: "thinking", Agent: fmt.Sprint("agent", p)})
			}
		}(p)
	}
	wg.Wait()
	sink.finish()
	<-writerDone

	if len(written) == 0 {
		t.Fatal("expected the writer to deliver events")
	}
	if len(written) > producers*perProducer {
		t.Fatalf("writer delivered more events than were sent : %d", len(written))
	}
	count := 0
	writerGoroutines.Range(func(_, _ any) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected exactly one writing goroutine, got %d", count)
	}
}

func TestStreamSinkNeverBlocksWhenFull(t *testing.T) {
	sink := newStreamSink()
	for i := 0; i < streamSinkBuffer; i++ {
		if !sink.send(agents.StreamEvent{Event: "thinking"}) {
			t.Fatalf("send %d should have been accepted", i)
		}
	}
	done := make(chan bool, 1)
	go func() { done <- sink.send(agents.StreamEvent{Event: "thinking"}) }()
	select {
	case accepted := <-done:
		if accepted {
			t.Error("expected the overflow event to be dropped")
		}
	case <-time.After(time.Second):
		t.Fatal("send blocked on a full buffer")
	}
}

func TestStreamSinkDropsAfterFinish(t *testing.T) {
	sink := newStreamSink()
	sink.finish()
	if sink.send(agents.StreamEvent{Event: "thinking"}) {
		t.Error("expected events to be dropped once the stream finished")
	}
}

func TestStreamSinkFinishIsIdempotent(t *testing.T) {
	sink := newStreamSink()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); sink.finish() }()
	}
	wg.Wait()
}

func TestStreamRegistryPublishRoutesToOwningStream(t *testing.T) {
	sinkA := activeStreams.open("stream-a")
	sinkB := activeStreams.open("stream-b")
	defer activeStreams.close("stream-a")
	defer activeStreams.close("stream-b")

	accepted := activeStreams.publish("stream-a", []agents.StreamEvent{
		{Event: "thinking", Agent: "sub1"},
		{Event: "thinking", Agent: "sub1"},
	})
	if accepted != 2 {
		t.Fatalf("expected 2 accepted, got %d", accepted)
	}
	if len(sinkA.events) != 2 {
		t.Errorf("expected 2 events queued on stream-a, got %d", len(sinkA.events))
	}
	if len(sinkB.events) != 0 {
		t.Errorf("stream-b must not receive stream-a's events, got %d", len(sinkB.events))
	}
}

func TestStreamRegistryPublishToUnknownStreamIsNoOp(t *testing.T) {
	if accepted := activeStreams.publish("no-such-stream", []agents.StreamEvent{{Event: "thinking"}}); accepted != 0 {
		t.Errorf("expected 0 accepted for an unknown stream, got %d", accepted)
	}
}

func TestStreamRegistryPublishAfterCloseIsNoOp(t *testing.T) {
	activeStreams.open("stream-c")
	activeStreams.close("stream-c")
	if accepted := activeStreams.publish("stream-c", []agents.StreamEvent{{Event: "thinking"}}); accepted != 0 {
		t.Errorf("expected 0 accepted after close, got %d", accepted)
	}
}

func TestStreamRegistryConcurrentOpenPublishClose(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprint("stream-", i)
			activeStreams.open(id)
			activeStreams.publish(id, []agents.StreamEvent{{Event: "thinking"}})
			activeStreams.close(id)
		}(i)
	}
	wg.Wait()
	activeStreams.mu.RLock()
	remaining := len(activeStreams.sinks)
	activeStreams.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("expected all streams deregistered, got %d", remaining)
	}
}

// currentGoroutineId identifies the calling goroutine so the test can assert that
// only one ever writes.
func currentGoroutineId() int {
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	buf = bytes.TrimPrefix(buf, []byte("goroutine "))
	id, _ := strconv.Atoi(string(buf[:bytes.IndexByte(buf, ' ')]))
	return id
}
