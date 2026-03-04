package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/google/uuid"
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
	sessionId   string
}

func NewMCPMessageHandler(server MCPServer, sessionId string) *MCPMessageHandler {
	return &MCPMessageHandler{
		server:    server,
		sessionId: sessionId,
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
	logs.WithContext(ctx).Info("MCP client initialized notification received")
	// initialized is a client notification — no id, no response required
	return nil, nil
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
	result, err := h.server.CallTool(ctx, h.sessionId, params)
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

// MCPSessionManager manages per-session MCPMessageHandler instances for HTTP transport.
type MCPSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*MCPMessageHandler
	server   MCPServer
}

func NewMCPSessionManager(server MCPServer) *MCPSessionManager {
	return &MCPSessionManager{
		sessions: make(map[string]*MCPMessageHandler),
		server:   server,
	}
}

func (m *MCPSessionManager) GetOrCreate(sessionId string) *MCPMessageHandler {
	m.mu.Lock()
	defer m.mu.Unlock()
	if h, ok := m.sessions[sessionId]; ok {
		return h
	}
	h := NewMCPMessageHandler(m.server, sessionId)
	m.sessions[sessionId] = h
	return h
}

func (m *MCPSessionManager) Delete(sessionId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionId)
}

// CreateMCPHttpHandler creates an HTTP handler for the MCP Streamable HTTP transport.
// Claude Desktop connects to this via POST /mcp (JSON-RPC) and GET /mcp (SSE for server notifications).
func CreateMCPHttpHandler(server MCPServer) http.HandlerFunc {
	manager := NewMCPSessionManager(server)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		sessionId := r.Header.Get("Mcp-Session-Id")

		switch r.Method {
		case http.MethodPost:
			if sessionId == "" {
				sessionId = uuid.New().String()
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			handler := manager.GetOrCreate(sessionId)
			response, err := handler.HandleMessage(ctx, body)
			if err != nil {
				logs.WithContext(ctx).Error("MCP HTTP handler error: " + err.Error())
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Mcp-Session-Id", sessionId)

			if response == nil {
				// Notification accepted — no body
				w.WriteHeader(http.StatusAccepted)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response)

		case http.MethodGet:
			// SSE stream for server-to-client notifications
			if sessionId == "" {
				sessionId = uuid.New().String()
			}
			manager.GetOrCreate(sessionId)

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Mcp-Session-Id", sessionId)
			w.WriteHeader(http.StatusOK)

			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			// Hold open until client disconnects
			<-ctx.Done()

		case http.MethodDelete:
			if sessionId != "" {
				manager.Delete(sessionId)
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
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

	return func(w http.ResponseWriter, r *http.Request) {
		sessionId := uuid.New().String()
		messageHandler := NewMCPMessageHandler(server, sessionId)
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
