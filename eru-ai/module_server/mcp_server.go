package module_server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/module_store"
	tools_factory "github.com/eru-tech/eru/eru-ai/tools/tools_factory"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-server/server"
)

const (
	MCPProtocolVersion = "2025-06-18"
	ServerName         = "eru-ai-mcp-server"
	ServerVersion      = "1.0.1"
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
			/* Resources: &server.MCPResourcesCapability{
				Subscribe:   true,
				ListChanged: true,
			},
			Prompts: &server.MCPPromptsCapability{
				ListChanged: true,
			},
			Logging: &server.MCPLoggingCapability{
				Level: "info",
			}, */
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

func (s *EruAIMCPServer) ListTools(ctx context.Context) (server.MCPListToolsResult, error) {
	var mcpTools []server.MCPTool

	// Add global tools using tools factory (same as /tools endpoint)
	toolName := "MS_EMAIL" // get it from env variable
	tool := tools_factory.GetTool(toolName)
	globalTools := tool.GetMcpTools()

	// Convert to MCP format
	for _, globalTool := range globalTools {
		mcpTool := server.MCPTool{
			Name:        globalTool.ToolName,
			Description: globalTool.ToolDescription,
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": make(map[string]interface{}),
			},
		}
		mcpTools = append(mcpTools, mcpTool)
	}

	projectList := s.store.Store.GetProjectList(ctx)

	for _, projectInfo := range projectList {
		projectName, ok := projectInfo["project_name"].(string)
		if !ok {
			continue
		}

		project, err := s.store.Store.GetProjectConfig(ctx, projectName)
		if err != nil {
			continue
		}

		for _, tenant := range project.Tenants {
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

				mcpTool := server.MCPTool{
					Name:        fmt.Sprintf("%s_%s_%s", projectName, tenant.TenantId, toolName),
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
				agent, err := s.store.Store.GetAgent(ctx, projectName, tenant.TenantId, agentName, s.store.Store)
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

				mcpTool := server.MCPTool{
					Name:        fmt.Sprintf("%s_%s_agent_%s", projectName, tenant.TenantId, agentName),
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

	return server.MCPListToolsResult{
		Tools: mcpTools,
	}, nil
}

func (s *EruAIMCPServer) CallTool(ctx context.Context, params server.MCPCallToolParams) (server.MCPCallToolResult, error) {
	parts := s.parseToolName(params.Name)
	if len(parts) < 3 {
		return server.MCPCallToolResult{}, fmt.Errorf("invalid tool name format")
	}

	project := parts[0]
	tenant := parts[1]

	if len(parts) == 4 && parts[2] == "agent" {
		return s.executeAgent(ctx, project, tenant, parts[3], params.Arguments)
	} else {
		return s.executeToolAction(ctx, project, tenant, parts[2], params.Arguments)
	}
}

func (s *EruAIMCPServer) executeAgent(ctx context.Context, project, tenant, agentName string, arguments map[string]interface{}) (server.MCPCallToolResult, error) {
	agent, err := s.store.Store.GetAgent(ctx, project, tenant, agentName, s.store.Store)
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

	result, err := agent.Execute(ctx, agentMessage, project, tenant)
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

func (s *EruAIMCPServer) executeToolAction(ctx context.Context, project, tenant, toolName string, arguments map[string]interface{}) (server.MCPCallToolResult, error) {
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

func (s *EruAIMCPServer) parseToolName(toolName string) []string {
	var parts []string
	current := ""

	for _, char := range toolName {
		if char == '_' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func (s *EruAIMCPServer) convertSchemaToInputSchema(schema interface{}) map[string]interface{} {
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
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
