package mcp_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	"github.com/gorilla/websocket"
)

const (
	ActionListTools    = "list_tools"
	ActionCallTool     = "call_tool"
	TransportHTTP      = "http"
	TransportWebSocket = "websocket"
	mcpProtocolVersion = "2025-11-25"
)

var requestCounter atomic.Int64

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

var mcpClientToolActions = []tools.ToolAction{
	{
		ActionName:  ActionListTools,
		Description: "List all available tools on the remote MCP server",
	},
	{
		ActionName:  ActionCallTool,
		Description: "Call a specific tool on the remote MCP server",
		GetParameters: func() eru_models.JSONSchema {
			return eru_models.JSONSchema{
				Type: "object",
				Properties: map[string]eru_models.JSONSchema{
					"tool_name": {
						Type:        "string",
						Description: "Name of the tool to call on the remote MCP server",
					},
					"arguments": {
						Type:        "object",
						Description: "Arguments to pass to the remote tool",
					},
				},
				Required: []string{"tool_name"},
			}
		},
	},
}

type MCPClientTool struct {
	tools.Tool
	ServerURL     string `json:"server_url" eru:"required"`
	TransportType string `json:"transport_type"` // "http" (default) or "websocket"
	AuthHeader    string `json:"auth_header"`    // e.g. "Authorization"
	AuthToken     string `json:"auth_token"`     // e.g. "Bearer xyz"
}

func (t *MCPClientTool) GetSpec() tools.Tooling {
	return t
}

func (t *MCPClientTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MCPClientTool MakeFromJson - Start")
	if err := json.Unmarshal(*rj, t); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (t *MCPClientTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &MCPClientTool{}
	if err := json.Unmarshal(toolObjJson, newTool); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return newTool, nil
}

func (t *MCPClientTool) GetActionsList() []string {
	actions := make([]string, 0, len(mcpClientToolActions))
	for _, a := range mcpClientToolActions {
		actions = append(actions, a.ActionName)
	}
	return actions
}

func (t *MCPClientTool) SetToolAction(actionName string) {
	for _, a := range mcpClientToolActions {
		if a.ActionName == actionName {
			t.Tool.ToolAction = a
			return
		}
	}
	t.Tool.ToolAction = tools.ToolAction{}
}

func (t *MCPClientTool) GetParameters() eru_models.JSONSchema {
	if t.Tool.ToolAction.GetParameters != nil {
		return t.Tool.ToolAction.GetParameters()
	}
	return t.Tool.Parameters
}

func (t *MCPClientTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug(fmt.Sprintf("MCPClientTool Execute - Start (action: %s)", actionName))

	transport := t.TransportType
	if transport == "" {
		transport = TransportHTTP
	}

	switch transport {
	case TransportWebSocket:
		return t.executeViaWebSocket(ctx, actionName, params)
	default:
		return t.executeViaHTTP(ctx, actionName, params)
	}
}

// --- HTTP (Streamable HTTP) transport ---

func (t *MCPClientTool) executeViaHTTP(ctx context.Context, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	client := &http.Client{Timeout: 60 * time.Second}

	sessionId, err := t.httpInitialize(ctx, client)
	if err != nil {
		return nil, false, fmt.Errorf("mcp initialize failed: %w", err)
	}

	if err := t.httpNotify(ctx, client, sessionId, "notifications/initialized", nil); err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("mcp notifications/initialized warning: %v", err))
	}

	switch actionName {
	case ActionListTools:
		return t.httpListTools(ctx, client, sessionId)
	case ActionCallTool:
		return t.httpCallTool(ctx, client, sessionId, params)
	default:
		return nil, false, fmt.Errorf("unknown action: %s", actionName)
	}
}

func (t *MCPClientTool) httpPost(ctx context.Context, client *http.Client, sessionId string, req jsonrpcRequest) (*jsonrpcResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.ServerURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if sessionId != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionId)
	}
	if t.AuthHeader != "" && t.AuthToken != "" {
		httpReq.Header.Set(t.AuthHeader, t.AuthToken)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(respBody) == 0 || string(respBody) == "{}" {
		return nil, nil
	}

	var jsonResp jsonrpcResponse
	if err := json.Unmarshal(respBody, &jsonResp); err != nil {
		return nil, fmt.Errorf("invalid JSON-RPC response: %w", err)
	}
	if jsonResp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", jsonResp.Error.Code, jsonResp.Error.Message)
	}
	return &jsonResp, nil
}

