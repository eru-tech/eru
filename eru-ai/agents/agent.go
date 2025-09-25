package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	models "github.com/eru-tech/eru/eru-ai/models"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	"github.com/eru-tech/eru/eru-cache/cache"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

type AgentMessage struct {
	Content        string                 `json:"content,omitempty"`
	Params         map[string]interface{} `json:"params,omitempty"`
	Files          []models.FileMessage   `json:"files,omitempty"`
	ConversationId string                 `json:"conversation_id,omitempty"`
	MessageId      string                 `json:"message_id,omitempty"`
}

type ConversationMessage struct {
	MessageId   string                 `json:"message_id"`
	Timestamp   time.Time              `json:"timestamp"`
	Role        string                 `json:"role"`
	Content     string                 `json:"content"`
	Params      map[string]interface{} `json:"params,omitempty"`
	Files       []models.FileMessage   `json:"files,omitempty"`
	ToolResults map[string]interface{} `json:"tool_results,omitempty"`
}

type Conversation struct {
	ConversationId string                `json:"conversation_id"`
	Messages       []ConversationMessage `json:"messages"`
	NewMessages    []ConversationMessage `json:"-"`
}

type AgentTools struct {
	ToolName       string        `json:"tool_name"`
	ActionName     string        `json:"action_name"`
	ActionPrompt   string        `json:"action_prompt"`
	DependentTools []AgentTools  `json:"dependent_tools"`
	ToolKey        string        `json:"tool_key"`
	ToolOutputType string        `json:"tool_output_type"`
	Tool           tools.Tooling `json:"-"`
}
type Agent struct {
	AgentType    string                `json:"agent_type" eru:"required"`
	AgentName    string                `json:"agent_name" eru:"required"`
	Description  string                `json:"description"`
	SystemPrompt string                `json:"system_prompt"`
	AgentTools   []AgentTools          `json:"agent_tools"`
	ModelName    string                `json:"model"`
	Model        models.ModelI         `json:"-"`
	OutputSchema eru_models.JSONSchema `json:"output_schema"`
	//Tools        map[string]tools.Tooling `json:"-"`
	RetryCount int               `json:"retry_count"`
	ChatMemory cache.CacheStoreI `json:"chat_memory"`
}

