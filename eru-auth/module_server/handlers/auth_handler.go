package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"log"
	"math/rand"
	"net/http"
)

func GenerateOtpHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		gatewayType := vars["gatewaytype"]
		messageType := vars["messagetype"]
		channel := vars["channel"]
		log.Println("inside GenerateOtpHandler")
		gatewayI, err := s.GetGatewayFromType(gatewayType, channel, projectId)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			gatewayName, gnerr := gatewayI.GetAttribute("GatewayName")
			if gnerr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": gnerr.Error()})
				return
			}
			mt, mterr := s.GetMessageTemplate(gatewayName.(string), projectId, messageType)
			if mterr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": mterr.Error()})
				return
			}
			//todo : to generate otp based on project setting 4 digits or 6 digits
			otp := fmt.Sprint(rand.Intn(999999-100000) + 100000)
			res, senderr := gatewayI.Send(mt.GetMessageText(otp), mt.TemplateId, r.URL.Query())
			if senderr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": senderr.Error()})
				return
			}
			server_handlers.FormatResponse(w, 200)
			//_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("OTP ", otp, " generated successfully")})
			_ = json.NewEncoder(w).Encode(res)
		}
	}
}
