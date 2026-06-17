package agents

import "context"

type contextKey string

const StreamCallbackKey contextKey = "stream_callback"

const (
	StreamEventThinking   = "thinking"
	StreamEventToolUse    = "tool_use"
	StreamEventToolResult = "tool_result"
	StreamEventTextDelta  = "text_delta"
	StreamEventDone       = "done"
	StreamEventError      = "error"
	StreamEventQuestion   = "question"
	StreamEventStatus     = "status"
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