type AgentI interface {
	GetSpec() AgentI
	Execute(ctx context.Context, agentMessage AgentMessage, projectId string, tenantId string) (map[string]interface{}, error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	//SetTools(tools map[string]tools.Tooling)
	ExecuteTools(ctx context.Context, chatRequest models.ChatRequest, agentTools []AgentTools, projectId string, tenantId string) (toolResults map[string]interface{}, err error)
	SetModel(model models.ModelI)
	SetChatMemory(ctx context.Context, cacheStoreI cache.CacheStoreI) error
	GetChatMemory() cache.CacheStoreI
	ValidateChatMemory(ctx context.Context, projectId string) error
	LoadConversationHistory(ctx context.Context, conversationId, projectId, tenantId string) (*Conversation, error)
	SaveConversation(ctx context.Context, conversation *Conversation, projectId string) error
	AddMessageToConversation(ctx context.Context, conversation *Conversation, role string, agentMessage AgentMessage, response map[string]interface{}) error
}

func (agent *Agent) GetSpec() AgentI {
	return agent
}

func (agent *Agent) Execute(ctx context.Context, agentMessage AgentMessage, projectId string, tenantId string) (map[string]interface{}, error) {
	err := logs.Err(ctx, fmt.Errorf("execute agent is not implemented"), "execute agent is not implemented")
	return nil, err
}

/* func (agent *Agent) processWithModel(ctx context.Context, conversation *Conversation, agentMessage AgentMessage, projectId string, tenantId string) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("processWithModel - Start")

	if agent.Model == nil {
		return nil, fmt.Errorf("model not configured for agent %s", agent.AgentName)
	}

	chatRequest := agent.buildChatRequest(conversation, agentMessage)

	if len(agent.AgentTools) > 0 {
		toolResults, err := agent.ExecuteTools(ctx, chatRequest, agent.AgentTools, projectId, tenantId)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("Tool execution failed: %v", err))
			return nil, err
		}
		return toolResults, nil
	}

	response, err := agent.Model.QueryModel(ctx, chatRequest, agent.AgentName)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Model query failed: %v", err))
		return nil, err
	}

	result := map[string]interface{}{
		"content": response.Content,
		"model":   response.ModelName,
	}

	if response.TokenUsage != nil {
		result["token_usage"] = response.TokenUsage
	}

	return result, nil
} */

/* func (agent *Agent) buildChatRequest(conversation *Conversation, currentMessage AgentMessage) models.ChatRequest {
	chatRequest := models.ChatRequest{
		SystemPrompt: agent.SystemPrompt,
		Messages:     []models.Message{},
		Model:        agent.ModelName,
		OutputSchema: agent.OutputSchema,
	}

	for _, msg := range conversation.Messages {
		chatMessage := models.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
		chatRequest.Messages = append(chatRequest.Messages, chatMessage)
	}

	if currentMessage.Content != "" {
		chatRequest.Messages = append(chatRequest.Messages, models.Message{
			Role:    "user",
			Content: currentMessage.Content,
		})
	}

	return chatRequest
} */

func (agent *Agent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &agent)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (agent *Agent) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "agent_type":
		return agent.AgentType, nil
	case "agent_name":
		return agent.AgentName, nil
	case "system_prompt":
		return agent.SystemPrompt, nil
	case "description":
		return agent.Description, nil
	case "agent_tools":
		return agent.AgentTools, nil
	case "model":
		return agent.ModelName, nil
	case "output_schema":
		return agent.OutputSchema, nil
	case "chat_memory_type":
		return agent.ChatMemory.GetAttribute(ctx, "cache_store_type")
	case "cache_db_alias":
		return agent.ChatMemory.GetAttribute(ctx, "cache_db_alias")
	case "persist_enabled":
		return agent.ChatMemory.GetAttribute(ctx, "persist_enabled")
	case "persist_error":
		return agent.ChatMemory.GetAttribute(ctx, "persist_error")
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

/* func (agent *Agent) SetTools(tools map[string]tools.Tooling) {
	agent.Tools = tools
} */

func (agent *Agent) SetModel(model models.ModelI) {
	agent.Model = model
}
func (agent *Agent) ExecuteTools(ctx context.Context, chatRequest models.ChatRequest, agentTools []AgentTools, projectId string, tenantId string) (toolResults map[string]interface{}, err error) {
	toolResults = make(map[string]interface{})
	for i, agentTool := range agentTools {
		tools := make(map[string]tools.Tooling)
		toolKey := agentTool.ToolKey
		if toolKey == "" {
			toolKey = fmt.Sprintf("%s_%s_%d", agent.AgentName, agentTool.ToolName, i)
		}
		tools[toolKey] = agentTool.Tool
		toolOutputType := agentTool.ToolOutputType
		if toolOutputType == "" {
			toolOutputType = "string"
		}
		response, err := agent.Model.QueryModelWithTool(ctx, chatRequest, tools, agent.AgentName, agentTool.ActionPrompt)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		if response.Content != nil {
			toolParams := make(map[string]interface{})
			toolParams["params"] = response.Content
			toolResult, _, err := agentTool.Tool.Execute(ctx, projectId, tenantId, agentTool.ActionName, toolParams)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, err
			}
			if toolOutputType == "json" {
				if len(agentTools) == 1 {
					toolResults = toolResult
				} else {
					toolResults[toolKey] = toolResult
				}
			} else {
				toolResultBytes, err := json.Marshal(toolResult)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, err
				}
				toolResults[toolKey] = string(toolResultBytes)
			}
		}
		if len(agentTool.DependentTools) > 0 {
			dependentToolResults, err := agent.ExecuteTools(ctx, chatRequest, agentTool.DependentTools, projectId, tenantId)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return nil, err
			}
			for key, value := range dependentToolResults {
				toolResults[key] = value
			}
		}
	}
	return toolResults, nil
}
func (agent *Agent) SetChatMemory(ctx context.Context, cacheStoreI cache.CacheStoreI) error {
	logs.WithContext(ctx).Debug("SetChatMemory - Start")
	agent.ChatMemory = cacheStoreI
	return nil
}
func (agent *Agent) ValidateChatMemory(ctx context.Context, projectId string) error {
	logs.WithContext(ctx).Debug("ValidateChatMemory - Start")
	if agent.ChatMemory == nil {
		_ = logs.Err(ctx, fmt.Errorf("chat memory is not set"), "chat memory is not set")
		return nil
	}
	return agent.ChatMemory.ValidatePersistence(ctx, projectId)
}

