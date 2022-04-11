package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/gateway"
	"github.com/eru-tech/eru/eru-auth/module_model"
	"github.com/eru-tech/eru/eru-auth/module_store"
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
func SmsGatewaySaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		gatewayName := vars["gatewayname"]
		sgFromReq := json.NewDecoder(r.Body)
		sgFromReq.DisallowUnknownFields()

		var smsGateway module_model.SmsGateway

		if err := sgFromReq.Decode(&smsGateway); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(smsGateway, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveSmsGateway(projectId, gatewayName, smsGateway, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Sms Gateway ", gatewayName, " created successfully")})
		}
	}
}

func SmsGatewayRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		gatewayName := vars["gatewayname"]

		err := s.RemoveSmsGateway(projectId, gatewayName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Sms Gateway ", gatewayName, " removed successfully")})
		}
	}
}

func EmailGatewaySaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		gatewayName := vars["gatewayname"]
		egFromReq := json.NewDecoder(r.Body)
		egFromReq.DisallowUnknownFields()

		var emailGateway module_model.EmailGateway

		if err := egFromReq.Decode(&emailGateway); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(emailGateway, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveEmailGateway(projectId, gatewayName, emailGateway, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Email Gateway ", gatewayName, " created successfully")})
		}
	}
}

func EmailGatewayRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		gatewayName := vars["gatewayname"]

		err := s.RemoveEmailGateway(projectId, gatewayName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Email Gateway ", gatewayName, " removed successfully")})
		}
	}
}

func MessageTemplateSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			err := utils.ValidateStruct(messageTemplate, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveMessageTemplate(projectId, messageTemplate, s)
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
		vars := mux.Vars(r)
		projectId := vars["project"]
		templateName := vars["templatename"]

		err := s.RemoveMessageTemplate(projectId, templateName, s)
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
		log.Print("inside GatewaySaveHandler")
		vars := mux.Vars(r)
		projectId := vars["project"]
		//gatewayName := vars["gatewayname"]
		gatewayType := vars["gatewaytype"]

		//log.Println(projectId, " ", gatewayName, " ", gatewayType, " ")

		gatewayFromReq := json.NewDecoder(r.Body)
		gatewayFromReq.DisallowUnknownFields()
		//t := new(map[string]string)
		//if err1 := storageFromReq.Decode(t); err1 != nil {
		//log.Println("error " , err1)
		//}
		//log.Println(t)
		gatewayObj := gateway.GetGateway(gatewayType)
		//storageObj := new(storage.Storage)
		if err := gatewayFromReq.Decode(&gatewayObj); err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			//err := utils.ValidateStruct(storageObj, "") //TODO to uncomment this code and validate the incoming json
			//if err != nil {
			//	server_handlers.FormatResponse(w, 400)
			//	json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
			//	return
			//}
		}
		//err := storageObj.Save(s,projectId,storageName)
		gatewayName, _ := gatewayObj.GetAttribute("GatewayName")
		err := s.SaveGateway(gatewayObj, projectId, s, true)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore("", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("gateway config for ", gatewayName.(string), " saved successfully")})
		}
		return
	}
}

func GatewayRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("inside GatewayRemoveHandler")
		vars := mux.Vars(r)
		projectId := vars["project"]
		gatewayName := vars["gatewayname"]
		gatewayType := vars["gatewaytype"]
		channel := vars["channel"]
		err := s.RemoveGateway(gatewayName, gatewayType, channel, projectId, s)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore("", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("gateway config for ", gatewayName, " removed successfully")})
		}
		return
	}
}
