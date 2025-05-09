package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

type MCPToolImpl struct {
	baseTool tools.Tool
	client   *MCPClient
}

func NewMCPTool(baseURL string, apiKey string, timeout int) *MCPToolImpl {
	return &MCPToolImpl{
		baseTool: tools.Tool{
			ToolType:     "MCP",
			ToolName:     "MCPClient",
			Description:  "A tool that allows interaction with MCP (Model Control Protocol) servers",
			SystemPrompt: "You are a helpful assistant that can interact with MCP servers. You can query models and list available models.",
			OutputSchema: eru_models.JSONSchema{
				Type: "object",
				Properties: map[string]eru_models.JSONSchema{
					"response": {
						Type: "object",
						Properties: map[string]eru_models.JSONSchema{
							"id":    {Type: "string"},
							"model": {Type: "string"},
							"choices": {
								Type: "array",
								Items: &eru_models.JSONSchema{
									Type: "object",
									Properties: map[string]eru_models.JSONSchema{
										"message": {
											Type: "object",
											Properties: map[string]eru_models.JSONSchema{
												"role":    {Type: "string"},
												"content": {Type: "string"},
											},
										},
										"finish_reason": {Type: "string"},
									},
								},
							},
						},
					},
				},
			},
			Parameters: eru_models.JSONSchema{
				Type: "object",
				Properties: map[string]eru_models.JSONSchema{
					"model": {
						Type: "string",
					},
					"messages": {
						Type: "array",
						Items: &eru_models.JSONSchema{
							Type: "object",
							Properties: map[string]eru_models.JSONSchema{
								"role":    {Type: "string"},
								"content": {Type: "string"},
							},
						},
					},
					"parameters": {
						Type: "object",
					},
				},
				Required: []string{"model", "messages"},
			},
		},
		client: NewMCPClient(baseURL, apiKey, timeout),
	}
}

func (mcpTool *MCPToolImpl) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("MCPTool Execute - Start")

	// Convert params to MCPRequest
	request := MCPRequest{}
	jsonData, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %v", err)
	}
	err = json.Unmarshal(jsonData, &request)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal params: %v", err)
	}

	// Execute the request
	response, err := mcpTool.client.Query(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("MCP query failed: %v", err)
	}

	// Convert response to map
	result := make(map[string]interface{})
	result["response"] = response

	return result, nil
}

func (mcpTool *MCPToolImpl) GetSpec() tools.Tooling {
	return mcpTool
}

func (mcpTool *MCPToolImpl) ValidateOutput(ctx context.Context, output json.RawMessage) error {
	return mcpTool.baseTool.ValidateOutput(ctx, output)
}

func (mcpTool *MCPToolImpl) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	return mcpTool.baseTool.MakeFromJson(ctx, rj)
}

func (mcpTool *MCPToolImpl) GetAttribute(ctx context.Context, attributeName string) (interface{}, error) {
	return mcpTool.baseTool.GetAttribute(ctx, attributeName)
}
