package models

import "testing"

func TestUsageAccumulatorSumsAcrossIterations(t *testing.T) {
	var acc UsageAccumulator
	acc.Add(1000, 50, 0, 0)
	acc.Add(1200, 80, 900, 0)

	usage := acc.TokenUsage()
	if usage.InputTokens != 2200 {
		t.Errorf("InputTokens = %d, want 2200", usage.InputTokens)
	}
	if usage.OutputTokens != 130 {
		t.Errorf("OutputTokens = %d, want 130", usage.OutputTokens)
	}
	if usage.CachedTokens != 900 {
		t.Errorf("CachedTokens = %d, want 900", usage.CachedTokens)
	}
	if usage.TotalTokens != 2330 {
		t.Errorf("TotalTokens = %d, want 2330", usage.TotalTokens)
	}
}

func TestUsageAccumulatorCachedIsSubsetOfInput(t *testing.T) {
	var acc UsageAccumulator
	acc.Add(7000, 100, 6900, 0)

	usage := acc.TokenUsage()
	if usage.CachedTokens > usage.InputTokens {
		t.Errorf("CachedTokens %d must not exceed InputTokens %d", usage.CachedTokens, usage.InputTokens)
	}
}

func TestUsageAccumulatorAttachSetsUsage(t *testing.T) {
	var acc UsageAccumulator
	acc.Add(10, 5, 0, 3)

	msg := acc.Attach(Message{Content: "hi", Role: "assistant"})
	if msg.Content != "hi" || msg.Role != "assistant" {
		t.Errorf("Attach altered the message: %+v", msg)
	}
	if msg.Usage == nil {
		t.Fatal("Attach did not set Usage")
	}
	if msg.Usage.ReasoningTokens != 3 {
		t.Errorf("ReasoningTokens = %d, want 3", msg.Usage.ReasoningTokens)
	}
	if msg.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", msg.Usage.TotalTokens)
	}
}

func TestUsageAccumulatorZeroValue(t *testing.T) {
	var acc UsageAccumulator
	usage := acc.TokenUsage()
	if usage == nil {
		t.Fatal("TokenUsage returned nil")
	}
	if usage.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0", usage.TotalTokens)
	}
}
