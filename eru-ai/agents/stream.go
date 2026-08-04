package agents

import "context"

type contextKey string

const StreamCallbackKey contextKey = "stream_callback"
const RawOutputKey contextKey = "raw_output"

const (
	StreamEventThinking   = "thinking"
	StreamEventToolUse    = "tool_use"
	StreamEventToolResult = "tool_result"
	StreamEventTextDelta  = "text_delta"
	StreamEventDone       = "done"
	StreamEventError      = "error"
	StreamEventQuestion   = "question"
	StreamEventStatus     = "status"
	StreamEventPlan       = "plan"
)

type StreamEvent struct {
	Event     string      `json:"event"`
	Data      interface{} `json:"data,omitempty"`
	Iteration int         `json:"iteration,omitempty"`
}

type StreamCallback func(event StreamEvent)

func WithStreamCallback(ctx context.Context, cb StreamCallback) context.Context {
	return context.WithValue(ctx, StreamCallbackKey, cb)
}

func GetStreamCallback(ctx context.Context) StreamCallback {
	if cb, ok := ctx.Value(StreamCallbackKey).(StreamCallback); ok {
		return cb
	}
	return nil
}

// WithRawOutput marks the request as wanting internal artifacts (orchestration
// plans, structured_output tool inputs) in the response. Set from the ?raw=true
// query param and meant for development only.
func WithRawOutput(ctx context.Context, raw bool) context.Context {
	return context.WithValue(ctx, RawOutputKey, raw)
}

func RawOutputEnabled(ctx context.Context) bool {
	if raw, ok := ctx.Value(RawOutputKey).(bool); ok {
		return raw
	}
	return false
}
