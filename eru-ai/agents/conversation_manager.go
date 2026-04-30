package agents

import (
	"context"
	"encoding/json"
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

	historyMessages := cm.convertAgentMessagesToMessages(conversation.Messages, agentName)

	allMessages := append(historyMessages, currentMessage)

	tokenCount := cm.estimateTokenCount(allMessages)
	logs.WithContext(ctx).Info(fmt.Sprintf("Total estimated tokens: %d", tokenCount))

	if tokenCount <= cm.Config.MaxTokens {
		return &models.ChatRequest{Messages: allMessages}, nil
	}

	protectedCount := cm.Config.MaxRecentMessages
	if protectedCount <= 0 {
		protectedCount = 20
	}

	var protectedMessages []models.Message
	var compactableMessages []models.Message

	if len(historyMessages) > protectedCount {
		compactableMessages = historyMessages[:len(historyMessages)-protectedCount]
		protectedMessages = historyMessages[len(historyMessages)-protectedCount:]
	} else {
		protectedMessages = historyMessages
	}

	var managedMessages []models.Message

	if len(compactableMessages) > 0 {
		summary, err := cm.summarizeMessages(ctx, compactableMessages)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("Failed to summarize messages: %v", err))
		} else if summary != "" {
			managedMessages = append(managedMessages, models.Message{
				Role:    "user",
				Content: fmt.Sprintf("Previous conversation summary:\n%s", summary),
				Name:    "conversation_summary",
			})
		}
	}

	managedMessages = append(managedMessages, protectedMessages...)
	managedMessages = append(managedMessages, currentMessage)
	return &models.ChatRequest{Messages: managedMessages}, nil
}

// convertAgentMessagesToMessages converts AgentMessage slice to models.Message slice
func (cm *ConversationManager) convertAgentMessagesToMessages(agentMessages []AgentMessage, agentName string) []models.Message {
	messages := make([]models.Message, 0, len(agentMessages))

	for _, agentMsg := range agentMessages {
		content := agentMsg.Content
		if content == "" && len(agentMsg.Actions) > 0 {
			actionBytes, err := json.Marshal(agentMsg.Actions[0].Action)
			if err == nil {
				content = string(actionBytes)
			}
		}
		msg := models.Message{
			Role:    agentMsg.Role,
			Content: content,
			Name:    agentName,
			Files:   agentMsg.Files,
		}
		messages = append(messages, msg)
	}

	return messages
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
	summaryPrompt := fmt.Sprintf(`Summarize the following conversation concisely, preserving:
1. Key decisions and requirements
2. Names of any generated functions, queries, agents, or schemas
3. Important field names, step keys, and configuration values
4. User preferences and constraints

Do NOT include full JSON outputs — just note what was generated and key identifiers.

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
