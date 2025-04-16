package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	AnthropicApiUrl     = "https://api.anthropic.com/v1/messages"
	AnthropicApiVersion = "2023-06-01"
)

type AnthropicModel struct {
	Model
}

// Request Types
type AnthropicChatRequest struct {
	Model         string                   `json:"model" eru:"required"`
	Messages      []AnthropicMessage       `json:"messages" eru:"required"`
	MaxTokens     int                      `json:"max_tokens,omitempty"`
	StopSequences []string                 `json:"stop_sequences,omitempty"`
	Stream        bool                     `json:"stream,omitempty"`
	Temperature   float64                  `json:"temperature,omitempty"`
	System        string                   `json:"system,omitempty"`
	Metadata      AnthropicMessageMetaData `json:"metadata,omitempty"`
}

type AnthropicChatToolRequest struct {
	AnthropicChatRequest
	ToolChoice AnthropicToolChoice    `json:"tool_choice,omitempty"`
	Tools      []AnthropicRequestTool `json:"tools,omitempty"`
	TopK       int                    `json:"top_k,omitempty"`
	TopP       float64                `json:"top_p,omitempty"`
}

type AnthropicMessageMetaData struct {
	UserId string `json:"user_id"`
}

type AnthropicMessage struct {
	Role    string                    `json:"role"`
	Content []AnthropicMessageContent `json:"content"`
}

type AnthropicMessageContent struct {
	Text string `json:"text"`
	//Source       AnthropicContentSource `json:"source,omitempty"`
	Id           string                `json:"id,omitempty"`
	ToolUseId    string                `json:"tool_use_id,omitempty"`
	Content      string                `json:"content,omitempty"`
	IsError      bool                  `json:"is_error,omitempty"`
	Name         string                `json:"name,omitempty"`
	Input        interface{}           `json:"input,omitempty"`
	Type         string                `json:"type" eru:"required"`
	CacheControl AnthropicCacheControl `json:"cache_control,omitempty"`
}

type AnthropicContentSource struct {
	Data      string `json:"data"`
	MediaType string `json:"media_type"`
	Type      string `json:"type"`
}
type AnthropicCacheControl struct {
	Type string `json:"type" eru:"required"`
}

type AnthropicToolChoice struct {
	//Name                   string `json:"name" eru:"required"`
	Type string `json:"type" eru:"required"`
	//DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
}

type AnthropicRequestTool struct {
	//Type         string                `json:"type" eru:"required"`
	Name        string                `json:"name" eru:"required"`
	Description string                `json:"description" eru:"required"`
	InputSchema eru_models.JSONSchema `json:"input_schema,omitempty"`
	//CacheControl AnthropicCacheControl `json:"cache_control,omitempty"`
}

// Response Types
type AnthropicChatResponse struct {
	Id           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence"`
	Usage        AnthropicUsage `json:"usage"`
}

type ContentBlock struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	Id    string      `json:"id,omitempty"`
	Input interface{} `json:"input,omitempty"`
	Name  string      `json:"name,omitempty"`
}

type AnthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

// Implementation of ModelI interface
func (anthropicModel *AnthropicModel) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &anthropicModel)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (anthropicModel *AnthropicModel) QueryModel(ctx context.Context, chatRequest ChatRequest) (queryResponse Message, err error) {
	logs.WithContext(ctx).Debug("QueryModel - Start")

	anthropicRequest, err := anthropicModel.makeAnthropicChatRequest(ctx, chatRequest)
	if err != nil {
		return
	}

	anthropicResponse, err := anthropicModel.queryModel(ctx, anthropicRequest)
	if err != nil {
		return
	}

	// Combine all text blocks from content
	var combinedContent string
	for _, block := range anthropicResponse.Content {
		if block.Type == "text" {
			combinedContent += block.Text
		}
	}

	queryResponse = Message{
		Content: combinedContent,
		Role:    "assistant",
	}
	return
}

func (anthropicModel *AnthropicModel) makeAnthropicChatRequest(ctx context.Context, chatRequest ChatRequest) (anthropicRequest AnthropicChatRequest, err error) {
	logs.WithContext(ctx).Debug("makeAnthropicChatRequest - Start")

	anthropicRequest = AnthropicChatRequest{
		Model:       anthropicModel.LLMName,
		MaxTokens:   1024,
		Temperature: anthropicModel.Temprature,
	}

	for _, message := range chatRequest.Messages {
		aContent := AnthropicMessageContent{
			Type:         "text",
			Text:         message.Content,
			CacheControl: AnthropicCacheControl{Type: "ephemeral"},
		}
		anthropicRequest.Messages = append(anthropicRequest.Messages, AnthropicMessage{
			Role:    message.Role,
			Content: []AnthropicMessageContent{aContent},
		})
	}

	return
}

