package module_server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	"github.com/eru-tech/eru/eru-server/server"
)

const (
	MCPProtocolVersion = "2025-03-26"
	ServerName         = "eru-ai-mcp-server"
	ServerVersion      = "1.0.1"
	mcpNameSep         = "__"
)

type EruAIMCPServer struct {
	store        *module_store.StoreHolder
	capabilities server.MCPCapabilities
}

func NewEruAIMCPServer(store *module_store.StoreHolder) *EruAIMCPServer {
	return &EruAIMCPServer{
		store: store,
		capabilities: server.MCPCapabilities{
			Tools: &server.MCPToolsCapability{
				ListChanged: true,
			},
		},
	}
}

func (s *EruAIMCPServer) Initialize(ctx context.Context, params server.MCPInitializeParams) (server.MCPInitializeResult, error) {
	logs.WithContext(ctx).Info(fmt.Sprintf("MCP client initializing: %s v%s", params.ClientInfo.Name, params.ClientInfo.Version))

	return server.MCPInitializeResult{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    s.capabilities,
		ServerInfo: server.MCPServerInfo{
			Name:    ServerName,
			Version: ServerVersion,
		},
	}, nil
}

func (s *EruAIMCPServer) ListTools(ctx context.Context, projectId string, tenantId string) (server.MCPListToolsResult, error) {
	var mcpTools []server.MCPTool

	projectList := s.store.Store.GetProjectList(ctx)

	for _, projectInfo := range projectList {
		projectName, ok := projectInfo["project_name"].(string)
		if !ok {
			continue
		}
		if projectId == "" || projectName == projectId {
			project, err := s.store.Store.GetProjectConfig(ctx, projectName)
			if err != nil {
				continue
			}

			for _, tenant := range project.Tenants {
				if tenantId == "" || tenant.TenantId == tenantId || tenant.TenantId == projectId {
					toolNames, err := s.store.Store.GetToolNames(ctx, projectName, tenant.TenantId)
					if err != nil {
						continue
					}

					for _, toolName := range toolNames {
						tool, err := s.store.Store.GetTool(ctx, projectName, tenant.TenantId, toolName, "", s.store.Store)
						if err != nil {
							continue
						}

						description := ""
						if desc, err := tool.GetAttribute(ctx, "description"); err == nil {
							if descStr, ok := desc.(string); ok {
								description = descStr
							}
						}

						toolPrefix := "tool"
						if projectId == tenantId && projectId != "" {
							toolPrefix = strings.Join([]string{"tool", projectId}, mcpNameSep)
						}
						mcpTool := server.MCPTool{
							Name:        strings.Join([]string{toolPrefix, toolName}, mcpNameSep),
							Description: description,
							InputSchema: s.convertSchemaToInputSchema(tool.GetParameters()),
						}
						mcpTools = append(mcpTools, mcpTool)
					}

					agentNames, err := s.store.Store.GetAgentNames(ctx, projectName, tenant.TenantId)
					if err != nil {
						continue
					}

					for _, agentName := range agentNames {
						agent, err := s.store.Store.GetAgent(ctx, projectName, tenant.TenantId, "", agentName, s.store.Store)
						if err != nil {
							continue
						}

						description := ""
						if desc, err := agent.GetAttribute(ctx, "description"); err == nil {
							if descStr, ok := desc.(string); ok {
								description = descStr
							}
						}

						if description == "" {
							description = fmt.Sprintf("AI Agent: %s", agentName)
						}
						agentPrefix := "agent"
						if projectId == tenantId && projectId != "" {
							agentPrefix = strings.Join([]string{"agent", projectId}, mcpNameSep)
						}
						mcpTool := server.MCPTool{
							Name:        strings.Join([]string{agentPrefix, agentName}, mcpNameSep),
							Description: description,
							InputSchema: map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"content": map[string]interface{}{
										"type":        "string",
										"description": "Input message for the agent",
									},
									"params": map[string]interface{}{
										"type":        "object",
										"description": "Additional parameters for the agent",
									},
								},
								"required": []string{"content"},
							},
						}
						mcpTools = append(mcpTools, mcpTool)
					}
				}
			}
		}
	}

	if mcpTools == nil {
		mcpTools = []server.MCPTool{}
	}

	return server.MCPListToolsResult{
		Tools: mcpTools,
	}, nil
}

func (s *EruAIMCPServer) CallTool(ctx context.Context, conversationId string, params server.MCPCallToolParams, projectId string, tenantId string) (server.MCPCallToolResult, error) {
	parts := s.parseToolName(params.Name)
	if len(parts) < 2 {
		return server.MCPCallToolResult{}, fmt.Errorf("invalid tool name format: %s", params.Name)
	}
	toolAgentName := parts[1]
	if len(parts) == 3 {
		toolAgentName = parts[2]
	}
	if parts[0] == "agent" {
		return s.executeAgent(ctx, conversationId, projectId, tenantId, toolAgentName, params.Arguments)
	}
	return s.executeToolAction(ctx, conversationId, projectId, tenantId, toolAgentName, params.Arguments)
}

