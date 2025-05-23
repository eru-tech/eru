package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	model "github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/module_store"
	"github.com/eru-tech/eru/eru-ai/tools"
	tools_factory "github.com/eru-tech/eru/eru-ai/tools/tools_factory"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
)

func ToolListHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ToolListHandler - Start")
		toolName := "MS_EMAIL" //get it from env variable
		tool := tools_factory.GetTool(toolName)
		mcpTools := tool.GetMcpTools()
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"tools": mcpTools})
	}
}
func ModelQueryHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ModelSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		modelId := vars["model"]
		toolName := vars["tool"]
		modelFromReq := json.NewDecoder(r.Body)
		modelFromReq.DisallowUnknownFields()

		var chatMessage model.ChatRequest
		if err := modelFromReq.Decode(&chatMessage); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		modelObj, err := s.GetModel(r.Context(), projectId, tenantId, modelId, s)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		if toolName == "" {
			res, err := modelObj.QueryModel(r.Context(), chatMessage)
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(res)
		} else {
			tool, tErr := s.GetTool(r.Context(), projectId, tenantId, toolName, "", s)
			if tErr != nil {
				logs.WithContext(r.Context()).Error(tErr.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": tErr.Error()})
				return
			}
			toolMap := make(map[string]tools.Tooling)
			toolMap[toolName] = tool
			res, err := modelObj.QueryModelWithTool(r.Context(), chatMessage, toolMap, "", "")
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(res)
		}
	}
}

func AgentListNamesHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("AgentListNamesHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		tenantID := vars["tenant"]
		agents, err := s.GetAgentNames(r.Context(), projectID, tenantID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			logs.WithContext(r.Context()).Info(fmt.Sprintf("Agents: %v", agents))
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"agents": agents})
		}
	}
}

func ToolListNamesHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ToolListNamesHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		tenantID := vars["tenant"]
		tools, err := s.GetToolNames(r.Context(), projectID, tenantID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			logs.WithContext(r.Context()).Info(fmt.Sprintf("Tools: %v", tools))
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"tools": tools})
		}
	}
}
