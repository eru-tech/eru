package models

import (
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestAgentPromptStringPreservesConcatenation(t *testing.T) {
	prompt := AgentPrompt{Static: "static part", Dynamic: "\nDYNAMIC"}
	if prompt.String() != "static part\nDYNAMIC" {
		t.Errorf("String() = %q", prompt.String())
	}

	staticOnly := StaticAgentPrompt("only static")
	if staticOnly.String() != "only static" {
		t.Errorf("StaticAgentPrompt String() = %q", staticOnly.String())
	}
	if staticOnly.Dynamic != "" {
		t.Error("StaticAgentPrompt must leave Dynamic empty")
	}
	if (AgentPrompt{}).IsEmpty() != true {
		t.Error("zero AgentPrompt must report empty")
	}
}

func TestCachedSystemBlocksPutsBreakpointOnStaticOnly(t *testing.T) {
	prompt := AgentPrompt{Static: "STATIC PROMPT", Dynamic: "\nEXECUTION CONTEXT: tenant-a"}

	blocks := cachedSystemBlocks(prompt, "")

	if len(blocks) != 2 {
		t.Fatalf("expected a static block and a dynamic block, got %d", len(blocks))
	}
	if blocks[0].Text != "STATIC PROMPT" {
		t.Errorf("first block should be the static prompt, got %q", blocks[0].Text)
	}
	if !hasCacheBreakpoint(t, blocks[0]) {
		t.Error("the static block must carry the cache breakpoint")
	}
	if blocks[1].Text != "\nEXECUTION CONTEXT: tenant-a" {
		t.Errorf("second block should be the per-tenant context, got %q", blocks[1].Text)
	}
	if hasCacheBreakpoint(t, blocks[1]) {
		t.Error("the per-tenant block must NOT carry a breakpoint, otherwise every tenant pays its own cache write")
	}
}

func TestCachedSystemBlocksSharesStaticPrefixAcrossTenants(t *testing.T) {
	tenantA := cachedSystemBlocks(AgentPrompt{Static: "STATIC", Dynamic: "tenant-a"}, "tool prompt")
	tenantB := cachedSystemBlocks(AgentPrompt{Static: "STATIC", Dynamic: "tenant-b"}, "tool prompt")

	if tenantA[0].Text != tenantB[0].Text {
		t.Errorf("the cached prefix must be byte identical across tenants:\n a=%q\n b=%q", tenantA[0].Text, tenantB[0].Text)
	}
	if tenantA[1].Text == tenantB[1].Text {
		t.Error("the uncached block should differ per tenant")
	}
}

func TestCachedSystemBlocksFoldsToolPromptIntoCachedPrefix(t *testing.T) {
	blocks := cachedSystemBlocks(AgentPrompt{Static: "STATIC"}, "TOOL GUIDANCE")

	if len(blocks) != 1 {
		t.Fatalf("with no dynamic content there should be a single block, got %d", len(blocks))
	}
	if blocks[0].Text != "STATIC\nTOOL GUIDANCE" {
		t.Errorf("tool prompt must be inside the cached prefix, got %q", blocks[0].Text)
	}
	if !hasCacheBreakpoint(t, blocks[0]) {
		t.Error("the single block must carry the cache breakpoint")
	}
}

func TestAnthropicUsageNormalisationMatchesDocumentedFormula(t *testing.T) {
	var acc UsageAccumulator
	accumulateUsage(&acc, anthropicUsageFixture(50, 6900, 0, 120))

	usage := acc.TokenUsage()
	if usage.InputTokens != 6950 {
		t.Errorf("InputTokens = %d, want 6950 (input + cache_read + cache_creation)", usage.InputTokens)
	}
	if usage.CachedTokens != 6900 {
		t.Errorf("CachedTokens = %d, want 6900", usage.CachedTokens)
	}
	if usage.OutputTokens != 120 {
		t.Errorf("OutputTokens = %d, want 120", usage.OutputTokens)
	}
}

func TestAnthropicUsageAccumulatesWriteThenRead(t *testing.T) {
	var acc UsageAccumulator
	accumulateUsage(&acc, anthropicUsageFixture(50, 0, 6900, 100))
	accumulateUsage(&acc, anthropicUsageFixture(80, 6900, 0, 150))

	usage := acc.TokenUsage()
	if usage.InputTokens != 13930 {
		t.Errorf("InputTokens = %d, want 13930", usage.InputTokens)
	}
	if usage.CachedTokens != 6900 {
		t.Errorf("CachedTokens = %d, want 6900 (only the read counts as cached)", usage.CachedTokens)
	}
}

func anthropicUsageFixture(inputTokens int64, cacheRead int64, cacheWrite int64, outputTokens int64) anthropic.Usage {
	return anthropic.Usage{
		InputTokens:              inputTokens,
		CacheReadInputTokens:     cacheRead,
		CacheCreationInputTokens: cacheWrite,
		OutputTokens:             outputTokens,
	}
}

func hasCacheBreakpoint(t *testing.T, block anthropic.TextBlockParam) bool {
	t.Helper()
	encoded, err := block.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return strings.Contains(string(encoded), "cache_control")
}