func (anthropicModel *AnthropicModel) queryModel(ctx context.Context, chatRequest AnthropicChatRequest) (anthropicResponse AnthropicChatResponse, err error) {
	logs.WithContext(ctx).Debug("queryModel - Start")

	reqHeader := http.Header{}
	reqHeader.Add("x-api-key", anthropicModel.LLMSecret)
	reqHeader.Add("anthropic-version", AnthropicApiVersion)
	reqHeader.Add("Content-Type", "application/json")

	logs.WithContext(ctx).Info(fmt.Sprint(chatRequest))

	response, _, _, _, err := utils.CallHttp(ctx, "POST", AnthropicApiUrl, reqHeader, nil, nil, nil, chatRequest)
	if err != nil {
		return
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	err = json.Unmarshal(responseJson, &anthropicResponse)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	logs.WithContext(ctx).Info(fmt.Sprint(anthropicResponse))
	return
}

// Add the tool implementation
func (anthropicModel *AnthropicModel) QueryModelWithTool(ctx context.Context, chatRequest ChatRequest, tools map[string]tools.Tooling, agentName string, agentPrompt string) (queryResponse JsonMessage, err error) {
	logs.WithContext(ctx).Debug("QueryModelWithTool - Start")
	anthropicChatToolRequest, err := anthropicModel.makeAnthropicChatToolRequest(ctx, chatRequest, tools, agentName, agentPrompt)
	if err != nil {
		return
	}
	anthropicChatResponse, err := anthropicModel.queryModelTool(ctx, anthropicChatToolRequest)
	if err != nil {
		return
	}
	outputJson := make(map[string]interface{})
	outputJson["raw"] = anthropicChatResponse.Content
	queryResponse = JsonMessage{
		Content: outputJson,
		Role:    "assistant",
	}
	return
}
func (anthropicModel *AnthropicModel) makeAnthropicChatToolRequest(ctx context.Context, chatRequest ChatRequest, tools map[string]tools.Tooling, agentName string, agentPrompt string) (anthropicChatRequest AnthropicChatToolRequest, err error) {
	logs.WithContext(ctx).Debug("makeAnthropicChatToolRequest - Start")

	var anthropicRequestTools []AnthropicRequestTool
	toolPrompt := ""
	for _, tool := range tools {
		toolNameI, _ := tool.GetAttribute(ctx, "tool_name")
		toolDescriptionI, _ := tool.GetAttribute(ctx, "description")
		toolParametersI, _ := tool.GetAttribute(ctx, "parameters")
		toolSystemPromptI, _ := tool.GetAttribute(ctx, "system_prompt")
		toolName := toolNameI.(string)
		toolDescription := toolDescriptionI.(string)
		toolParameters := toolParametersI.(eru_models.JSONSchema)

		toolPrompt += fmt.Sprint("Tool prompt for Tool ", toolName, " is as follows :\n", toolSystemPromptI.(string))
		reqTool := AnthropicRequestTool{
			Name:        toolName,
			Description: toolDescription,
			InputSchema: toolParameters,
			//Type:         "custom",
			//CacheControl: AnthropicCacheControl{Type: "ephemeral"},
		}
		anthropicRequestTools = append(anthropicRequestTools, reqTool)
	}
	anthropicChatRequest.Model = anthropicModel.LLMName
	anthropicChatRequest.Temperature = anthropicModel.Temprature
	anthropicChatRequest.MaxTokens = 150
	//anthropicChatRequest.StopSequences = []string{""}
	anthropicChatRequest.Stream = false
	anthropicChatRequest.System = ""
	anthropicChatRequest.Metadata = AnthropicMessageMetaData{}
	anthropicChatRequest.ToolChoice = AnthropicToolChoice{
		Type: "any",
	}
	anthropicChatRequest.Tools = anthropicRequestTools
	//anthropicChatRequest.Metadata.Content.CacheControl = AnthropicCacheControl{Type: "ephemeral"}

	anthropicChatRequest.Messages = append(anthropicChatRequest.Messages, AnthropicMessage{
		Role:    "assistant",
		Content: []AnthropicMessageContent{{Type: "text", Text: fmt.Sprint(agentPrompt, "\n", toolPrompt), CacheControl: AnthropicCacheControl{Type: "ephemeral"}}},
	})

	for _, message := range chatRequest.Messages {
		anthropicChatRequest.Messages = append(anthropicChatRequest.Messages, AnthropicMessage{
			Role:    message.Role,
			Content: []AnthropicMessageContent{{Type: "text", Text: message.Content, CacheControl: AnthropicCacheControl{Type: "ephemeral"}}},
		})
	}
	return
}

func (anthropicModel *AnthropicModel) queryModelTool(ctx context.Context, toolRequest AnthropicChatToolRequest) (anthropicResponse AnthropicChatResponse, err error) {
	logs.WithContext(ctx).Debug("queryModelTool - Start")

	reqHeader := http.Header{}
	reqHeader.Add("x-api-key", anthropicModel.LLMSecret)
	reqHeader.Add("anthropic-version", AnthropicApiVersion)
	reqHeader.Add("Content-Type", "application/json")

	logs.WithContext(ctx).Info(fmt.Sprint(toolRequest))

	response, _, _, _, err := utils.CallHttp(ctx, "POST", AnthropicApiUrl, reqHeader, nil, nil, nil, toolRequest)
	if err != nil {
		return
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	err = json.Unmarshal(responseJson, &anthropicResponse)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	logs.WithContext(ctx).Info(fmt.Sprint(anthropicResponse))
	return
}
