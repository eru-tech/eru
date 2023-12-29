package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-gateway/module_model"
	"github.com/eru-tech/eru/eru-gateway/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"net/http"
)

func FetchVarsHandler(s module_store.ModuleStoreI) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		service := vars["service"]
		if service == "gateway" {

		}
	}
}

func StoreCompareHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StoreCompareHandler - Start")

		projectJson := json.NewDecoder(r.Body)
		projectJson.DisallowUnknownFields()
		var compareListenerRules []module_model.ListenerRule
		storeCompare := module_model.StoreCompare{}

		if err := projectJson.Decode(&compareListenerRules); err == nil {
			storeCompare, err = s.CompareListenerRules(r.Context(), compareListenerRules)

		} else {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(storeCompare)

	}
}
func SaveListenerRuleHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveListenerRuleHandler - Start")
		lrFromReq := json.NewDecoder(r.Body)
		lrFromReq.DisallowUnknownFields()

		lrObj := module_model.ListenerRule{}
		if err := lrFromReq.Decode(&lrObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), lrObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveListenerRule(r.Context(), &lrObj, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Listener Rule ", lrObj.RuleName, " created successfully")})
		}
	}
}

func RemoveListenerRuleHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RemoveListenerRuleHandler - Start")
		vars := mux.Vars(r)
		listenerRuleName := vars["listenerrulename"]
		err := s.RemoveListenerRule(r.Context(), listenerRuleName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Listener Rule  ", listenerRuleName, " removed successfully")})
		}
	}
}

func GetListenerRulesHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GetListenerRulesHandler - Start")
		listenerRules := s.GetListenerRules(r.Context())
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"listener_rules": listenerRules})
	}
}

func SaveAuthorizerHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveAuthorizerHandler - Start")
		authFromReq := json.NewDecoder(r.Body)
		authFromReq.DisallowUnknownFields()

		authObj := module_model.Authorizer{}
		if err := authFromReq.Decode(&authObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), authObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveAuthorizer(r.Context(), authObj, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Authorizer ", authObj.AuthorizerName, " created successfully")})
		}
	}
}

func RemoveAuthorizerHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RemoveAuthorizerHandler - Start")
		vars := mux.Vars(r)
		authorizerName := vars["authorizername"]
		err := s.RemoveAuthorizer(r.Context(), authorizerName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Authorizer  ", authorizerName, " removed successfully")})
		}
	}
}

func GetAuthorizerHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GetAuthorizerHandler - Start")
		authorizers := s.GetAuthorizers(r.Context())
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"authorizers": authorizers})
	}
}

func GetProjectSetingsHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GetProjectSetingsHandler - Start")
		project_settings := s.GetProjectSettings(r.Context())
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"project_settings": project_settings})
	}
}

func ProjectSetingsSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectSetingsSaveHandler - Start")

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

		err := s.SaveProjectSettings(r.Context(), projectSettings, s, true)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("gateway settings saved successfully")})
		}
	}
}
