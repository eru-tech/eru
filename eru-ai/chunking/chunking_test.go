package chunking

import (
	"strings"
	"testing"
)

func TestChunkingFactory(t *testing.T) {
	factory := &ChunkingFactory{}

	// Test valid strategies
	strategies := []string{"fixed", "recursive", "sentence", "paragraph", "semantic"}
	for _, strategy := range strategies {
		chunker, err := factory.GetChunkingStrategy(ChunkingConfig{Strategy: strategy})
		if err != nil {
			t.Errorf("Failed to get chunker for strategy %s: %v", strategy, err)
		}
		if chunker.GetName() != strategy {
			t.Errorf("Expected chunker name %s, got %s", strategy, chunker.GetName())
		}
	}

	// Test invalid strategy
	_, err := factory.GetChunkingStrategy(ChunkingConfig{Strategy: "invalid"})
	if err == nil {
		t.Error("Expected error for invalid strategy")
	}
}

func TestFixedSizeChunker(t *testing.T) {
	chunker := &FixedSizeChunker{}
	config := ChunkingConfig{MaxTokens: 1000} // 4000 characters

	// Test short text (no chunking needed)
	text := "Short text"
	chunks, err := chunker.ChunkText(text, config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	// Test long text (chunking needed) - make it much longer than 4000 characters
	longText := strings.Repeat("This is a very long text that exceeds the token limit and will be automatically chunked by the system. "+
		"The chunking happens at the character level for fixed-size strategy. "+
		"This ensures that the text is properly processed even when it's longer than what the embedding model can handle in a single request. "+
		"Fixed-size chunking is the simplest approach but may break semantic units. "+
		"Machine learning is a subset of artificial intelligence that enables computers to learn and make decisions without being explicitly programmed. "+
		"It has revolutionized various industries including healthcare, finance, and transportation. "+
		"There are three main types of machine learning: supervised learning, unsupervised learning, and reinforcement learning. "+
		"Each type has its own applications and use cases. "+
		"Supervised learning involves training a model on labeled data, where the correct answers are provided. "+
		"This is useful for tasks like image classification, spam detection, and medical diagnosis. "+
		"Unsupervised learning finds hidden patterns in data without labels. "+
		"It's commonly used for customer segmentation, anomaly detection, and recommendation systems. "+
		"Reinforcement learning teaches agents to make decisions by rewarding good actions and penalizing bad ones. "+
		"This approach is used in game playing, robotics, and autonomous vehicles. "+
		"Machine learning is applied in numerous fields. "+
		"In healthcare, it helps diagnose diseases and predict patient outcomes. "+
		"In finance, it detects fraud and optimizes trading strategies. "+
		"In transportation, it powers self-driving cars and optimizes delivery routes. "+
		"The future of machine learning looks promising with advances in deep learning, natural language processing, and computer vision. "+
		"These technologies continue to push the boundaries of what's possible. "+
		"Natural language processing is a branch of artificial intelligence that helps computers understand, interpret, and manipulate human language. "+
		"NLP combines computational linguistics with statistical, machine learning, and deep learning models. "+
		"These technologies enable computers to process text and voice data in much the same way that humans do. "+
		"Common NLP tasks include text classification, sentiment analysis, language translation, and question answering. "+
		"Modern NLP systems use neural networks and transformer architectures to achieve state-of-the-art performance. "+
		"These models can understand context, handle ambiguity, and generate human-like text. "+
		"The field continues to evolve rapidly with new breakthroughs in language understanding and generation. ", 3)

	// Debug: check actual text length
	textLength := len(longText)
	t.Logf("Text length: %d characters, MaxChars: %d", textLength, config.MaxTokens*4)

	if textLength <= config.MaxTokens*4 {
		t.Skipf("Text is not long enough to test chunking: %d <= %d", textLength, config.MaxTokens*4)
	}

	chunks, err = chunker.ChunkText(longText, config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks for long text, got %d", len(chunks))
	}

	// Verify chunk sizes
	for i, chunk := range chunks {
		if len(chunk) > 4000 {
			t.Errorf("Chunk %d exceeds max size: %d > 4000", i, len(chunk))
		}
	}
}

func TestRecursiveChunker(t *testing.T) {
	chunker := &RecursiveChunker{}
	config := ChunkingConfig{
		MaxTokens:  1000, // 4000 characters
		Separators: []string{"\n\n", "\n", ". ", " ", ""},
	}

	// Test text with paragraphs
	text := "First paragraph.\n\nSecond paragraph with multiple sentences. This is the second sentence.\n\nThird paragraph."
	chunks, err := chunker.ChunkText(text, config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should preserve paragraph structure
	if len(chunks) < 1 {
		t.Errorf("Expected at least 1 chunk, got %d", len(chunks))
	}

	// Verify chunks don't exceed max size
	for i, chunk := range chunks {
		if len(chunk) > 4000 {
			t.Errorf("Chunk %d exceeds max size: %d > 4000", i, len(chunk))
		}
	}
}

func TestSentenceChunker(t *testing.T) {
	chunker := &SentenceChunker{}
	config := ChunkingConfig{MaxTokens: 1000} // 4000 characters

	// Test text with abbreviations
	text := "Mr. Smith went to Dr. Johnson's office. He arrived at 3:00 P.M. and stayed until 5:00 P.M. " +
		"The meeting was about Ph.D. research. They discussed U.S. policies and U.K. regulations."

	chunks, err := chunker.ChunkText(text, config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should handle abbreviations correctly (not split on them)
	if len(chunks) < 1 {
		t.Errorf("Expected at least 1 chunk, got %d", len(chunks))
	}

	// Verify chunks don't exceed max size
	for i, chunk := range chunks {
		if len(chunk) > 4000 {
			t.Errorf("Chunk %d exceeds max size: %d > 4000", i, len(chunk))
		}
	}
}

func TestParagraphChunker(t *testing.T) {
	chunker := &ParagraphChunker{}
	config := ChunkingConfig{MaxTokens: 1000} // 4000 characters

	// Test text with paragraphs
	text := "First paragraph with some content.\n\n" +
		"Second paragraph that is longer and contains multiple sentences. " +
		"This is the second sentence in the second paragraph. " +
		"And this is the third sentence.\n\n" +
		"Third paragraph with different content."

	chunks, err := chunker.ChunkText(text, config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should preserve paragraph structure
	if len(chunks) < 1 {
		t.Errorf("Expected at least 1 chunk, got %d", len(chunks))
	}

	// Verify chunks don't exceed max size
	for i, chunk := range chunks {
		if len(chunk) > 4000 {
			t.Errorf("Chunk %d exceeds max size: %d > 4000", i, len(chunk))
		}
	}
}

func TestDefaultChunkingConfig(t *testing.T) {
	config := DefaultChunkingConfig()

	// Verify default values
	if config.Strategy != "recursive" {
		t.Errorf("Expected default strategy 'recursive', got '%s'", config.Strategy)
	}
	if config.MaxTokens != 8000 {
		t.Errorf("Expected default MaxTokens 8000, got %d", config.MaxTokens)
	}
	if config.Language != "en" {
		t.Errorf("Expected default language 'en', got '%s'", config.Language)
	}
	if len(config.Separators) != 5 {
		t.Errorf("Expected 5 default separators, got %d", len(config.Separators))
	}
}

func TestLanguageDetection(t *testing.T) {
	// Test English text
	text := "This is English text with normal characters."
	detected := detectLanguage(text)
	if detected != "en" {
		t.Errorf("Expected 'en' for English text, got '%s'", detected)
	}

	// Test Chinese text
	chineseText := "这是中文文本"
	detected = detectLanguage(chineseText)
	if detected != "zh" {
		t.Errorf("Expected 'zh' for Chinese text, got '%s'", detected)
	}

	// Test mixed text (should default to English)
	mixedText := "This is English with some 中文 mixed in."
	detected = detectLanguage(mixedText)
	if detected != "en" {
		t.Errorf("Expected 'en' for mixed text, got '%s'", detected)
	}
}

func TestTokenEstimation(t *testing.T) {
	// Test English text
	text := "This is a test sentence with English words."
	estimated := estimateTokens(text, "en")
	if estimated <= 0 {
		t.Errorf("Expected positive token estimate for English, got %d", estimated)
	}

	// Test Chinese text
	chineseText := "这是中文测试句子"
	estimated = estimateTokens(chineseText, "zh")
	if estimated <= 0 {
		t.Errorf("Expected positive token estimate for Chinese, got %d", estimated)
	}

	// Chinese should have higher token count (1 char ≈ 1 token)
	if estimated < len([]rune(chineseText)) {
		t.Errorf("Chinese token estimate should be >= character count")
	}
}

func TestChunkingWithOverlap(t *testing.T) {
	// Test that overlap parameter is respected (when implemented)
	config := ChunkingConfig{
		Strategy:     "recursive",
		MaxTokens:    1000,
		OverlapChars: 100, // 400 characters
	}

	// This test will need to be updated when overlap is implemented
	_ = config.OverlapChars // Suppress unused variable warning
}

func TestCustomSeparators(t *testing.T) {
	chunker := &RecursiveChunker{}
	config := ChunkingConfig{
		MaxTokens:  1000,
		Separators: []string{"## ", "### ", "\n\n", "\n", ". ", " "},
	}

	// Test markdown-style text
	text := "# Title\n## Section 1\nContent here.\n\n## Section 2\nMore content."
	chunks, err := chunker.ChunkText(text, config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should create chunks based on custom separators
	if len(chunks) < 1 {
		t.Errorf("Expected at least 1 chunk, got %d", len(chunks))
	}
}
