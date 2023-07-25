package handlers

import (
	"encoding/json"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"io"
	"net/http"
)

func FuncHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("FuncHandler - Start")
		// Close the body of the request
		//TODO to add request body close in all handlers across projects
		defer r.Body.Close()

		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		vars := mux.Vars(r)
		projectId := vars["project"]
		funcName := vars["funcname"]

		// Lookup a routes in a function based on host and url

		funcGroup, err := s.GetAndValidateFunc(r.Context(), funcName, projectId, host, url, r.Method, r.Header, s)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		response, err := funcGroup.Execute(r.Context(), r, module_store.FuncThreads, module_store.LoopThreads)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		defer response.Body.Close()
		if response.StatusCode >= 300 && response.StatusCode <= 399 {
			http.Redirect(w, r, response.Header.Get("Location"), response.StatusCode)
		} else {

			for k, v := range response.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(response.StatusCode)
			_, err = io.Copy(w, response.Body)
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			return
		}
	}
}
