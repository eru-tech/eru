package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	tools "github.com/eru-tech/eru/eru-ai/tools"
)

type fakeTool struct {
	tools.Tool
	name string
	err  error
}

func (f *fakeTool) Execute(ctx context.Context, projectId, tenantId, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	return map[string]interface{}{"ok": true}, false, f.err
}

func (f *fakeTool) GetAttribute(ctx context.Context, name string) (interface{}, error) {
	if name == "tool_name" {
		return f.name, nil
	}
	return nil, errors.New("not found")
}

// The record is written by the tool, not the loop, so a call made outside any
// traced loop is still counted - which is the whole point.
func TestRecordingCapturesCallsAndArguments(t *testing.T) {
	record := NewToolRecord()
	ctx := WithToolRecord(context.Background(), record)
	tool := Recording(&fakeTool{name: "run_query"})

	_, _, _ = tool.Execute(ctx, "processo", "t1", "run_query", map[string]interface{}{"query_name": "db_disb"})
	_, _, _ = tool.Execute(ctx, "processo", "t1", "run_query", map[string]interface{}{"query_name": "db_tiles_os"})

	calls := record.Calls()
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].Tool != "run_query" || calls[0].Args["query_name"] != "db_disb" {
		t.Errorf("first call = %+v", calls[0])
	}
	// Arguments are the point: counts cannot tell which query was probed.
	if calls[1].Args["query_name"] != "db_tiles_os" {
		t.Errorf("second call lost its argument: %+v", calls[1])
	}
	if calls[0].Seq != 1 || calls[1].Seq != 2 {
		t.Errorf("order not preserved: %d, %d", calls[0].Seq, calls[1].Seq)
	}
	if tally := record.Tally(); tally["run_query"] != 2 {
		t.Errorf("tally = %v", tally)
	}
}

func TestRecordingKeepsFailuresWithTheirReason(t *testing.T) {
	record := NewToolRecord()
	ctx := WithToolRecord(context.Background(), record)
	tool := Recording(&fakeTool{name: "save_field", err: errors.New("storage_name is mandatory")})

	_, _, err := tool.Execute(ctx, "processo", "t1", "save_field", map[string]interface{}{"field": "x"})
	if err == nil {
		t.Fatal("the error should still reach the caller")
	}
	calls := record.Calls()
	if len(calls) != 1 || calls[0].Ok {
		t.Fatalf("a failed call should be recorded as failed: %+v", calls)
	}
	if !strings.Contains(calls[0].Error, "storage_name") {
		t.Errorf("the reason should be kept: %q", calls[0].Error)
	}
}

// Arguments travel back in the reply, so anything that looks like a credential
// must not go with them.
func TestRecordingRedactsSecrets(t *testing.T) {
	record := NewToolRecord()
	ctx := WithToolRecord(context.Background(), record)
	tool := Recording(&fakeTool{name: "call_api"})

	_, _, _ = tool.Execute(ctx, "p", "t", "call_api", map[string]interface{}{
		"url": "https://example.test", "api_key": "sk-live-123", "password": "hunter2", "claims": "{...}",
	})
	args := record.Calls()[0].Args
	for _, key := range []string{"api_key", "password", "claims"} {
		if args[key] != "[redacted]" {
			t.Errorf("%s = %v, want [redacted]", key, args[key])
		}
	}
	if args["url"] != "https://example.test" {
		t.Errorf("an ordinary argument must survive: %v", args["url"])
	}
}

// A saved page's definition is tens of kilobytes; carrying it back on every
// reply would cost far more than it is worth.
func TestRecordingSummarisesLargeArguments(t *testing.T) {
	record := NewToolRecord()
	ctx := WithToolRecord(context.Background(), record)
	tool := Recording(&fakeTool{name: "save_page"})

	big := map[string]interface{}{"components": strings.Repeat("x", 5000)}
	_, _, _ = tool.Execute(ctx, "p", "t", "save_page", map[string]interface{}{"page_id": "p1", "page_def": big})

	args := record.Calls()[0].Args
	if args["page_id"] != "p1" {
		t.Errorf("small arguments stay whole: %v", args["page_id"])
	}
	rendered, _ := json.Marshal(args["page_def"])
	if len(rendered) > 200 {
		t.Errorf("large argument not summarised: %d bytes", len(rendered))
	}
}

// Outside a run there is no record, and a tool must not care.
func TestRecordingIsHarmlessWithoutARecord(t *testing.T) {
	tool := Recording(&fakeTool{name: "x"})
	if _, _, err := tool.Execute(context.Background(), "p", "t", "x", nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ToolRecordFrom(context.Background()).Tally() != nil && len(ToolRecordFrom(context.Background()).Tally()) != 0 {
		t.Error("a nil record should tally to nothing")
	}
}

// Wrapping twice must not count twice: a tool reachable through both toolsMap
// and AgentTools is wrapped at both, and a doubled record would make every
// count assertion read twice the truth.
func TestRecordingIsIdempotent(t *testing.T) {
	inner := &fakeTool{}
	once := Recording(inner)
	twice := Recording(once)
	if once != twice {
		t.Fatal("re-wrapping a recorded tool must return the same wrapper")
	}

	ctx := WithToolRecord(context.Background(), NewToolRecord())
	if _, _, err := twice.Execute(ctx, "p", "t", "save_field", map[string]interface{}{"a": 1}); err != nil {
		t.Fatal(err)
	}
	calls := ToolRecordFrom(ctx).Calls()
	if len(calls) != 1 {
		t.Fatalf("one execution must produce one record, got %d", len(calls))
	}
}
