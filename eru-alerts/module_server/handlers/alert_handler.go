package handlers

import (
	"encoding/json"
	"github.com/eru-tech/eru/eru-alerts/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
)

func ExecuteAlertHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("inside ChannelRemoveHandler")
		vars := mux.Vars(r)
		projectId := vars["project"]
		channelName := vars["channelname"]
		messageTemplate := vars["messagetemplate"]

		log.Println(projectId)
		log.Println(channelName)
		log.Println(messageTemplate)
		mt, mtErr := s.GetMessageTemplate(projectId, messageTemplate, s)
		if mtErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": mtErr.Error()})
		}

		log.Println("mt text printed below")

		ch, err := s.GetChannel(channelName, projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		}

		response, err := ch.Execute(r, mt)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, response.StatusCode)
			_, err = io.Copy(w, response.Body)
		}
		return
	}
}
