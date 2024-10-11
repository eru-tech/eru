package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-functions/functions"
	"github.com/eru-tech/eru/eru-functions/module_model"
	"github.com/eru-tech/eru/eru-functions/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"net/http"
)

func StoreCompareHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StoreCompareHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		projectJson := json.NewDecoder(r.Body)
		projectJson.DisallowUnknownFields()
		var compareProject module_model.ExtendedProject
		storeCompare := module_model.StoreCompare{}

		if err := projectJson.Decode(&compareProject); err == nil {
			myPrj, err := s.GetExtendedProjectConfig(r.Context(), projectId, s)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			storeCompare, err = myPrj.CompareProject(r.Context(), compareProject)

		} else {
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
		//}
		//	vars := mux.Vars(r)
		//	projectId := vars["project"]
		//
		//	prjConfigFromReq := json.NewDecoder(r.Body)
		//	prjConfigFromReq.DisallowUnknownFields()
		//
		//	var projectCOnfig module_model.ProjectConfig
		//
		//	if err := prjConfigFromReq.Decode(&projectCOnfig); err != nil {
		//		logs.WithContext(r.Context()).Error(err.Error())
		//		server_handlers.FormatResponse(w, 400)
		//		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		//		return
		//	} else {
		//		err := utils.ValidateStruct(r.Context(), projectCOnfig, "")
		//		if err != nil {
		//			server_handlers.FormatResponse(w, 400)
		//			json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
		//			return
		//		}
		//	}
		//
		//	err := s.SaveProjectConfig(r.Context(), projectId, projectCOnfig, s)
		//	if err != nil {
		//		server_handlers.FormatResponse(w, 400)
		//		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		//	} else {
		//		server_handlers.FormatResponse(w, 200)
		//		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project config for ", projectId, " saved successfully")})
		//	}
	}
}

//func ProjectAuthorizerSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		logs.WithContext(r.Context()).Debug("ProjectAuthorizerSaveHandler - Start")
//		vars := mux.Vars(r)
//		projectId := vars["project"]
//
//		prjAuthorizerFromReq := json.NewDecoder(r.Body)
//		prjAuthorizerFromReq.DisallowUnknownFields()
//
//		var prjAuthorizer functions.Authorizer
//
//		if err := prjAuthorizerFromReq.Decode(&prjAuthorizer); err != nil {
//			logs.WithContext(r.Context()).Error(err.Error())
//			server_handlers.FormatResponse(w, 400)
//			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
//			return
//		} else {
//			err := utils.ValidateStruct(r.Context(), prjAuthorizer, "")
//			if err != nil {
//				server_handlers.FormatResponse(w, 400)
//				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
//				return
//			}
//		}
//
//		err := s.SaveProjectAuthorizer(r.Context(), projectId, prjAuthorizer, s)
//		if err != nil {
//			server_handlers.FormatResponse(w, 400)
//			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
//		} else {
//			server_handlers.FormatResponse(w, 200)
//			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Project Authorizer ", prjAuthorizer.AuthorizerName, " saved successfully")})
//		}
//	}
//}
//
//func ProjectAuthorizerRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		logs.WithContext(r.Context()).Debug("ProjectAuthorizerRemoveHandler - Start")
//		vars := mux.Vars(r)
//		projectId := vars["project"]
//		authorizerName := vars["authorizername"]
//
//		err := s.RemoveProjectAuthorizer(r.Context(), projectId, authorizerName)
//		if err != nil {
//			server_handlers.FormatResponse(w, 400)
//			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
//		} else {
//			//TODO to check if save store is required here
//			s.SaveStore(r.Context(), projectId,"", s)
//			server_handlers.FormatResponse(w, 200)
//			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Project Authorizer ", authorizerName, " removed successfully")})
//		}
//		return
//	}
//}

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
		projectId := vars["project"]
		project, err := s.GetExtendedProjectConfig(r.Context(), projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"project": project})
		}
	}
}

func RouteSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RouteSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		routeFromReq := json.NewDecoder(r.Body)
		routeFromReq.DisallowUnknownFields()

		var routeObj functions.Route
		if err := routeFromReq.Decode(&routeObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), routeObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		//err := storageObj.Save(s,projectId,storageName)
		err := s.SaveRoute(r.Context(), routeObj, projectId, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("route config for ", routeObj.RouteName, " saved successfully")})
		}
		return
	}
}

func RouteRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RouteRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		routeName := vars["routename"]

		err := s.RemoveRoute(r.Context(), routeName, projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("route config for ", routeName, " removed successfully")})
		}
		return
	}
}

func FuncValidateHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("FuncValidateHandler - Start")

		funcFromReq := json.NewDecoder(r.Body)
		funcFromReq.DisallowUnknownFields()

		var funcObj functions.FuncGroup
		if err := funcFromReq.Decode(&funcObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), funcObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("function config for ", funcObj.FuncGroupName, " validated successfully")})
		return
	}
}
func FuncSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("FuncSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		funcFromReq := json.NewDecoder(r.Body)
		funcFromReq.DisallowUnknownFields()

		var funcObj functions.FuncGroup
		if err := funcFromReq.Decode(&funcObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), funcObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		//err := storageObj.Save(s,projectId,storageName)
		err := s.SaveFunc(r.Context(), funcObj, projectId, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("function config for ", funcObj.FuncGroupName, " saved successfully")})
		}
		return
	}
}

func FuncRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("FuncRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		funcName := vars["funcname"]

		err := s.RemoveFunc(r.Context(), funcName, projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("function ", funcName, " removed successfully")})
		return
	}
}

func ProjectMyQueryListNamesHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectMyQueryListNamesHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]

		reqHeader := http.Header{}
		res, _, _, _, err := utils.CallHttp(r.Context(), http.MethodGet, fmt.Sprint(module_store.Eruqlbaseurl, "/store/", projectID, "/myquery/list"), reqHeader, nil, nil, nil, nil)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(res)
		}

		return
	}
}

func ProjectFunctionListHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectFunctionListHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]

		myqueries, err := s.GetFunctionNames(r.Context(), projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"functions": myqueries})
		}
		return
	}
}

func WfSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("WfSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		wfFromReq := json.NewDecoder(r.Body)
		wfFromReq.DisallowUnknownFields()

		var wfObj functions.Workflow
		if err := wfFromReq.Decode(&wfObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), wfObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		//err := storageObj.Save(s,projectId,storageName)
		err := s.SaveWf(r.Context(), wfObj, projectId, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("workflow config for ", wfObj.WfName, " saved successfully")})
		}
		return
	}
}

func WfRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("WfRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		wfName := vars["wfname"]

		err := s.RemoveWf(r.Context(), wfName, projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("workflow ", wfName, " removed successfully")})
		return
	}
}
