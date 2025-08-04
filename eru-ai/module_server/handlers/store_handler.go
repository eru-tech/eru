package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	agents_factory "github.com/eru-tech/eru/eru-ai/agents/agents_factory"
	models "github.com/eru-tech/eru/eru-ai/models"
	"github.com/eru-tech/eru/eru-ai/module_model"
	"github.com/eru-tech/eru/eru-ai/module_store"
	tools_factory "github.com/eru-tech/eru/eru-ai/tools/tools_factory"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	vectorstore "github.com/eru-tech/eru/eru-vectorstore/vectorstore"
	"github.com/gorilla/mux"
)

const StoreTableName = "eruai_config"
const StoreTenantTableName = "eruai_tenant_config"

func StoreLoadHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("StoreLoadHandler - Start")

		// Load new store from DB
		newStore, err := module_store.LoadStore(StoreTableName, StoreTenantTableName)
		if err != nil {
			logs.WithContext(r.Context()).Error(fmt.Sprintf("Failed to load store: %v", err))
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		// Update the global StoreHolder with the new store
		sh.Store = newStore

		logs.WithContext(r.Context()).Info("Store loaded and replaced successfully")
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(newStore)
	}
}

func StoreCompareHandler(sh *module_store.StoreHolder) http.HandlerFunc {
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

		myPrj, err := sh.Store.GetExtendedProjectConfig(r.Context(), projectID, sh.Store)
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

func ProjectSaveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("ProjectSaveHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := sh.Store.SaveProject(r.Context(), projectID, sh.Store, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {

			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project ", projectID, " created successfully")})
		}
	}
}

func ProjectRemoveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("ProjectRemoveHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := sh.Store.RemoveProject(r.Context(), projectID, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project ", projectID, " removed successfully")})
		}
	}
}

func ProjectListHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logs.WithContext(r.Context()).Debug("ProjectListHandler - Start")
		projectIds := sh.Store.GetProjectList(r.Context())
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"projects": projectIds})
	}
}

func ProjectConfigHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logs.WithContext(r.Context()).Debug("ProjectConfigHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		project, err := sh.Store.GetExtendedProjectConfig(r.Context(), projectID, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"project": project})
		}
	}
}

func ModelSaveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("ModelSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		provider := ""
		modelFromReq := json.NewDecoder(r.Body)
		modelFromReq.DisallowUnknownFields()

		var modelObjTmp map[string]interface{}
		if err := modelFromReq.Decode(&modelObjTmp); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			if providerI, ok := modelObjTmp["provider"]; !ok {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "missing field in object : llm_name"})
				return
			} else {
				provider = providerI.(string)
			}
		}
		logs.WithContext(r.Context()).Info(fmt.Sprint(provider))
		modelObj := models.GetModel(provider)
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

		err = sh.Store.SaveModel(r.Context(), modelObj, projectId, tenantId, sh.Store, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			modelName, anErr := modelObj.GetAttribute(r.Context(), "model_name")
			if anErr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": anErr.Error()})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("model ", modelName, " saved successfully")})
		}
	}
}

func ModelRemoveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("ModelRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		modelName := vars["modelname"]
		err := sh.Store.RemoveModel(r.Context(), modelName, projectId, tenantId, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			sh.Store.SaveStore(r.Context(), projectId, "", sh.Store)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("model ", modelName, " removed successfully")})
		}
	}
}

func AgentSaveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("AgentSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		agentType := ""
		agentFromReq := json.NewDecoder(r.Body)
		agentFromReq.DisallowUnknownFields()

		var agentObjTmp map[string]interface{}
		if err := agentFromReq.Decode(&agentObjTmp); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			if agentTypeI, ok := agentObjTmp["agent_type"]; !ok {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "missing field in object : agent_type"})
				return
			} else {
				agentType = agentTypeI.(string)
			}
		}
		agentObj := agents_factory.GetAgent(agentType)
		agentJson, err := json.Marshal(agentObjTmp)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		if err = json.Unmarshal(agentJson, &agentObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err = utils.ValidateStruct(r.Context(), agentObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err = sh.Store.SaveAgent(r.Context(), agentObj, projectId, tenantId, sh.Store, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			agentName, anErr := agentObj.GetAttribute(r.Context(), "agent_name")
			if anErr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": anErr.Error()})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("agent ", agentName, " saved successfully")})
		}
	}
}

func AgentRemoveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("AgentRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		agentName := vars["agentname"]
		err := sh.Store.RemoveAgent(r.Context(), agentName, projectId, tenantId, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			sh.Store.SaveStore(r.Context(), projectId, "", sh.Store)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("agent ", agentName, " removed successfully")})
		}
	}
}

func VectorStoreSaveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("VectorStoreSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		storeType := ""
		vectorStoreFromReq := json.NewDecoder(r.Body)
		vectorStoreFromReq.DisallowUnknownFields()

		var vectorStoreObjTmp map[string]interface{}
		if err := vectorStoreFromReq.Decode(&vectorStoreObjTmp); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			if storeTypeI, ok := vectorStoreObjTmp["vector_type"]; !ok {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "missing field in object : store_type"})
				return
			} else {
				storeType = storeTypeI.(string)
			}
		}
		vectorStoreObj := vectorstore.GetVectorStore(storeType)
		vectorStoreJson, err := json.Marshal(vectorStoreObjTmp)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		if err = json.Unmarshal(vectorStoreJson, &vectorStoreObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err = utils.ValidateStruct(r.Context(), vectorStoreObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err = sh.Store.SaveVectorStore(r.Context(), vectorStoreObj, projectId, tenantId, sh.Store, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			vectorStoreName := vectorStoreObj.GetAttribute(r.Context(), "vector_name")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("vectorstore ", vectorStoreName, " saved successfully")})
		}
	}
}

func VectorStoreRemoveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("VectorStoreRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		vectorStoreName := vars["vectorstorename"]
		err := sh.Store.RemoveVectorStore(r.Context(), vectorStoreName, projectId, tenantId, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			sh.Store.SaveStore(r.Context(), projectId, "", sh.Store)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("vectorstore ", vectorStoreName, " removed successfully")})
		}
	}
}

func ToolSaveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("ToolSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		toolType := ""
		toolFromReq := json.NewDecoder(r.Body)
		toolFromReq.DisallowUnknownFields()

		var toolObjTmp map[string]interface{}
		if err := toolFromReq.Decode(&toolObjTmp); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			if toolTypeI, ok := toolObjTmp["tool_type"]; !ok {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "missing field in object : tool_type"})
				return
			} else {
				toolType = toolTypeI.(string)
			}
		}
		logs.WithContext(r.Context()).Info(fmt.Sprint(toolType))
		toolObj := tools_factory.GetTool(toolType)
		logs.WithContext(r.Context()).Info(fmt.Sprint(toolObj))
		toolJson, err := json.Marshal(toolObjTmp)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		if err = json.Unmarshal(toolJson, &toolObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err = utils.ValidateStruct(r.Context(), toolObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err = sh.Store.SaveTool(r.Context(), toolObj, projectId, tenantId, sh.Store, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			toolName, _ := toolObj.GetAttribute(r.Context(), "tool_name")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("tool ", toolName, " saved successfully")})
		}
	}
}

func ToolRemoveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

		logs.WithContext(r.Context()).Debug("ToolRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		toolName := vars["toolname"]
		err := sh.Store.RemoveTool(r.Context(), toolName, projectId, tenantId, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("tool ", toolName, " removed successfully")})
		}
	}
}

func AgentExecuteHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logs.WithContext(r.Context()).Debug("AgentExecuteHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		agentName := vars["agentname"]
		agent, err := sh.Store.GetAgent(r.Context(), projectId, tenantId, agentName, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			logs.WithContext(r.Context()).Info(fmt.Sprintf("Agent: %v", agent))
			agentParamsFromReq := json.NewDecoder(r.Body)
			agentParamsFromReq.DisallowUnknownFields()

			var agentMessage agents.AgentMessage
			if err := agentParamsFromReq.Decode(&agentMessage); err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			logs.WithContext(r.Context()).Info(fmt.Sprintf("AgentMessage: %v", agentMessage))
			agentResult, err := agent.Execute(r.Context(), agentMessage)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			} else {
				logs.WithContext(r.Context()).Info(fmt.Sprintf("AgentResult: %v", agentResult))
				server_handlers.FormatResponse(w, 200)
				_ = json.NewEncoder(w).Encode(agentResult)
			}
		}
	}
}

func ToolCallbackHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logs.WithContext(r.Context()).Debug("ToolCallbackHandler - Start")
		ctx := context.WithValue(r.Context(), "Erufuncbaseurl", module_store.Erufuncbaseurl)
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		toolName := vars["toolname"]
		actionName := "callback"

		tool, err := sh.Store.GetTool(ctx, projectId, tenantId, toolName, actionName, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			toolBodyFromReq := json.NewDecoder(r.Body)
			toolBodyFromReq.DisallowUnknownFields()

			var toolBody map[string]interface{}
			if err := toolBodyFromReq.Decode(&toolBody); err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				//server_handlers.FormatResponse(w, 400)
				//json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				//return
				toolBody = make(map[string]interface{})
			}

			params := r.URL.Query()
			toolParams := make(map[string][]string)
			for key, values := range params {
				toolParams[key] = values
			}

			toolResult, _, err := tool.Callback(ctx, projectId, tenantId, actionName, toolBody, toolParams)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			} else {
				responseContentType := tool.GetToolCallback().ResponseContentType
				w.Header().Set("Content-Type", responseContentType)
				w.WriteHeader(http.StatusOK)
				if responseContentType == "application/json" {
					_ = json.NewEncoder(w).Encode(toolResult)
				} else {
					w.Write([]byte(toolResult.(string)))
				}
			}
		}
	}
}

func ToolCbUrlHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logs.WithContext(r.Context()).Debug("ToolCbUrlHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		toolName := vars["toolname"]
		actionName := "callback"
		tool, err := sh.Store.GetTool(r.Context(), projectId, tenantId, toolName, actionName, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"url": tool.GetToolCbUrl(projectId, tenantId)})
	}
}

func ToolExecuteHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		logs.WithContext(r.Context()).Debug("ToolExecuteHandler - Start")
		ctx := context.WithValue(r.Context(), "eruauthbaseurl", module_store.Eruauthbaseurl)
		ctx = context.WithValue(ctx, "eruaiport", module_store.Eruaiport)
		vars := mux.Vars(r)
		projectId := vars["project"]
		tenantId := vars["tenant"]
		toolName := vars["toolname"]
		actionName := vars["actionname"]
		logs.WithContext(r.Context()).Info(fmt.Sprintf("Tool Name: %v", toolName))
		logs.WithContext(r.Context()).Info(fmt.Sprintf("Action Name: %v", actionName))
		tool, err := sh.Store.GetTool(r.Context(), projectId, tenantId, toolName, actionName, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			toolParamsFromReq := json.NewDecoder(r.Body)
			toolParamsFromReq.DisallowUnknownFields()

			type tParams struct {
				Params map[string]interface{} `json:"params"`
			}

			var toolParams tParams
			if err := toolParamsFromReq.Decode(&toolParams); err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			if toolParams.Params == nil {
				toolParams.Params = make(map[string]interface{})
			}
			queryParams := r.URL.Query()
			for key, values := range queryParams {
				if len(values) > 0 {
					toolParams.Params[key] = values[0]
				}
			}
			toolResult, persistStore, err := tool.Execute(ctx, projectId, tenantId, actionName, toolParams.Params)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			} else {
				if persistStore {
					err = sh.Store.SaveTool(r.Context(), tool, projectId, tenantId, sh.Store, true)
					if err != nil {
						logs.WithContext(r.Context()).Error(err.Error())
						server_handlers.FormatResponse(w, 400)
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
					}
				}
				server_handlers.FormatResponse(w, 200)
				_ = json.NewEncoder(w).Encode(toolResult)
			}
		}
	}
}

func ProjectSettingsSaveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()

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

		err := sh.Store.SaveProjectSettings(r.Context(), projectId, projectSettings, sh.Store)
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
