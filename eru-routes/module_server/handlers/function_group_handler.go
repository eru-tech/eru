package handlers

import (
	"encoding/json"
	"github.com/eru-tech/eru/eru-routes/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
)

func FuncHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Close the body of the request
		//TODO to add request body close in all handlers across projects
		defer r.Body.Close()

		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		vars := mux.Vars(r)
		projectId := vars["project"]
		funcName := vars["funcname"]

		// Lookup a routes in a function based on host and url

		funcGroup, err := s.GetAndValidateFunc(funcName, projectId, host, url, r.Method, r.Header)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		log.Print(funcGroup)
		response, err := funcGroup.Execute(r)
		if err != nil {
			log.Println(" httpClient.Do error ")
			log.Println(err)
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		defer response.Body.Close()
		for k, v := range response.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(response.StatusCode)
		_, err = io.Copy(w, response.Body)
		if err != nil {
			log.Println("================")
			log.Println(err)
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		return
	}
}
