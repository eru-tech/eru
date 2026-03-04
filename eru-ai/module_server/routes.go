package module_server

import (
	"fmt"
	"net/http"

	module_handlers "github.com/eru-tech/eru/eru-ai/module_server/handlers"
	"github.com/eru-tech/eru/eru-ai/module_store"
	"github.com/eru-tech/eru/eru-server/server"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-ai"
}

func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {

	// MCP server endpoints using eru-server infrastructure
	mcpServer := NewEruAIMCPServer(sh)

	// Streamable HTTP MCP endpoint (Claude Desktop compatible)
	mcpHttpHandler := server.CreateMCPHttpHandler(mcpServer)
	serverRouter.Methods(http.MethodPost).Path("/mcp").HandlerFunc(mcpHttpHandler)
	serverRouter.Methods(http.MethodGet).Path("/mcp").HandlerFunc(mcpHttpHandler)
	serverRouter.Methods(http.MethodDelete).Path("/mcp").HandlerFunc(mcpHttpHandler)

	// WebSocket MCP endpoint
	serverRouter.Methods(http.MethodGet).Path("/mcp/websocket").HandlerFunc(
		server.CreateMCPWebSocketHandler(mcpServer),
	)

	// MCP health endpoint
	serverRouter.Methods(http.MethodGet).Path("/mcp/health").HandlerFunc(
		server.CreateMCPHealthHandler(mcpServer),
	)

	// MCP stats endpoint
	serverRouter.Methods(http.MethodGet).Path("/mcp/stats").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		tools, err := mcpServer.ListTools(ctx)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"tools_count":%d,"status":"ready","server_info":{"name":"eru-ai-mcp-server","version":"1.0.0"}}`, len(tools.Tools))))
	})

	// A2A minimal endpoints
	serverRouter.Methods(http.MethodPost).Path("/a2a/task.submit").HandlerFunc(module_handlers.A2ATaskSubmitHandler(sh))
	serverRouter.Methods(http.MethodGet).Path("/a2a/task.status").HandlerFunc(module_handlers.A2ATaskStatusHandler(sh))
	serverRouter.Methods(http.MethodGet).Path("/a2a/agent.discover").HandlerFunc(module_handlers.A2AAgentDiscoverHandler(sh))

	//store functions specific to files
	serverRouter.Methods(http.MethodGet).Path("/mcp/tools").HandlerFunc(module_handlers.McpToolListHandler(sh))
	serverRouter.Methods(http.MethodGet).Path("/tools").HandlerFunc(module_handlers.ToolListHandler(sh))
	serverRouter.Methods(http.MethodPost).Path("/{event_name}").HandlerFunc(module_handlers.ConfigSyncHandler(sh))
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()
	storeRouter.Methods(http.MethodGet).Path("/load").HandlerFunc(module_handlers.StoreLoadHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/compare").HandlerFunc(module_handlers.StoreCompareHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/{tenant}/save/model").HandlerFunc(module_handlers.ModelSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/{tenant}/remove/model/{modelname}").HandlerFunc(module_handlers.ModelRemoveHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/{tenant}/save/agent").HandlerFunc(module_handlers.AgentSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/{tenant}/remove/agent/{agentname}").HandlerFunc(module_handlers.AgentRemoveHandler(sh))

	storeRouter.Methods(http.MethodPost).Path("/{project}/{tenant}/save/vectorstore").HandlerFunc(module_handlers.VectorStoreSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/{tenant}/remove/vectorstore/{vectorstorename}").HandlerFunc(module_handlers.VectorStoreRemoveHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/{tenant}/sync/vectorstore/{vectorstorename}").HandlerFunc(module_handlers.VectorStoreSyncHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/{tenant}/save/vector/{vectorstorename}").HandlerFunc(module_handlers.VectorSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/{tenant}/remove/vector/{vectorstorename}").HandlerFunc(module_handlers.VectorRemoveHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/{tenant}/search/vector/{vectorstorename}").HandlerFunc(module_handlers.VectorSearchHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/{tenant}/list/vector/{vectorstorename}").HandlerFunc(module_handlers.VectorListHandler(sh))

	storeRouter.Methods(http.MethodPost).Path("/{project}/{tenant}/save/tool").HandlerFunc(module_handlers.ToolSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/{tenant}/remove/tool/{toolname}").HandlerFunc(module_handlers.ToolRemoveHandler(sh))

	storeRouter.Methods(http.MethodPost).Path("/{project}/settings/save").HandlerFunc(module_handlers.ProjectSettingsSaveHandler(sh))
	storeRouter.Methods(http.MethodGet).PathPrefix("/{project}/{tenant}/agent/list").HandlerFunc(module_handlers.AgentListNamesHandler(sh))
	storeRouter.Methods(http.MethodGet).PathPrefix("/{project}/agent/list").HandlerFunc(module_handlers.AgentListNamesHandler(sh))
	storeRouter.Methods(http.MethodGet).PathPrefix("/{project}/{tenant}/vectorstore/list").HandlerFunc(module_handlers.VectorStoreListNamesHandler(sh))
	storeRouter.Methods(http.MethodGet).PathPrefix("/{project}/vectorstore/list").HandlerFunc(module_handlers.VectorStoreListNamesHandler(sh))
	storeRouter.Methods(http.MethodGet).PathPrefix("/{project}/{tenant}/tool/list").HandlerFunc(module_handlers.ToolListNamesHandler(sh))
	storeRouter.Methods(http.MethodGet).PathPrefix("/{project}/tool/list").HandlerFunc(module_handlers.ToolListNamesHandler(sh))
	// functions for ai events
	aiRouter := serverRouter.PathPrefix("/{project}").Subrouter()
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/{model}/query").HandlerFunc(module_handlers.ModelQueryHandler(sh))
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/{model}/embeddings").HandlerFunc(module_handlers.ModelEmbeddingsHandler(sh))
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/{model}/{tool}/query").HandlerFunc(module_handlers.ModelQueryHandler(sh))
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/{model}/{tool}/{action}/query").HandlerFunc(module_handlers.ModelQueryHandler(sh))
	aiRouter.Methods(http.MethodPost).PathPrefix("/login/{tenant}/execute/tool/{toolname}/{actionname}").HandlerFunc(module_handlers.ToolExecuteHandler(sh))
	aiRouter.Methods(http.MethodPost).PathPrefix("/whatsapp/{tenant}/execute/tool/{toolname}/{actionname}/{endpoint}").HandlerFunc(module_handlers.ToolWhatsAppEndpointExecuteHandler(sh))
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/execute/tool/{toolname}/{actionname}").HandlerFunc(module_handlers.ToolExecuteHandler(sh))
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/execute/tool/{toolname}").HandlerFunc(module_handlers.ToolExecuteHandler(sh))
	aiRouter.Methods(http.MethodGet).PathPrefix("/{tenant}/cburl/tool/{toolname}").HandlerFunc(module_handlers.ToolCbUrlHandler(sh))
	aiRouter.PathPrefix("/callback/{tenant}/tool/{toolname}").HandlerFunc(module_handlers.ToolCallbackHandler(sh))
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/execute/agent/{agentname}/{conversationid}").HandlerFunc(module_handlers.AgentExecuteHandler(sh))
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/execute/agent/{agentname}").HandlerFunc(module_handlers.AgentExecuteHandler(sh))
	aiRouter.Methods(http.MethodGet).PathPrefix("/{tenant}/list/conversations/{agentname}/{conversationid}").HandlerFunc(module_handlers.FetchConversationHandler(sh))
	aiRouter.Methods(http.MethodGet).PathPrefix("/{tenant}/list/conversations/{agentname}").HandlerFunc(module_handlers.ListConversationsHandler(sh))

}
