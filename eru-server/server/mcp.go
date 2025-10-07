package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type MCPMessage struct {
	JSONRPCVersion string          `json:"jsonrpc"`
	ID             interface{}     `json:"id,omitempty"`
	Method         string          `json:"method,omitempty"`
	Params         json.RawMessage `json:"params,omitempty"`
	Result         json.RawMessage `json:"result,omitempty"`
	Error          *MCPError       `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MCPCapabilities struct {
	Tools     interface{} `json:"tools,omitempty"`
	Resources interface{} `json:"resources,omitempty"`
	Prompts   interface{} `json:"prompts,omitempty"`
	Logging   interface{} `json:"logging,omitempty"`
	// Additional fields that Cursor may send
	Elicitation interface{} `json:"elicitation,omitempty"`
	Roots       interface{} `json:"roots,omitempty"`
}

type MCPToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type MCPResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type MCPPromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type MCPLoggingCapability struct {
	Level string `json:"level,omitempty"`
}

type MCPInitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    MCPCapabilities        `json:"capabilities"`
	ClientInfo      MCPClientInfo          `json:"clientInfo"`
	Meta            map[string]interface{} `json:"meta,omitempty"`
}

type MCPInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    MCPCapabilities `json:"capabilities"`
	ServerInfo      MCPServerInfo   `json:"serverInfo"`
}

type MCPClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type MCPListToolsResult struct {
	Tools      []MCPTool `json:"tools"`
	NextCursor *string   `json:"nextCursor,omitempty"`
}

type MCPCallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type MCPCallToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type MCPServer interface {
	Initialize(ctx context.Context, params MCPInitializeParams) (MCPInitializeResult, error)
	ListTools(ctx context.Context) (MCPListToolsResult, error)
	CallTool(ctx context.Context, conversationId string, params MCPCallToolParams) (MCPCallToolResult, error)
	GetCapabilities() MCPCapabilities
	GetServerInfo() MCPServerInfo
}

type MCPMessageHandler struct {
	server      MCPServer
	initialized bool
}

func NewMCPMessageHandler(server MCPServer) *MCPMessageHandler {
	return &MCPMessageHandler{
		server:      server,
		initialized: false,
	}
}

func (h *MCPMessageHandler) HandleMessage(ctx context.Context, data []byte) ([]byte, error) {
	var request MCPMessage
	if err := json.Unmarshal(data, &request); err != nil {
		return h.createErrorResponse(nil, -32700, "Parse error", nil)
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("MCP request: %s", request.Method))

	switch request.Method {
	case "initialize":
		return h.handleInitialize(ctx, request)
	case "initialized":
		return h.handleInitialized(ctx, request)
	case "tools/list":
		return h.handleListTools(ctx, request)
	case "tools/call":
		return h.handleCallTool(ctx, request)
	default:
		return h.createErrorResponse(request.ID, -32601, "Method not found", nil)
	}
}

func (h *MCPMessageHandler) handleInitialize(ctx context.Context, request MCPMessage) ([]byte, error) {
	logs.WithContext(ctx).Info(fmt.Sprintf("request: %v", request))
	var params MCPInitializeParams
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return h.createErrorResponse(request.ID, -32602, "Invalid params", nil)
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("MCP client: %s v%s", params.ClientInfo.Name, params.ClientInfo.Version))

	result, err := h.server.Initialize(ctx, params)
	if err != nil {
		return h.createErrorResponse(request.ID, -32603, "Initialize failed", err.Error())
	}

	// Mark as initialized
	h.initialized = true

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return h.createErrorResponse(request.ID, -32603, "Internal error", nil)
	}

	response := MCPMessage{
		JSONRPCVersion: "2.0",
		ID:             request.ID,
		Result:         resultBytes,
	}

	return json.Marshal(response)
}

func (h *MCPMessageHandler) handleInitialized(ctx context.Context, request MCPMessage) ([]byte, error) {
	h.initialized = true
	logs.WithContext(ctx).Info("MCP server initialized")
	logs.WithContext(ctx).Info(fmt.Sprintf("request: %v", request))
	response := MCPMessage{
		JSONRPCVersion: "2.0",
		ID:             request.ID,
		Result:         json.RawMessage("{}"),
	}

	return json.Marshal(response)
}

func (h *MCPMessageHandler) handleListTools(ctx context.Context, request MCPMessage) ([]byte, error) {
	if !h.initialized {
		return h.createErrorResponse(request.ID, -32002, "Server not initialized", nil)
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("request: %v", request))
	result, err := h.server.ListTools(ctx)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Error listing tools: %v", err))
		return h.createErrorResponse(request.ID, -32603, "Internal error", nil)
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return h.createErrorResponse(request.ID, -32603, "Internal error", nil)
	}

	response := MCPMessage{
		JSONRPCVersion: "2.0",
		ID:             request.ID,
		Result:         resultBytes,
	}

	return json.Marshal(response)
}

func (h *MCPMessageHandler) handleCallTool(ctx context.Context, request MCPMessage) ([]byte, error) {
	if !h.initialized {
		return h.createErrorResponse(request.ID, -32002, "Server not initialized", nil)
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("request: %v", request))
	var params MCPCallToolParams
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return h.createErrorResponse(request.ID, -32602, "Invalid params", nil)
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("Calling tool: %s", params.Name))
	conversationId := "" //TODO: get conversationId from mcp client request
	result, err := h.server.CallTool(ctx, conversationId, params)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Tool execution error: %v", err))
		return h.createErrorResponse(request.ID, -32603, fmt.Sprintf("Tool execution failed: %v", err), nil)
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return h.createErrorResponse(request.ID, -32603, "Internal error", nil)
	}

	response := MCPMessage{
		JSONRPCVersion: "2.0",
		ID:             request.ID,
		Result:         resultBytes,
	}

	return json.Marshal(response)
}

func (h *MCPMessageHandler) createErrorResponse(id interface{}, code int, message string, data interface{}) ([]byte, error) {
	response := MCPMessage{
		JSONRPCVersion: "2.0",
		ID:             id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return json.Marshal(response)
}

func CreateMCPWebSocketHandler(server MCPServer, config ...WebSocketConfig) http.HandlerFunc {
	var wsConfig WebSocketConfig
	if len(config) > 0 {
		wsConfig = config[0]
		if len(wsConfig.Subprotocols) == 0 {
			wsConfig.Subprotocols = []string{"mcp"}
		}
	} else {
		wsConfig = DefaultWebSocketConfig()
		wsConfig.Subprotocols = []string{"mcp"}
	}

	// Return a handler that creates a new message handler per connection
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a new message handler for this connection
		messageHandler := NewMCPMessageHandler(server)
		wsHandler := NewWebSocketHandler(messageHandler.HandleMessage, wsConfig)
		wsHandler.ServeHTTP(w, r)
	}
}

func CreateMCPHealthHandler(server MCPServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serverInfo := server.GetServerInfo()
		capabilities := server.GetCapabilities()

		health := map[string]interface{}{
			"status":       "healthy",
			"server_info":  serverInfo,
			"capabilities": capabilities,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(health)
	}
}