func (agent *Agent) UnmarshalJSON(b []byte) error {
	logs.Logger.Info("Agent UnmarshalJSON - Start")
	ctx := context.Background()
	type TempAgent struct {
		AgentType    string                `json:"agent_type"`
		AgentName    string                `json:"agent_name"`
		Description  string                `json:"description"`
		SystemPrompt string                `json:"system_prompt"`
		AgentTools   []AgentTools          `json:"agent_tools"`
		ModelName    string                `json:"model"`
		OutputSchema eru_models.JSONSchema `json:"output_schema"`
		RetryCount   int                   `json:"retry_count"`
	}
	var tempAgent TempAgent
	if err := json.Unmarshal(b, &tempAgent); err != nil {
		err = logs.Err(ctx, err, "failed to unmarshal agent")
		return err
	}
	agent.AgentType = tempAgent.AgentType
	agent.AgentName = tempAgent.AgentName
	agent.Description = tempAgent.Description
	agent.SystemPrompt = tempAgent.SystemPrompt
	agent.AgentTools = tempAgent.AgentTools
	agent.ModelName = tempAgent.ModelName
	agent.OutputSchema = tempAgent.OutputSchema
	agent.RetryCount = tempAgent.RetryCount

	var agentMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &agentMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	var cacheStoreObj map[string]*json.RawMessage
	var cacheStoreJson *json.RawMessage
	if _, ok := agentMap["chat_memory"]; ok {
		if agentMap["chat_memory"] != nil {
			err = json.Unmarshal(*agentMap["chat_memory"], &cacheStoreObj)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			err = json.Unmarshal(*agentMap["chat_memory"], &cacheStoreJson)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			var cacheStoreType string
			if _, seOk := cacheStoreObj["cache_store_type"]; seOk {
				err = json.Unmarshal(*cacheStoreObj["cache_store_type"], &cacheStoreType)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				cacheStoreI := cache.GetCacheStore(cacheStoreType, "")
				err = cacheStoreI.MakeFromJson(ctx, cacheStoreJson)
				if err == nil {
					agent.ChatMemory = cacheStoreI
				} else {
					return err
				}
			} else {
				logs.WithContext(ctx).Info("ignoring secret manager as sm_store_type attribute not found")
			}
		}
	}
	return nil
}
func (agent *Agent) GetChatMemory() cache.CacheStoreI {
	return agent.ChatMemory
}

func (agent *Agent) LoadConversationHistory(ctx context.Context, conversationId, projectId, tenantId string) (*Conversation, error) {
	logs.WithContext(ctx).Debug("LoadConversationHistory - Start")

	conversation := &Conversation{
		ConversationId: conversationId,
		Messages:       []ConversationMessage{},
		NewMessages:    []ConversationMessage{},
	}

	if agent.ChatMemory == nil || conversationId == "" {
		logs.WithContext(ctx).Info("Chat memory not configured or no conversation ID, returning empty conversation")
		return conversation, nil
	}

	messages, err := agent.loadMessages(ctx, conversationId)
	if err != nil {
		logs.WithContext(ctx).Info(fmt.Sprintf("Failed to load messages: %v, returning empty conversation", err))
		return conversation, nil
	}

	conversation.Messages = messages

	logs.WithContext(ctx).Info(fmt.Sprintf("Loaded conversation with %d messages", len(conversation.Messages)))
	return conversation, nil
}

