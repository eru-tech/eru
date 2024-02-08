package handlers

import (
	"encoding/json"
	"github.com/eru-tech/eru/eru-alerts/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"io"
	"net/http"
)

func ExecuteAlertHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ExecuteAlertHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		channelName := vars["channelname"]
		messageTemplate := vars["messagetemplate"]

		mt, mtErr := s.GetMessageTemplate(r.Context(), projectId, messageTemplate, s)
		if mtErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": mtErr.Error()})
		}

		ch, err := s.GetChannel(r.Context(), channelName, projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		}

		response, err := ch.Execute(r.Context(), r, mt)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, response.StatusCode)
			_, err = io.Copy(w, response.Body)
		}
		return
	}
}
