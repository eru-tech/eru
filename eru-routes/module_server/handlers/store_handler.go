package handlers

import (
	"encoding/json"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/module_model"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"github.com/eru-tech/eru/eru-routes/routes"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gorilla/mux"
	"net/http"
)

func StoreCompareHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StoreCompareHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]

		projectJson := json.NewDecoder(r.Body)
		projectJson.DisallowUnknownFields()
		var compareProject module_model.Project
		storeCompare := module_model.StoreCompare{}

		if err := projectJson.Decode(&compareProject); err == nil {
			myPrj, err := s.GetProjectConfig(r.Context(), projectID)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			//if !cmp.Equal(*myPrj, compareProject, cmp.AllowUnexported(module_model.Project{}), cmp.Reporter(&diffR)) {
			//log.Println(diffR.Output())
			//}

			for _, mr := range myPrj.Routes {
				var diffR utils.DiffReporter
				rFound := false
				for _, cr := range compareProject.Routes {
					if mr.RouteName == cr.RouteName {
						rFound = true
						if !cmp.Equal(mr, cr, cmpopts.IgnoreFields(routes.TargetHost{}, "Host"), cmpopts.IgnoreFields(routes.TargetHost{}, "Scheme"), cmp.Reporter(&diffR)) {
							if storeCompare.MismatchRoutes == nil {
								storeCompare.MismatchRoutes = make(map[string]interface{})
							}
							storeCompare.MismatchRoutes[mr.RouteName] = diffR.Output()
						}
						break
					}
				}
				if !rFound {
					storeCompare.DeleteRoutes = append(storeCompare.DeleteRoutes, mr.RouteName)
				}
			}

			for _, cr := range compareProject.Routes {
				rFound := false
				for _, mr := range myPrj.Routes {
					if mr.RouteName == cr.RouteName {
						rFound = true
						break
					}
				}
				if !rFound {
					storeCompare.NewRoutes = append(storeCompare.NewRoutes, cr.RouteName)
				}
			}
			/*
				//compare funcs
				for _, mf := range myPrj.FuncGroups {
					var diffR utils.DiffReporter
					fFound := false
					for _, cf := range compareProject.FuncGroups {
						if mf.FuncGroupName == cf.FuncGroupName {
							fFound = true
							if !cmp.Equal(mf, cf, cmpopts.IgnoreFields(routes.TargetHost{}, "Host"), cmpopts.IgnoreFields(routes.TargetHost{}, "Scheme"), cmp.Reporter(&diffR)) {
								if storeCompare.MismatchFuncs == nil {
									storeCompare.MismatchFuncs = make(map[string]interface{})
								}
								storeCompare.MismatchFuncs[mf.FuncGroupName] = diffR.Output()

							}
							break
						}
					}
					if !fFound {
						storeCompare.DeleteFuncs = append(storeCompare.DeleteFuncs, mf.FuncGroupName)
					}
				}

				for _, cf := range compareProject.FuncGroups {
					fFound := false
					for _, mf := range myPrj.FuncGroups {
						if mf.FuncGroupName == cf.FuncGroupName {
							fFound = true
							break
						}
					}
					if !fFound {
						storeCompare.NewFuncs = append(storeCompare.NewFuncs, cf.FuncGroupName)
					}
				}

			*/

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

func ProjectConfigSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectConfigSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		prjConfigFromReq := json.NewDecoder(r.Body)
		prjConfigFromReq.DisallowUnknownFields()

		var projectCOnfig module_model.ProjectConfig

		if err := prjConfigFromReq.Decode(&projectCOnfig); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), projectCOnfig, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveProjectConfig(r.Context(), projectId, projectCOnfig, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project config for ", projectId, " saved successfully")})
		}
	}
}

func ProjectAuthorizerSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectAuthorizerSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		prjAuthorizerFromReq := json.NewDecoder(r.Body)
		prjAuthorizerFromReq.DisallowUnknownFields()

		var prjAuthorizer routes.Authorizer

		if err := prjAuthorizerFromReq.Decode(&prjAuthorizer); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), prjAuthorizer, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveProjectAuthorizer(r.Context(), projectId, prjAuthorizer, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Project Authorizer ", prjAuthorizer.AuthorizerName, " saved successfully")})
		}
	}
}

func ProjectAuthorizerRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectAuthorizerRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authorizerName := vars["authorizername"]

		err := s.RemoveProjectAuthorizer(r.Context(), projectId, authorizerName)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			//TODO to check if save store is required here
			s.SaveStore(r.Context(), "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Project Authorizer ", authorizerName, " removed successfully")})
		}
		return
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
		project, err := s.GetProjectConfig(r.Context(), projectID)
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

		var routeObj routes.Route
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
			s.SaveStore(r.Context(), "", s)
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
			s.SaveStore(r.Context(), "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("route config for ", routeName, " removed successfully")})
		}
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

		var funcObj routes.FuncGroup
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
			s.SaveStore(r.Context(), "", s)
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
			s.SaveStore(r.Context(), "", s)
			server_handlers.FormatResponse(w, 200)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("function ", funcName, " removed successfully")})
		return
	}
}
