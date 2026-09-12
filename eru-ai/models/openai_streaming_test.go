package models

import (
	"encoding/json"
	"testing"
)

func consumeChunks(t *testing.T, rawChunks []string, onTextDelta func(string), onToolInputDelta func(string, int, string)) openAIStreamResult {
	t.Helper()

	accumulator := newOpenAIStreamAccumulator()
	for _, raw := range rawChunks {
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			t.Fatalf("fixture chunk is not valid JSON: %v\n%s", err, raw)
		}
		accumulator.consume(chunk, onTextDelta, onToolInputDelta)
	}
	return accumulator.result()
}

func TestOpenAIStreamAccumulatesText(t *testing.T) {
	var deltas []string
	result := consumeChunks(t, []string{
		`{"choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`,
		`{"choices":[{"index":0,"delta":{"content":", "}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"world"},"finish_reason":"stop"}]}`,
	}, func(chunk string) { deltas = append(deltas, chunk) }, nil)

	if result.Content != "Hello, world" {
		t.Errorf("Content = %q, want %q", result.Content, "Hello, world")
	}
	if result.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", result.FinishReason)
	}
	if len(deltas) != 3 {
		t.Errorf("expected 3 text deltas, got %d: %v", len(deltas), deltas)
	}
}

func TestOpenAIStreamAssemblesToolCallAcrossChunks(t *testing.T) {
	result := consumeChunks(t, []string{
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_user","arguments":""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"id\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"42}"}}]}}],"finish_reason":"tool_calls"}`,
	}, nil, nil)

	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	call := result.ToolCalls[0]
	if call.Id != "call_1" {
		t.Errorf("Id = %q, want call_1", call.Id)
	}
	if call.Function.Name != "get_user" {
		t.Errorf("Name = %q, want get_user", call.Function.Name)
	}
	if call.Function.Arguments != `{"id":42}` {
		t.Errorf("Arguments = %q, want %q", call.Function.Arguments, `{"id":42}`)
	}

	inputMap := map[string]interface{}{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &inputMap); err != nil {
		t.Fatalf("assembled arguments must be valid JSON: %v", err)
	}
}

func TestOpenAIStreamKeepsParallelToolCallsSeparate(t *testing.T) {
	result := consumeChunks(t, []string{
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"alpha","arguments":"{\"x\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_b","function":{"name":"beta","arguments":"{\"y\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"2}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]}}]}`,
	}, nil, nil)

	if len(result.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "alpha" || result.ToolCalls[0].Function.Arguments != `{"x":1}` {
		t.Errorf("first call wrong: %+v", result.ToolCalls[0])
	}
	if result.ToolCalls[1].Function.Name != "beta" || result.ToolCalls[1].Function.Arguments != `{"y":2}` {
		t.Errorf("second call wrong: %+v", result.ToolCalls[1])
	}
}

func TestOpenAIStreamToolInputDeltaCarriesToolName(t *testing.T) {
	type delta struct {
		name    string
		index   int
		partial string
	}
	var deltas []delta

	consumeChunks(t, []string{
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"structured_output","arguments":"{\"a\""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":1}"}}]}}]}`,
	}, nil, func(toolName string, blockIndex int, partial string) {
		deltas = append(deltas, delta{toolName, blockIndex, partial})
	})

	if len(deltas) != 2 {
		t.Fatalf("expected 2 tool input deltas, got %d", len(deltas))
	}
	for _, d := range deltas {
		if d.name != "structured_output" {
			t.Errorf("tool name should be carried on every delta, got %q", d.name)
		}
		if d.index != 0 {
			t.Errorf("block index = %d, want 0", d.index)
		}
	}
}

func TestOpenAIStreamReadsUsageFromFinalChunk(t *testing.T) {
	result := consumeChunks(t, []string{
		`{"choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":"stop"}]}`,
		`{"choices":[],"usage":{"prompt_tokens":7000,"completion_tokens":120,"total_tokens":7120,"prompt_tokens_details":{"cached_tokens":6912},"completion_tokens_details":{"reasoning_tokens":64}}}`,
	}, nil, nil)

	var acc UsageAccumulator
	accumulateOpenAIUsage(&acc, result.Usage)
	usage := acc.TokenUsage()

	if usage.InputTokens != 7000 {
		t.Errorf("InputTokens = %d, want 7000", usage.InputTokens)
	}
	if usage.CachedTokens != 6912 {
		t.Errorf("CachedTokens = %d, want 6912", usage.CachedTokens)
	}
	if usage.ReasoningTokens != 64 {
		t.Errorf("ReasoningTokens = %d, want 64", usage.ReasoningTokens)
	}
	if usage.CachedTokens > usage.InputTokens {
		t.Errorf("cached %d must be a subset of input %d", usage.CachedTokens, usage.InputTokens)
	}
}

func TestOpenAIUsageTokensAcceptsResponsesApiNaming(t *testing.T) {
	usage := OpenAIChatUsage{
		InputTokens:  500,
		OutputTokens: 60,
	}
	usage.InputTokensDetails.CachedTokens = 128
	usage.OutputTokensDetails.ReasoningTokens = 32

	inputTokens, outputTokens, cachedTokens, reasoningTokens := openAIUsageTokens(usage)
	if inputTokens != 500 || outputTokens != 60 || cachedTokens != 128 || reasoningTokens != 32 {
		t.Errorf("Responses-API field names not honoured: got %d %d %d %d", inputTokens, outputTokens, cachedTokens, reasoningTokens)
	}
}

func TestReasoningEffortForBudget(t *testing.T) {
	cases := []struct {
		budget     int
		configured string
		want       string
	}{
		{0, "", "medium"},
		{1024, "", "low"},
		{4096, "", "medium"},
		{32000, "", "high"},
		{32000, "low", "low"},
	}
	for _, c := range cases {
		if got := reasoningEffortForBudget(c.budget, c.configured); got != c.want {
			t.Errorf("reasoningEffortForBudget(%d, %q) = %q, want %q", c.budget, c.configured, got, c.want)
		}
	}
}
