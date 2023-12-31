package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/gateway"
	"github.com/eru-tech/eru/eru-auth/module_model"
	"github.com/eru-tech/eru/eru-auth/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	gomail "gopkg.in/gomail.v2"
	"net/http"
	"os"
	"strings"
)

func StoreCompareHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StoreCompareHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]

		projectJson := json.NewDecoder(r.Body)
		projectJson.DisallowUnknownFields()
		//var compareProject module_model.Project
		var postBody interface{}
		storeCompareMap := make(map[string]map[string]interface{})
		storeCompare := module_model.StoreCompare{}

		if err := projectJson.Decode(&postBody); err == nil {
			storeCompareMap["projects"] = make(map[string]interface{})
			storeCompareMap["projects"][projectID] = postBody
			postBodyBytes, pbbErr := json.Marshal(storeCompareMap)
			if pbbErr != nil {
				logs.Logger.Error(pbbErr.Error())
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": pbbErr.Error()})
				return
			}
			compareStore := module_store.GetStore(strings.ToUpper(os.Getenv("STORE_TYPE")))
			umErr := module_store.UnMarshalStore(r.Context(), postBodyBytes, compareStore)
			if umErr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": umErr.Error()})
				return
			}
			compareProject, cpErr := compareStore.GetProjectConfig(r.Context(), projectID)
			if cpErr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": cpErr.Error()})
				return
			}
			myProject, mpErr := s.GetProjectConfig(r.Context(), projectID)
			if mpErr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": cpErr.Error()})
				return
			}
			storeCompare, err = myProject.CompareProject(r.Context(), *compareProject)

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

func MessageTemplateSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("MessageTemplateSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		mtFromReq := json.NewDecoder(r.Body)
		mtFromReq.DisallowUnknownFields()

		var messageTemplate module_model.MessageTemplate

		if err := mtFromReq.Decode(&messageTemplate); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), messageTemplate, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveMessageTemplate(r.Context(), projectId, messageTemplate, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Message Template ", fmt.Sprint(messageTemplate.GatewayName, "_", messageTemplate.TemplateType), " created successfully")})
		}
	}
}

func MessageTemplateRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("MessageTemplateRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		templateName := vars["templatename"]

		err := s.RemoveMessageTemplate(r.Context(), projectId, templateName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Message Template ", templateName, " removed successfully")})
		}
	}
}

func GatewaySaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GatewaySaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		//gatewayName := vars["gatewayname"]
		gatewayType := vars["gatewaytype"]

		gatewayFromReq := json.NewDecoder(r.Body)
		gatewayFromReq.DisallowUnknownFields()
		gatewayObj := gateway.GetGateway(gatewayType)
		if err := gatewayFromReq.Decode(&gatewayObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			//err := file_utils.ValidateStruct(storageObj, "") //TODO to uncomment this code and validate the incoming json
			//if err != nil {
			//	server_handlers.FormatResponse(w, 400)
			//	json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
			//	return
			//}
		}
		//err := storageObj.Save(s,projectId,storageName)
		gatewayName, _ := gatewayObj.GetAttribute("Ggateway_name")
		err := s.SaveGateway(r.Context(), gatewayObj, projectId, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("gateway config for ", gatewayName.(string), " saved successfully")})
		}
		return
	}
}

func GatewayRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GatewayRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		gatewayName := vars["gatewayname"]
		gatewayType := vars["gatewaytype"]
		channel := vars["channel"]
		err := s.RemoveGateway(r.Context(), gatewayName, gatewayType, channel, projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("gateway config for ", gatewayName, " removed successfully")})
		}
		return
	}
}

func AuthSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("AuthSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authType := ""

		authFromReq := json.NewDecoder(r.Body)
		authFromReq.DisallowUnknownFields()

		var authObjTmp map[string]interface{}
		if err := authFromReq.Decode(&authObjTmp); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			if at, ok := authObjTmp["AuthType"]; !ok {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : AuthType")})
				return
			} else {
				authType = at.(string)
			}
		}
		authObj := auth.GetAuth(authType)

		authJson, err := json.Marshal(authObjTmp)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		if err = json.Unmarshal(authJson, &authObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err = utils.ValidateStruct(r.Context(), authObj, "")
			//TODO to uncomment this code and validate the incoming json
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err = s.SaveAuth(r.Context(), authObj, projectId, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
			authName, anErr := authObj.GetAttribute(r.Context(), "auth_name")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": anErr.Error()})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("auth config ", authName, " saved successfully")})
		}
		return
	}
}
func TestEmail(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("TestEmail - Start")
		msg := gomail.NewMessage()
		//msg.SetHeader("From", "altaf@smartvalues.co.in")
		//msg.SetHeader("From", "altaf@eru-tech.com")
		msg.SetHeader("From", "altaf.baradia@stayvista.com")
		//msg.SetHeader("From", "altaf.baradia@artfine.in")
		msg.SetHeader("To", "abaradia@gmail.com")
		msg.SetHeader("Subject", "Hi")
		msg.SetBody("text/html", "<b>This is the body of the mail</b>")
		//msg.Attach("/home/User/cat.jpg")

		//n := gomail.NewDialer("smtp.office365.com", 587, "altaf@smartvalues.co.in", "Smart@123")
		//n := gomail.NewDialer("smtp.gmail.com", 587, "altaf.baradia@artfine.in", "Artfine@123")
		n := gomail.NewDialer("hmail.smartvalues.co.in", 587, "info@hmail.smartvalues.co.in", "Info@123")

		// Send the email
		if err := n.DialAndSend(msg); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": "Email Sent Successfully!"})

	}
}
func AuthRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("AuthRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]
		err := s.RemoveAuth(r.Context(), authName, projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), projectId, "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("auth config ", authName, " removed successfully")})
		}
		return
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