func (agent *Agent) loadMessages(ctx context.Context, conversationId string) ([]ConversationMessage, error) {
	logs.WithContext(ctx).Debug("loadMessages - Start")

	var messages []ConversationMessage

	conversationJSON, err := agent.ChatMemory.Get(ctx, conversationId)
	if err != nil {
		logs.WithContext(ctx).Info(fmt.Sprintf("Failed to get conversation from cache: %v", err))

		persistEnabled, _ := agent.ChatMemory.GetAttribute(ctx, "persist_enabled")
		if persistEnabled != nil && persistEnabled.(bool) {
			logs.WithContext(ctx).Info("Loading messages from database")
			dbMessages, err := agent.ChatMemory.LoadMessagesFromDatabase(ctx, "", conversationId)
			if err != nil {
				logs.WithContext(ctx).Info(fmt.Sprintf("Failed to load messages from database: %v", err))
				return messages, nil
			}

			for _, dbMsg := range dbMessages {
				var msg ConversationMessage
				err := json.Unmarshal([]byte(dbMsg.Value), &msg)
				if err != nil {
					logs.WithContext(ctx).Error(fmt.Sprintf("Failed to unmarshal message: %v", err))
					continue
				}
				messages = append(messages, msg)
			}

			if len(messages) > 0 {
				messagesJSON, _ := json.Marshal(messages)
				agent.ChatMemory.Set(ctx, conversationId, string(messagesJSON))
			}
		}
	} else {
		err = json.Unmarshal([]byte(conversationJSON), &messages)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("Failed to unmarshal conversation: %v", err))
			return messages, nil
		}
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("Loaded %d messages", len(messages)))
	return messages, nil
}

func (agent *Agent) SaveConversation(ctx context.Context, conversation *Conversation, projectId string) error {
	logs.WithContext(ctx).Debug("SaveConversation - Start")

	if agent.ChatMemory == nil || len(conversation.NewMessages) == 0 {
		logs.WithContext(ctx).Info("Chat memory not configured or no new messages to save")
		return nil
	}

	conversationJSON, err := json.Marshal(conversation.Messages)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to marshal conversation: %v", err))
		return err
	}

	err = agent.ChatMemory.Set(ctx, conversation.ConversationId, string(conversationJSON))
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to save conversation to cache: %v", err))
		return err
	}

	persistEnabled, _ := agent.ChatMemory.GetAttribute(ctx, "persist_enabled")
	if persistEnabled != nil && persistEnabled.(bool) {
		ttl := 30 * 24 * time.Hour
		for _, msg := range conversation.NewMessages {
			go agent.saveMessageToDatabase(ctx, conversation.ConversationId, msg, string(conversationJSON), ttl)
		}
	}

	newMessageCount := len(conversation.NewMessages)
	conversation.NewMessages = []ConversationMessage{}
	logs.WithContext(ctx).Info(fmt.Sprintf("Saved %d new messages for conversation %s", newMessageCount, conversation.ConversationId))

	return nil
}

func (agent *Agent) saveMessageToDatabase(ctx context.Context, conversationId string, msg ConversationMessage, msgJSON string, ttl time.Duration) {
	logs.WithContext(ctx).Debug("saveMessageToDatabase - Start")

	err := agent.ChatMemory.SyncMessageToDatabase(ctx, "", conversationId, msg.MessageId, msg.Role, msgJSON, msg.Timestamp, ttl)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to sync message to database: %v", err))
	} else {
		logs.WithContext(ctx).Info(fmt.Sprintf("Successfully synced message to database: %s", msg.MessageId))
	}
}

func (agent *Agent) AddMessageToConversation(ctx context.Context, conversation *Conversation, role string, agentMessage AgentMessage, response map[string]interface{}) error {
	logs.WithContext(ctx).Debug("AddMessageToConversation - Start")

	if conversation == nil {
		return fmt.Errorf("conversation cannot be nil")
	}

	message := ConversationMessage{
		MessageId: agentMessage.MessageId,
		Timestamp: time.Now(),
		Role:      role,
		Content:   agentMessage.Content,
		Params:    agentMessage.Params,
		Files:     agentMessage.Files,
	}

	if role == "assistant" && response != nil {
		message.ToolResults = response
		if content, ok := response["content"].(string); ok {
			message.Content = content
		}
	}

	if message.MessageId == "" {
		message.MessageId = fmt.Sprintf("%s_%d", role, time.Now().UnixNano())
	}

	conversation.Messages = append(conversation.Messages, message)
	conversation.NewMessages = append(conversation.NewMessages, message)

	logs.WithContext(ctx).Info(fmt.Sprintf("Added %s message to conversation. Total messages: %d",
		role, len(conversation.Messages)))

	return nil
}
