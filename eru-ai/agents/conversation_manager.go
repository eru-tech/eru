package agents

import (
	"context"
	"fmt"
	"strings"

	models "github.com/eru-tech/eru/eru-ai/models"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

// ConversationManager handles conversation history management with summarization
type ConversationManager struct {
	Config       *ConversationConfig // Configuration for conversation management
	SummaryModel models.ModelI       // Model for summarization
}

// ConversationSummary represents a summarized portion of conversation
type ConversationSummary struct {
	Summary      string `json:"summary"`
	MessageCount int    `json:"message_count"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
}

// BuildChatRequest builds a chat request with conversation history management
func (cm *ConversationManager) BuildChatRequest(ctx context.Context, conversation *Conversation, currentMessage models.Message, agentName string) (*models.ChatRequest, error) {
	logs.WithContext(ctx).Debug("BuildChatRequest - Start")

	// Convert AgentMessages to models.Messages
	historyMessages := cm.convertAgentMessagesToMessages(conversation.Messages, agentName)

	// Add current message
	allMessages := append(historyMessages, currentMessage)

	// Calculate token count
	tokenCount := cm.estimateTokenCount(allMessages)
	logs.WithContext(ctx).Info(fmt.Sprintf("Total estimated tokens: %d", tokenCount))

	// If within limits, return all messages
	if tokenCount <= cm.Config.MaxTokens {
		return &models.ChatRequest{Messages: allMessages}, nil
	}

	// Need to manage conversation history
	managedMessages, err := cm.manageConversationHistory(ctx, historyMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to manage conversation history: %v", err)
	}
	managedMessages = append(managedMessages, currentMessage)
	return &models.ChatRequest{Messages: managedMessages}, nil
}

// convertAgentMessagesToMessages converts AgentMessage slice to models.Message slice
func (cm *ConversationManager) convertAgentMessagesToMessages(agentMessages []AgentMessage, agentName string) []models.Message {
	messages := make([]models.Message, 0, len(agentMessages))

	for _, agentMsg := range agentMessages {
		msg := models.Message{
			Role:    agentMsg.Role,
			Content: agentMsg.Content,
			Name:    agentName,
			Files:   agentMsg.Files,
		}
		messages = append(messages, msg)
	}

	return messages
}

// manageConversationHistory manages conversation history with summarization
func (cm *ConversationManager) manageConversationHistory(ctx context.Context, messages []models.Message) ([]models.Message, error) {
	logs.WithContext(ctx).Debug("manageConversationHistory - Start")

	// Keep recent messages in full detail
	recentMessages := messages
	if len(messages) > cm.Config.MaxRecentMessages {
		recentMessages = messages[len(messages)-cm.Config.MaxRecentMessages:]
		olderMessages := messages[:len(messages)-cm.Config.MaxRecentMessages]

		// Summarize older messages
		summary, err := cm.summarizeMessages(ctx, olderMessages)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("Failed to summarize messages: %v", err))
			// If summarization fails, just truncate older messages
			return recentMessages, nil
		}

		// Create summary message
		summaryMessage := models.Message{
			Role:    "system",
			Content: fmt.Sprintf("Previous conversation summary: %s", summary),
			Name:    "conversation_summary",
		}

		// Combine summary with recent messages
		managedMessages := append([]models.Message{summaryMessage}, recentMessages...)

		// Check if still within limits
		if cm.estimateTokenCount(managedMessages) <= cm.Config.MaxTokens {
			return managedMessages, nil
		}

		// If still too long, further reduce recent messages
		return cm.manageConversationHistory(ctx, managedMessages)
	}

	return recentMessages, nil
}

// summarizeMessages creates a summary of older conversation messages
func (cm *ConversationManager) summarizeMessages(ctx context.Context, messages []models.Message) (string, error) {
	logs.WithContext(ctx).Debug("summarizeMessages - Start")

	if len(messages) == 0 {
		return "", nil
	}

	// Create a conversation text for summarization
	conversationText := cm.buildConversationText(messages)

	// Use the model to create a summary
	summaryPrompt := fmt.Sprintf(`
Please provide a concise summary of the following conversation, focusing on:
1. Key topics discussed
2. Important decisions made
3. User preferences or requirements mentioned
4. Any specific context that would be useful for future responses

Conversation:
%s

Summary:`, conversationText)

	summaryRequest := models.ChatRequest{
		Messages: []models.Message{
			{
				Role:    "user",
				Content: summaryPrompt,
				Name:    "summarizer",
			},
		},
	}

	response, err := cm.SummaryModel.QueryModel(ctx, summaryRequest)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %v", err)
	}

	return response.Content, nil
}

// buildConversationText builds a readable conversation text from messages
func (cm *ConversationManager) buildConversationText(messages []models.Message) string {
	var parts []string

	for _, msg := range messages {
		role := msg.Role
		if msg.Name != "" {
			role = fmt.Sprintf("%s (%s)", msg.Role, msg.Name)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", role, msg.Content))
	}

	return strings.Join(parts, "\n")
}

// reduceRecentMessages reduces the number of recent messages to fit within token limits
/* func (cm *ConversationManager) reduceRecentMessages(messages []models.Message) []models.Message {
	// Start with summary message
	if len(messages) == 0 {
		return messages
	}

	summaryMessage := messages[0]
	recentMessages := messages[1:]

	// Reduce recent messages until we fit within limits
	for len(recentMessages) > 1 {
		testMessages := append([]models.Message{summaryMessage}, recentMessages...)
		if cm.estimateTokenCount(testMessages) <= cm.Config.MaxTokens {
			break
		}
		recentMessages = recentMessages[1:] // Remove oldest recent message
	}

	return append([]models.Message{summaryMessage}, recentMessages...)
} */

// estimateTokenCount provides a rough estimate of token count
// This is a simplified estimation - in production, you might want to use a proper tokenizer
func (cm *ConversationManager) estimateTokenCount(messages []models.Message) int {
	totalTokens := 0

	for _, msg := range messages {
		// Rough estimation: 1 token ≈ 4 characters for English text
		contentTokens := len(msg.Content) / 4
		roleTokens := len(msg.Role) / 4
		nameTokens := len(msg.Name) / 4

		// Add overhead for JSON structure and formatting
		overhead := 10

		totalTokens += contentTokens + roleTokens + nameTokens + overhead
	}

	return totalTokens
}
