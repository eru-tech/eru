package handlers

import (
	"encoding/json"
	"net/http"

	model "github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/module_store"
	"github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
)

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
			tool, tErr := s.GetTool(r.Context(), projectId, tenantId, toolName, s)
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
