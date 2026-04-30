package module_server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/module_store"
	"github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	"github.com/eru-tech/eru/eru-server/server"
	"github.com/google/uuid"
)

const (
	MCPProtocolVersion = "2025-03-26"
	ServerName         = "eru-ai-mcp-server"
	ServerVersion      = "1.0.1"
	mcpNameSep         = "__"
	mcpActionSep       = "___"
)

var supportedMCPVersions = []string{"2025-06-18", "2025-03-26"}

func negotiateMCPVersion(clientVersion string) string {
	for _, v := range supportedMCPVersions {
		if v == clientVersion {
			return v
		}
	}
	return supportedMCPVersions[0]
}

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
		ProtocolVersion: negotiateMCPVersion(params.ProtocolVersion),
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

						toolDescription := ""
						if desc, err := tool.GetAttribute(ctx, "description"); err == nil {
							if descStr, ok := desc.(string); ok {
								toolDescription = descStr
							}
						}
						toolMCPName := toolName
						if projectId == tenantId && projectId != "" {
							toolMCPName = strings.Join([]string{projectId, toolName}, mcpNameSep)
						}

						actions := tool.GetActions()
						if len(actions) > 0 {
							for _, action := range actions {
								actionDescription := toolDescription
								if action.Description != "" {
									actionDescription = toolDescription + " - " + action.Description
								}
								var actionSchema interface{}
								if action.GetParameters != nil {
									actionSchema = map[string]interface{}{"params": action.GetParameters()}
								} else {
									actionSchema = action.Parameters
								}
								mcpTools = append(mcpTools, server.MCPTool{
									Name:        toolMCPName + mcpActionSep + action.ActionName,
									Description: actionDescription,
									InputSchema: s.convertSchemaToInputSchema(actionSchema),
								})
							}
						} else {
							mcpTools = append(mcpTools, server.MCPTool{
								Name:        toolMCPName,
								Description: toolDescription,
								InputSchema: s.convertSchemaToInputSchema(tool.GetParameters()),
							})
						}
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

						properties := map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "Input message for the agent",
							},
							"params": map[string]interface{}{
								"type":        "object",
								"description": "Additional parameters for the agent",
							},
							"conversation_id": map[string]interface{}{
								"type":        "string",
								"description": "Optional conversation id to continue an existing conversation. When provided, the agent loads the prior conversation history for this id and appends the new message to it. Omit (or leave empty) to start a fresh conversation — the agent will generate a new id and return it in the response.",
							},
							"files": map[string]interface{}{
								"type":        "array",
								"description": "Optional list of files to attach to the agent message (images, documents, etc.).",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"name": map[string]interface{}{
											"type":        "string",
											"description": "File name including extension (e.g. invoice.pdf).",
										},
										"content": map[string]interface{}{
											"type":        "string",
											"description": "Base64-encoded file contents.",
										},
										"mime_type": map[string]interface{}{
											"type":        "string",
											"description": "MIME type of the file (e.g. application/pdf, image/png).",
										},
									},
									"required": []string{"name", "content"},
								},
							},
						}
						if s.agentHasStructuredOutput(ctx, agent) {
							properties["code"] = map[string]interface{}{
								"type":        "string",
								"description": "Existing structured output (as a JSON string) that the agent should build on top of. Provide this when modifying or extending an output produced earlier — especially when the prior conversation history is not available to the caller. The agent will treat this as the baseline and apply the new instruction on top of it.",
							}
						}

						mcpTool := server.MCPTool{
							Name:        strings.Join([]string{agentPrefix, agentName}, mcpNameSep),
							Description: description,
							InputSchema: map[string]interface{}{
								"type":       "object",
								"properties": properties,
								"required":   []string{"content"},
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
	mcpName := params.Name
	actionName := ""
	if idx := strings.Index(mcpName, mcpActionSep); idx != -1 {
		actionName = mcpName[idx+len(mcpActionSep):]
		mcpName = mcpName[:idx]
	}

	parts := s.parseToolName(mcpName)
	isAgent := false
	toolAgentName := ""
	switch {
	case len(parts) == 1:
		toolAgentName = parts[0]
	case len(parts) == 2 && parts[0] == "agent":
		isAgent = true
		toolAgentName = parts[1]
	case len(parts) == 2:
		toolAgentName = parts[1]
	case len(parts) == 3 && parts[0] == "agent":
		isAgent = true
		toolAgentName = parts[2]
	default:
		return server.MCPCallToolResult{}, fmt.Errorf("invalid tool name format: %s", params.Name)
	}
	if isAgent {
		return s.executeAgent(ctx, conversationId, projectId, tenantId, toolAgentName, params.Arguments)
	}
	return s.executeToolAction(ctx, conversationId, projectId, tenantId, toolAgentName, actionName, params.Arguments)
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

	code, _ := arguments["code"].(string)

	if argConvId, ok := arguments["conversation_id"].(string); ok && argConvId != "" {
		conversationId = argConvId
	}
	if conversationId == "" {
		conversationId = uuid.New().String()
	}

	agentMessage := agents.AgentMessage{
		Content: content,
		Code:    code,
		Params:  params,
		Files:   files,
	}

	streamCb := agents.StreamCallback(func(event agents.StreamEvent) {
		eventBytes, mErr := json.Marshal(event)
		if mErr != nil {
			logs.WithContext(ctx).Debug(fmt.Sprintf("mcp agent stream event: %s (iter %d)", event.Event, event.Iteration))
			return
		}
		logs.WithContext(ctx).Debug(fmt.Sprintf("mcp agent stream event: %s", string(eventBytes)))
	})
	ctx = agents.WithStreamCallback(ctx, streamCb)

	result, err := agent.Execute(ctx, agentMessage, conversationId, project, tenant)
	result.ConversationId = conversationId
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

func (s *EruAIMCPServer) executeToolAction(ctx context.Context, conversationId, project, tenant, toolName, actionName string, arguments map[string]interface{}) (server.MCPCallToolResult, error) {
	tool, err := s.store.Store.GetTool(ctx, project, tenant, toolName, "", s.store.Store)
	if err != nil {
		return server.MCPCallToolResult{}, err
	}

	ctx = context.WithValue(ctx, "eruauthbaseurl", module_store.Eruauthbaseurl)
	ctx = context.WithValue(ctx, "eruaiport", module_store.Eruaiport)
	ctx = context.WithValue(ctx, "eruqlbaseurl", module_store.Eruqlbaseurl)
	ctx = context.WithValue(ctx, tools.EruFuncBaseUrlKey, module_store.Erufuncbaseurl)

	toolParams := map[string]interface{}{}
	if wrapped, ok := arguments["params"].(map[string]interface{}); ok {
		toolParams = wrapped
	}

	result, _, err := tool.Execute(ctx, project, tenant, actionName, toolParams)
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

func (s *EruAIMCPServer) agentHasStructuredOutput(ctx context.Context, agent agents.AgentI) bool {
	if schemaI, err := agent.GetAttribute(ctx, "output_schema"); err == nil {
		if js, ok := schemaI.(eru_models.JSONSchema); ok && js.Type != "" {
			return true
		}
	}
	if provider := agent.GetProvider(); provider != nil {
		if js := provider.GetOutputSchema(ctx); js.Type != "" {
			return true
		}
	}
	return false
}

func (s *EruAIMCPServer) convertSchemaToInputSchema(schema interface{}) map[string]interface{} {
	defaultSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}

	switch v := schema.(type) {
	case eru_models.JSONSchema:
		return s.jsonSchemaToMap(v)
	case map[string]interface{}:
		properties := map[string]interface{}{}
		required := []string{}
		for name, val := range v {
			if js, ok := val.(eru_models.JSONSchema); ok {
				properties[name] = s.jsonSchemaToMap(js)
				required = append(required, name)
			}
		}
		out := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			out["required"] = required
		}
		return out
	default:
		return defaultSchema
	}
}

func (s *EruAIMCPServer) jsonSchemaToMap(jsonSchema eru_models.JSONSchema) map[string]interface{} {
	out := map[string]interface{}{}

	if jsonSchema.Type != "" {
		out["type"] = jsonSchema.Type
	} else {
		out["type"] = "object"
	}

	if jsonSchema.Description != "" {
		out["description"] = jsonSchema.Description
	}
	if jsonSchema.Format != "" {
		out["format"] = jsonSchema.Format
	}
	if len(jsonSchema.Enum) > 0 {
		out["enum"] = jsonSchema.Enum
	}

	if len(jsonSchema.Properties) > 0 {
		properties := map[string]interface{}{}
		for name, prop := range jsonSchema.Properties {
			properties[name] = s.jsonSchemaToMap(prop)
		}
		out["properties"] = properties
	} else if jsonSchema.Type == "object" || jsonSchema.Type == "" {
		out["properties"] = map[string]interface{}{}
	}

	if len(jsonSchema.Required) > 0 {
		out["required"] = jsonSchema.Required
	}

	if jsonSchema.Items != nil {
		out["items"] = s.jsonSchemaToMap(*jsonSchema.Items)
	}

	if jsonSchema.AdditionalProperties != nil {
		if apSchema, ok := jsonSchema.AdditionalProperties.(eru_models.JSONSchema); ok {
			out["additionalProperties"] = s.jsonSchemaToMap(apSchema)
		} else {
			out["additionalProperties"] = jsonSchema.AdditionalProperties
		}
	}

	return out
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
