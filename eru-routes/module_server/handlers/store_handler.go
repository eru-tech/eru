package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-routes/module_model"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"github.com/eru-tech/eru/eru-routes/routes"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func ProjectSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.SaveProject(projectID, s, true)
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
		vars := mux.Vars(r)
		projectId := vars["project"]
		log.Print(projectId)

		prjConfigFromReq := json.NewDecoder(r.Body)
		prjConfigFromReq.DisallowUnknownFields()

		var projectCOnfig module_model.ProjectConfig

		if err := prjConfigFromReq.Decode(&projectCOnfig); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(projectCOnfig, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveProjectConfig(projectId, projectCOnfig, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project config for ", projectId, " saved successfully")})
		}
	}
}

func ProjectRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.RemoveProject(projectID, s)
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
		//token, err := VerifyToken(r.Header.Values("Authorization")[0])
		//log.Print(token.Method)
		//log.Print(err)
		projectIds := s.GetProjectList()
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"projects": projectIds})
	}
}

func ProjectConfigHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		log.Print(projectID)
		project, err := s.GetProjectConfig(projectID)
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
		log.Print("inside RouteSaveHandler")
		vars := mux.Vars(r)
		projectId := vars["project"]
		//routeName := vars["routename"]

		//log.Println(projectId, " ", routeName)

		routeFromReq := json.NewDecoder(r.Body)
		routeFromReq.DisallowUnknownFields()

		var routeObj routes.Route
		if err := routeFromReq.Decode(&routeObj); err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(routeObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		//err := storageObj.Save(s,projectId,storageName)
		err := s.SaveRoute(routeObj, projectId, s, true)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore("", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("route config for ", routeObj.RouteName, " saved successfully")})
		}
		return
	}
}

func RouteRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("inside RouteRemoveHandler")
		vars := mux.Vars(r)
		projectId := vars["project"]
		routeName := vars["routename"]

		err := s.RemoveRoute(routeName, projectId, s)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore("", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("route config for ", routeName, " removed successfully")})
		}
		return
	}
}

func FuncSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("inside FuncSaveHandler")
		vars := mux.Vars(r)
		projectId := vars["project"]

		funcFromReq := json.NewDecoder(r.Body)
		funcFromReq.DisallowUnknownFields()

		var funcObj routes.FuncGroup
		if err := funcFromReq.Decode(&funcObj); err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(funcObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		//err := storageObj.Save(s,projectId,storageName)
		err := s.SaveFunc(funcObj, projectId, s, true)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore("", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("function config for ", funcObj.FuncGroupName, " saved successfully")})
		}
		return
	}
}

func FuncRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("inside FuncRemoveHandler")
		vars := mux.Vars(r)
		projectId := vars["project"]
		funcName := vars["funcname"]

		err := s.RemoveFunc(funcName, projectId, s)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore("", s)
			server_handlers.FormatResponse(w, 200)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("function ", funcName, " removed successfully")})
		return
	}
}
