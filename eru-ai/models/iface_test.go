package models

import "testing"

func TestProviderInterfaceParity(t *testing.T) {
	var _ ModelI = (*AnthropicModel)(nil)
	var _ ModelI = (*OpenAIModel)(nil)
	var _ ModelI = (*GeminiModel)(nil)

	var _ StreamingModelI = (*AnthropicModel)(nil)
	var _ StreamingModelI = (*OpenAIModel)(nil)
	var _ StreamingModelI = (*GeminiModel)(nil)

	var _ ReasoningModelI = (*AnthropicModel)(nil)
	var _ ReasoningModelI = (*OpenAIModel)(nil)
	var _ ReasoningModelI = (*GeminiModel)(nil)
}