func (s *EruAIMCPServer) executeAgent(ctx context.Context, conversationId, project, tenant, agentName string, arguments map[string]interface{}) (server.MCPCallToolResult, error) {
	agent, err := s.store.Store.GetAgent(ctx, project, tenant, "", agentName, s.store.Store)
	if err != nil {
		return server.MCPCallToolResult{}, err
	}

	content, ok := arguments["content"].(string)
	if !ok {
		return server.MCPCallToolResult{}, fmt.Errorf("content parameter required")
	}

	params, _ := arguments["params"].(map[string]interface{})
	if params == nil {
		params = make(map[string]interface{})
	}

	var files []models.FileMessage
	if filesArg, ok := arguments["files"].([]interface{}); ok {
		for _, fileData := range filesArg {
			if fileMap, ok := fileData.(map[string]interface{}); ok {
				file := models.FileMessage{}

				if name, ok := fileMap["name"].(string); ok {
					file.FileName = name
				}

				if content, ok := fileMap["content"].(string); ok {
					file.FileData = content
				}

				if mimeType, ok := fileMap["mime_type"].(string); ok {
					file.FileType = mimeType
				}

				files = append(files, file)
			}
		}
	}

	agentMessage := agents.AgentMessage{
		Content: content,
		Params:  params,
		Files:   files,
	}

	result, err := agent.Execute(ctx, agentMessage, conversationId, project, tenant)
	if err != nil {
		return server.MCPCallToolResult{
			Content: []server.MCPContent{
				{
					Type: "text",
					Text: fmt.Sprintf("Agent execution failed: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	resultText := s.formatResult(result)
	return server.MCPCallToolResult{
		Content: []server.MCPContent{
			{
				Type: "text",
				Text: resultText,
			},
		},
	}, nil
}

func (s *EruAIMCPServer) executeToolAction(ctx context.Context, conversationId, project, tenant, toolName string, arguments map[string]interface{}) (server.MCPCallToolResult, error) {
	tool, err := s.store.Store.GetTool(ctx, project, tenant, toolName, "", s.store.Store)
	if err != nil {
		return server.MCPCallToolResult{}, err
	}

	result, _, err := tool.Execute(ctx, project, tenant, "", arguments)
	if err != nil {
		return server.MCPCallToolResult{
			Content: []server.MCPContent{
				{
					Type: "text",
					Text: fmt.Sprintf("Tool execution failed: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	resultText := s.formatResult(result)
	return server.MCPCallToolResult{
		Content: []server.MCPContent{
			{
				Type: "text",
				Text: resultText,
			},
		},
	}, nil
}

func (s *EruAIMCPServer) GetCapabilities() server.MCPCapabilities {
	return s.capabilities
}

func (s *EruAIMCPServer) GetServerInfo() server.MCPServerInfo {
	return server.MCPServerInfo{
		Name:    ServerName,
		Version: ServerVersion,
	}
}

// parseToolName splits an MCP tool name on the __ separator.
// Tool names use the format: project__tenant__toolname
// Agent names use the format: project__tenant__agent__agentname
func (s *EruAIMCPServer) parseToolName(toolName string) []string {
	return strings.Split(toolName, mcpNameSep)
}

func (s *EruAIMCPServer) convertSchemaToInputSchema(schema interface{}) map[string]interface{} {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	jsonSchema, ok := schema.(eru_models.JSONSchema)
	if !ok {
		return inputSchema
	}

	if jsonSchema.Type != "" {
		inputSchema["type"] = jsonSchema.Type
	}

	if len(jsonSchema.Properties) > 0 {
		properties := map[string]interface{}{}
		for name, prop := range jsonSchema.Properties {
			propMap := map[string]interface{}{
				"type": prop.Type,
			}
			if prop.Description != "" {
				propMap["description"] = prop.Description
			}
			if prop.Format != "" {
				propMap["format"] = prop.Format
			}
			if len(prop.Enum) > 0 {
				propMap["enum"] = prop.Enum
			}
			if prop.Items != nil {
				propMap["items"] = s.convertSchemaToInputSchema(*prop.Items)
			}
			properties[name] = propMap
		}
		inputSchema["properties"] = properties
	}

	if len(jsonSchema.Required) > 0 {
		inputSchema["required"] = jsonSchema.Required
	}

	return inputSchema
}

func (s *EruAIMCPServer) formatResult(result interface{}) string {
	if result == nil {
		return "Operation completed successfully"
	}

	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Result: %v", result)
	}

	return string(resultBytes)
}
