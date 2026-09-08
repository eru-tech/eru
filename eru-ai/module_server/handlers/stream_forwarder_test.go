package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	"github.com/gorilla/mux"
)

func muxVars(r *http.Request, vars map[string]string) *http.Request {
	return mux.SetURLVars(r, vars)
}

type batchRecorder struct {
	mu      sync.Mutex
	batches [][]agents.StreamEvent
	paths   []string
}

func (b *batchRecorder) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var batch streamEventBatch
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b.mu.Lock()
		b.batches = append(b.batches, batch.Events)
		b.paths = append(b.paths, r.URL.Path)
		b.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}
}

func (b *batchRecorder) all() []agents.StreamEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []agents.StreamEvent
	for _, batch := range b.batches {
		out = append(out, batch...)
	}
	return out
}

func TestStreamForwarderBatchesAndAttributes(t *testing.T) {
	rec := &batchRecorder{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	f := newStreamForwarder(agents.StreamTarget{StreamId: "s1", CallbackUrl: srv.URL}, "processo", "processo", "generate_sql", []string{"root_orchestrator"})
	cb := f.Callback()
	for i := 0; i < 10; i++ {
		cb(agents.StreamEvent{Event: agents.StreamEventThinking, Data: "chunk"})
	}
	f.Close()

	events := rec.all()
	if len(events) != 10 {
		t.Fatalf("expected 10 forwarded events, got %d", len(events))
	}
	rec.mu.Lock()
	batches := len(rec.batches)
	path := rec.paths[0]
	rec.mu.Unlock()
	if batches != 1 {
		t.Errorf("expected the 10 events to be batched into 1 post, got %d posts", batches)
	}
	if path != "/processo/processo/stream/s1/event" {
		t.Errorf("unexpected callback path : %s", path)
	}
	for i, event := range events {
		if event.Agent != "generate_sql" {
			t.Errorf("event %d not attributed to the emitting agent : %+v", i, event)
		}
		if event.Chain != "root_orchestrator/generate_sql" {
			t.Errorf("event %d has the wrong chain : %s", i, event.Chain)
		}
		if event.Seq != int64(i+1) {
			t.Errorf("event %d has seq %d", i, event.Seq)
		}
	}
}

func TestStreamForwarderKeepsSourceAttributionOnRelay(t *testing.T) {
	rec := &batchRecorder{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	f := newStreamForwarder(agents.StreamTarget{StreamId: "s1", CallbackUrl: srv.URL}, "p", "t", "middle_orchestrator", []string{"root"})
	f.Callback()(agents.StreamEvent{Event: agents.StreamEventThinking, Agent: "deep_agent", Chain: "root/middle_orchestrator/deep_agent"})
	f.Close()

	events := rec.all()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Agent != "deep_agent" || events[0].Chain != "root/middle_orchestrator/deep_agent" {
		t.Errorf("relaying must not relabel an already attributed event : %+v", events[0])
	}
}

func TestStreamForwarderSurvivesDeadCollector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	srv.Close()

	f := newStreamForwarder(agents.StreamTarget{StreamId: "s1", CallbackUrl: srv.URL}, "p", "t", "a", nil)
	cb := f.Callback()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			cb(agents.StreamEvent{Event: agents.StreamEventThinking})
		}
		f.Close()
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("forwarder blocked the producer when the collector was unreachable")
	}
}

func TestStreamForwarderCallbackNeverBlocks(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	defer close(block)

	f := newStreamForwarder(agents.StreamTarget{StreamId: "s1", CallbackUrl: srv.URL}, "p", "t", "a", nil)
	cb := f.Callback()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < forwarderQueue*2; i++ {
			cb(agents.StreamEvent{Event: agents.StreamEventThinking})
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("callback blocked while the collector was slow")
	}
}

func TestAgentStreamEventHandlerQueuesOntoOwningStream(t *testing.T) {
	sink := activeStreams.open("live-stream")
	defer activeStreams.close("live-stream")

	body := `{"events":[{"event":"thinking","agent":"sub","chain":"root/sub","seq":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/p/t/stream/live-stream/event", strings.NewReader(body))
	req = muxVars(req, map[string]string{"streamid": "live-stream"})
	rec := httptest.NewRecorder()
	AgentStreamEventHandler()(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	select {
	case event := <-sink.events:
		if event.Agent != "sub" || event.Chain != "root/sub" {
			t.Errorf("attribution lost in transit : %+v", event)
		}
	default:
		t.Fatal("event was not queued onto the stream")
	}
}

func TestAgentStreamEventHandlerAcceptsEventsForEndedStream(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/p/t/stream/gone/event", strings.NewReader(`{"events":[{"event":"thinking"}]}`))
	req = muxVars(req, map[string]string{"streamid": "gone"})
	rec := httptest.NewRecorder()
	AgentStreamEventHandler()(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("a sub-agent must not be failed by an ended stream, got %d", rec.Code)
	}
}

func TestStreamForwarderCloseDoesNotHoldUpTheResponse(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	defer close(block)

	f := newStreamForwarder(agents.StreamTarget{StreamId: "s1", CallbackUrl: srv.URL}, "p", "t", "a", nil)
	f.Callback()(agents.StreamEvent{Event: agents.StreamEventThinking})

	start := time.Now()
	f.Close()
	if elapsed := time.Since(start); elapsed > forwarderCloseWait*3 {
		t.Fatalf("Close blocked the sub-agent response for %s", elapsed)
	}
}
