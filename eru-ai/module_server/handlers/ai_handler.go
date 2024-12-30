package handlers

import (
	"encoding/json"
	"net/http"

	model "github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/module_store"
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
		toolId := vars["tool"]
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
		res, err := modelObj.QueryModelWithTool(r.Context(), chatMessage, toolId)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		/*
			logs.WithContext(r.Context()).Info(fmt.Sprint(llmName))
			modelObj := models.GetModel(llmName)
			logs.WithContext(r.Context()).Info(fmt.Sprint(modelObj))
			modelJson, err := json.Marshal(modelObjTmp)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}

			if err = json.Unmarshal(modelJson, &modelObj); err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			} else {
				err = utils.ValidateStruct(r.Context(), modelObj, "")
				if err != nil {
					server_handlers.FormatResponse(w, 400)
					json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
					return
				}
			}

			err = s.SaveModel(r.Context(), modelObj, projectId, tenantId, s, true)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			} else {
				server_handlers.FormatResponse(w, 200)
				modelId, anErr := modelObj.GetAttribute(r.Context(), "model_id")
				if anErr != nil {
					server_handlers.FormatResponse(w, 400)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": anErr.Error()})
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("model ", modelId, " saved successfully")})
			}
		*/
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(res)
	}
}