func (t *MCPClientTool) httpInitialize(ctx context.Context, client *http.Client) (string, error) {
	id := requestCounter.Add(1)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "eru-ai", "version": "1.0.0"},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.ServerURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if t.AuthHeader != "" && t.AuthToken != "" {
		httpReq.Header.Set(t.AuthHeader, t.AuthToken)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	sessionId := resp.Header.Get("Mcp-Session-Id")

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return sessionId, err
	}

	var jsonResp jsonrpcResponse
	if err := json.Unmarshal(respBody, &jsonResp); err != nil {
		return sessionId, fmt.Errorf("invalid initialize response: %w", err)
	}
	if jsonResp.Error != nil {
		return sessionId, fmt.Errorf("initialize error %d: %s", jsonResp.Error.Code, jsonResp.Error.Message)
	}
	return sessionId, nil
}

func (t *MCPClientTool) httpNotify(ctx context.Context, client *http.Client, sessionId string, method string, params interface{}) error {
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	_, err := t.httpPost(ctx, client, sessionId, req)
	return err
}

func (t *MCPClientTool) httpListTools(ctx context.Context, client *http.Client, sessionId string) (map[string]interface{}, bool, error) {
	id := requestCounter.Add(1)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	resp, err := t.httpPost(ctx, client, sessionId, req)
	if err != nil {
		return nil, false, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, false, err
	}
	return result, false, nil
}

func (t *MCPClientTool) httpCallTool(ctx context.Context, client *http.Client, sessionId string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	toolName, ok := params["tool_name"].(string)
	if !ok || toolName == "" {
		return nil, false, fmt.Errorf("tool_name is required")
	}
	arguments, _ := params["arguments"].(map[string]interface{})
	if arguments == nil {
		arguments = map[string]interface{}{}
	}

	id := requestCounter.Add(1)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	resp, err := t.httpPost(ctx, client, sessionId, req)
	if err != nil {
		return nil, false, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, false, err
	}
	return result, false, nil
}

// --- WebSocket transport ---

func (t *MCPClientTool) executeViaWebSocket(ctx context.Context, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		Subprotocols:     []string{"mcp"},
	}

	headers := http.Header{}
	if t.AuthHeader != "" && t.AuthToken != "" {
		headers.Set(t.AuthHeader, t.AuthToken)
	}

	conn, _, err := dialer.DialContext(ctx, t.ServerURL, headers)
	if err != nil {
		return nil, false, fmt.Errorf("websocket dial failed: %w", err)
	}
	defer conn.Close()

	if err := t.wsInitialize(ctx, conn); err != nil {
		return nil, false, fmt.Errorf("mcp ws initialize failed: %w", err)
	}

	switch actionName {
	case ActionListTools:
		return t.wsListTools(ctx, conn)
	case ActionCallTool:
		return t.wsCallTool(ctx, conn, params)
	default:
		return nil, false, fmt.Errorf("unknown action: %s", actionName)
	}
}

func (t *MCPClientTool) wsSend(conn *websocket.Conn, req jsonrpcRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, body)
}

func (t *MCPClientTool) wsRead(conn *websocket.Conn) (*jsonrpcResponse, error) {
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	var resp jsonrpcResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return &resp, nil
}

func (t *MCPClientTool) wsInitialize(ctx context.Context, conn *websocket.Conn) error {
	id := requestCounter.Add(1)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "eru-ai", "version": "1.0.0"},
		},
	}
	if err := t.wsSend(conn, req); err != nil {
		return err
	}
	if _, err := t.wsRead(conn); err != nil {
		return err
	}
	notify := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	return t.wsSend(conn, notify)
}

func (t *MCPClientTool) wsListTools(ctx context.Context, conn *websocket.Conn) (map[string]interface{}, bool, error) {
	id := requestCounter.Add(1)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}
	if err := t.wsSend(conn, req); err != nil {
		return nil, false, err
	}
	resp, err := t.wsRead(conn)
	if err != nil {
		return nil, false, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, false, err
	}
	return result, false, nil
}

func (t *MCPClientTool) wsCallTool(ctx context.Context, conn *websocket.Conn, params map[string]interface{}) (map[string]interface{}, bool, error) {
	toolName, ok := params["tool_name"].(string)
	if !ok || toolName == "" {
		return nil, false, fmt.Errorf("tool_name is required")
	}
	arguments, _ := params["arguments"].(map[string]interface{})
	if arguments == nil {
		arguments = map[string]interface{}{}
	}

	id := requestCounter.Add(1)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}
	if err := t.wsSend(conn, req); err != nil {
		return nil, false, err
	}
	resp, err := t.wsRead(conn)
	if err != nil {
		return nil, false, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, false, err
	}
	return result, false, nil
}
