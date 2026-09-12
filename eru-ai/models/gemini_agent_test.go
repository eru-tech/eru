package models

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	eru_models "github.com/eru-tech/eru/eru-models"
)

func TestSanitizeGeminiSchemaStripsAdditionalProperties(t *testing.T) {
	schema := eru_models.JSONSchema{
		Type:                 "object",
		AdditionalProperties: true,
		Properties: map[string]eru_models.JSONSchema{
			"rows": {
				Type: "array",
				Items: &eru_models.JSONSchema{
					Type:                 "object",
					AdditionalProperties: map[string]interface{}{"type": "string"},
					Properties: map[string]eru_models.JSONSchema{
						"name": {Type: "string"},
					},
				},
			},
		},
	}

	sanitized := sanitizeGeminiSchema(schema)

	encoded, err := json.Marshal(sanitized)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if strings.Contains(string(encoded), "additionalProperties") {
		t.Errorf("additionalProperties must be stripped at every level, got: %s", encoded)
	}
	if sanitized.Properties["rows"].Items.Properties["name"].Type != "string" {
		t.Error("sanitising must preserve the rest of the schema")
	}
}

func TestSanitizeGeminiSchemaDoesNotMutateInput(t *testing.T) {
	schema := eru_models.JSONSchema{
		Type:                 "object",
		AdditionalProperties: true,
		Properties: map[string]eru_models.JSONSchema{
			"a": {Type: "string", AdditionalProperties: true},
		},
	}

	sanitizeGeminiSchema(schema)

	if schema.AdditionalProperties == nil {
		t.Error("input schema must not be mutated")
	}
	if schema.Properties["a"].AdditionalProperties == nil {
		t.Error("nested input schema must not be mutated")
	}
}

func TestGeminiUsageMapping(t *testing.T) {
	var acc UsageAccumulator
	accumulateGeminiUsage(&acc, &GeminiUsageMetadata{
		PromptTokenCount:        7000,
		CandidatesTokenCount:    100,
		ThoughtsTokenCount:      250,
		CachedContentTokenCount: 6500,
		TotalTokenCount:         7350,
	})

	usage := acc.TokenUsage()
	if usage.InputTokens != 7000 {
		t.Errorf("InputTokens = %d, want 7000", usage.InputTokens)
	}
	if usage.OutputTokens != 350 {
		t.Errorf("OutputTokens = %d, want 350 (candidates + thoughts are both billed as output)", usage.OutputTokens)
	}
	if usage.CachedTokens != 6500 {
		t.Errorf("CachedTokens = %d, want 6500", usage.CachedTokens)
	}
	if usage.ReasoningTokens != 250 {
		t.Errorf("ReasoningTokens = %d, want 250", usage.ReasoningTokens)
	}
}

func TestGeminiUsageMappingHandlesNil(t *testing.T) {
	var acc UsageAccumulator
	accumulateGeminiUsage(&acc, nil)
	if acc.TokenUsage().TotalTokens != 0 {
		t.Error("nil usage metadata must be a no-op")
	}
}

func TestGeminiCandidateTextSeparatesThoughts(t *testing.T) {
	candidate := GeminiCandidate{
		Content: GeminiContent{
			Parts: []GeminiPart{
				{Text: "let me think", Thought: true},
				{Text: "The answer "},
				{Text: "is 42."},
			},
		},
	}

	content, thinking := geminiCandidateText(candidate)
	if content != "The answer is 42." {
		t.Errorf("content = %q", content)
	}
	if thinking != "let me think" {
		t.Errorf("thinking = %q", thinking)
	}
}

func TestGeminiFunctionCallsReturnsAllCalls(t *testing.T) {
	candidate := GeminiCandidate{
		Content: GeminiContent{
			Parts: []GeminiPart{
				{Text: "calling tools"},
				{FunctionCall: &GeminiFunctionCall{Id: "a", Name: "alpha", Args: map[string]interface{}{"x": 1.0}}},
				{FunctionCall: &GeminiFunctionCall{Id: "b", Name: "beta"}},
			},
		},
	}

	calls := geminiFunctionCalls(candidate)
	if len(calls) != 2 {
		t.Fatalf("parallel function calls must all be returned, got %d", len(calls))
	}
	if calls[0].Name != "alpha" || calls[1].Name != "beta" {
		t.Errorf("function names lost: %+v", calls)
	}
	if calls[0].Id != "a" || calls[1].Id != "b" {
		t.Errorf("function call ids must survive for response matching: %+v", calls)
	}
}

func TestMergeGeminiTextPartsCombinesStreamedFragments(t *testing.T) {
	merged := mergeGeminiTextParts([]GeminiPart{
		{Text: "think", Thought: true},
		{Text: "ing", Thought: true},
		{Text: "Hel"},
		{Text: "lo"},
		{FunctionCall: &GeminiFunctionCall{Name: "alpha"}},
		{Text: "after"},
	})

	if len(merged) != 4 {
		t.Fatalf("expected thought, text, functionCall, text; got %d parts: %+v", len(merged), merged)
	}
	if !merged[0].Thought || merged[0].Text != "thinking" {
		t.Errorf("thought fragments not merged: %+v", merged[0])
	}
	if merged[1].Text != "Hello" || merged[1].Thought {
		t.Errorf("text fragments not merged: %+v", merged[1])
	}
	if merged[2].FunctionCall == nil || merged[2].FunctionCall.Name != "alpha" {
		t.Errorf("function call not preserved in order: %+v", merged[2])
	}
	if merged[3].Text != "after" {
		t.Errorf("text after a function call must stay separate: %+v", merged[3])
	}
}

