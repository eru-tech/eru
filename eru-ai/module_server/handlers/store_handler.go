package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	models "github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/module_model"
	"github.com/eru-tech/eru/eru-ai/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
)

func StoreCompareHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StoreCompareHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]

		comparePrjFromReq := json.NewDecoder(r.Body)
		comparePrjFromReq.DisallowUnknownFields()
		var compareProject module_model.ExtendedProject

		if err := comparePrjFromReq.Decode(&compareProject); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		myPrj, err := s.GetExtendedProjectConfig(r.Context(), projectID, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		storeCompare, err := myPrj.CompareProject(r.Context(), compareProject)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(storeCompare)
	}
}

func ProjectSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectSaveHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.SaveProject(r.Context(), projectID, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project ", projectID, " created successfully")})
		}
	}
}

func ProjectRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectRemoveHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.RemoveProject(r.Context(), projectID, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project ", projectID, " removed successfully")})
		}
	}
}

func ProjectListHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectListHandler - Start")
		projectIds := s.GetProjectList(r.Context())
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"projects": projectIds})
	}
}

func ProjectConfigHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectConfigHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		project, err := s.GetExtendedProjectConfig(r.Context(), projectID, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"project": project})
		}
	}
}

func ModelSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ModelSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		llmName := ""
		modelFromReq := json.NewDecoder(r.Body)
		modelFromReq.DisallowUnknownFields()

		var modelObjTmp map[string]interface{}
		if err := modelFromReq.Decode(&modelObjTmp); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			if ln, ok := modelObjTmp["llm_name"]; !ok {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "missing field in object : llm_name"})
				return
			} else {
				llmName = ln.(string)
			}
		}
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
	}
}

func ModelRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ModelRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		modelId := vars["modelid"]
		err := s.RemoveModel(r.Context(), modelId, projectId, tenantId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("model ", modelId, " removed successfully")})
		}
	}
}

func ProjectSetingsSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectSetingsSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		prjConfigFromReq := json.NewDecoder(r.Body)
		prjConfigFromReq.DisallowUnknownFields()

		var projectSettings module_model.ProjectSettings

		if err := prjConfigFromReq.Decode(&projectSettings); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			logs.WithContext(r.Context()).Info(fmt.Sprint(projectSettings))
			err := utils.ValidateStruct(r.Context(), projectSettings, "")
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveProjectSettings(r.Context(), projectId, projectSettings, s)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project settings for ", projectId, " saved successfully")})
		}
	}
}
