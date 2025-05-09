package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

type MCPClient struct {
	BaseURL    string `json:"base_url" eru:"required"`
	APIKey     string `json:"api_key"`
	Timeout    int    `json:"timeout"`
	httpClient *http.Client
}

type MCPRequest struct {
	Model      string                 `json:"model" eru:"required"`
	Messages   []MCPMessage           `json:"messages" eru:"required"`
	Parameters map[string]interface{} `json:"parameters"`
	Tools      []MCPTool              `json:"tools"`
	ToolChoice string                 `json:"tool_choice"`
}

type MCPMessage struct {
	Role    string                 `json:"role" eru:"required"`
	Content string                 `json:"content" eru:"required"`
	Name    string                 `json:"name"`
	Params  map[string]interface{} `json:"params"`
}

type MCPTool struct {
	Name        string                `json:"name" eru:"required"`
	Description string                `json:"description" eru:"required"`
	Parameters  eru_models.JSONSchema `json:"parameters" eru:"required"`
}

type MCPResponse struct {
	ID      string      `json:"id"`
	Model   string      `json:"model"`
	Choices []MCPChoice `json:"choices"`
	Usage   MCPUsage    `json:"usage"`
}

type MCPChoice struct {
	Message      MCPMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

type MCPUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func NewMCPClient(baseURL string, apiKey string, timeout int) *MCPClient {
	client := &MCPClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Timeout: timeout,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
	return client
}

func (mcp *MCPClient) Query(ctx context.Context, request MCPRequest) (response MCPResponse, err error) {
	logs.WithContext(ctx).Debug("MCP Query - Start")

	reqHeader := http.Header{}
	if mcp.APIKey != "" {
		reqHeader.Add("Authorization", "Bearer "+mcp.APIKey)
	}
	reqHeader.Add("Content-Type", "application/json")

	rawResponse, _, _, _, err := utils.CallHttp(ctx, "POST", mcp.BaseURL+"/v1/chat/completions", reqHeader, nil, nil, nil, request)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("MCP Query error: %v", err))
		return
	}

	responseJson, err := json.Marshal(rawResponse)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("MCP Query marshal error: %v", err))
		return
	}

	err = json.Unmarshal(responseJson, &response)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("MCP Query unmarshal error: %v", err))
		return
	}

	return
}

func (mcp *MCPClient) ListModels(ctx context.Context) (models []string, err error) {
	logs.WithContext(ctx).Debug("MCP ListModels - Start")

	reqHeader := http.Header{}
	if mcp.APIKey != "" {
		reqHeader.Add("Authorization", "Bearer "+mcp.APIKey)
	}
	reqHeader.Add("Content-Type", "application/json")

	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	rawResponse, _, _, _, err := utils.CallHttp(ctx, "GET", mcp.BaseURL+"/v1/models", reqHeader, nil, nil, nil, nil)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("MCP ListModels error: %v", err))
		return
	}

	responseJson, err := json.Marshal(rawResponse)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("MCP ListModels marshal error: %v", err))
		return
	}

	err = json.Unmarshal(responseJson, &response)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("MCP ListModels unmarshal error: %v", err))
		return
	}

	for _, model := range response.Data {
		models = append(models, model.ID)
	}

	return
}