func TestGeminiSystemContentCollectsSystemMessages(t *testing.T) {
	chatRequest := ChatRequest{Messages: []Message{
		{Role: "system", Content: "be terse"},
		{Role: "user", Content: "hi"},
		{Role: "system", Content: "answer in json"},
	}}

	if got := geminiSystemContent(chatRequest); got != "be terse\nanswer in json" {
		t.Errorf("geminiSystemContent = %q", got)
	}
}

func TestGeminiRequestRoutesSystemToSystemInstruction(t *testing.T) {
	model := &GeminiModel{}
	chatRequest := ChatRequest{Messages: []Message{
		{Role: "system", Content: "be terse"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}}

	request := model.buildGeminiRequest(context.Background(), chatRequest, nil, geminiSystemContent(chatRequest), 0)

	if request.SystemInstruction == nil {
		t.Fatal("system message must become systemInstruction, not a user turn")
	}
	if request.SystemInstruction.Parts[0].Text != "be terse" {
		t.Errorf("systemInstruction = %+v", request.SystemInstruction)
	}
	if len(request.Contents) != 2 {
		t.Fatalf("system message must not be duplicated into contents, got %d contents", len(request.Contents))
	}
	if request.Contents[0].Role != "user" || request.Contents[1].Role != "model" {
		t.Errorf("roles not mapped to gemini vocabulary: %+v", request.Contents)
	}
}

func TestGeminiRequestHonoursMaxTokens(t *testing.T) {
	model := &GeminiModel{}
	model.MaxTokens = 32000

	request := model.buildGeminiRequest(context.Background(), ChatRequest{}, nil, "", 0)
	if request.GenerationConfig.MaxOutputTokens == nil || *request.GenerationConfig.MaxOutputTokens != 32000 {
		t.Errorf("configured MaxTokens must be honoured, got %+v", request.GenerationConfig.MaxOutputTokens)
	}

	defaultModel := &GeminiModel{}
	defaultRequest := defaultModel.buildGeminiRequest(context.Background(), ChatRequest{}, nil, "", 0)
	if *defaultRequest.GenerationConfig.MaxOutputTokens != geminiDefaultMaxOutputTokens {
		t.Errorf("default max output tokens = %d, want %d", *defaultRequest.GenerationConfig.MaxOutputTokens, geminiDefaultMaxOutputTokens)
	}
}

func TestGeminiRequestSetsThinkingConfigOnlyWhenBudgeted(t *testing.T) {
	model := &GeminiModel{}

	withThinking := model.buildGeminiRequest(context.Background(), ChatRequest{}, nil, "", 4096)
	if withThinking.GenerationConfig.ThinkingConfig == nil {
		t.Fatal("a positive thinking budget must produce a thinkingConfig")
	}
	if !withThinking.GenerationConfig.ThinkingConfig.IncludeThoughts {
		t.Error("includeThoughts must be set, otherwise no thought parts come back")
	}
	if *withThinking.GenerationConfig.ThinkingConfig.ThinkingBudget != 4096 {
		t.Errorf("thinkingBudget = %d", *withThinking.GenerationConfig.ThinkingConfig.ThinkingBudget)
	}

	withoutThinking := model.buildGeminiRequest(context.Background(), ChatRequest{}, nil, "", 0)
	if withoutThinking.GenerationConfig.ThinkingConfig != nil {
		t.Error("no thinking budget must leave thinkingConfig unset")
	}
}

func TestGeminiAuthUsesHeaderNotQueryParam(t *testing.T) {
	model := &GeminiModel{}
	model.LLMSecret = "secret-key"

	header := model.authHeader()
	if header.Get("x-goog-api-key") != "secret-key" {
		t.Errorf("api key must travel in the x-goog-api-key header, got %+v", header)
	}
}

func TestCheckGeminiResponseSurfacesBlockAndEmpty(t *testing.T) {
	ctx := context.Background()

	blocked := GeminiChatResponse{PromptFeedback: &GeminiPromptFeedback{BlockReason: "SAFETY"}}
	if err := checkGeminiResponse(ctx, blocked); err == nil {
		t.Error("a blocked prompt must surface as an error, not an empty string")
	}

	empty := GeminiChatResponse{}
	if err := checkGeminiResponse(ctx, empty); err == nil {
		t.Error("a response with no candidates must be an error")
	}

	safetyStop := GeminiChatResponse{Candidates: []GeminiCandidate{{FinishReason: "SAFETY"}}}
	if err := checkGeminiResponse(ctx, safetyStop); err == nil {
		t.Error("a safety finish reason must surface as an error")
	}

	fine := GeminiChatResponse{Candidates: []GeminiCandidate{{FinishReason: "STOP"}}}
	if err := checkGeminiResponse(ctx, fine); err != nil {
		t.Errorf("a normal response must not error: %v", err)
	}
}
