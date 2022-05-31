package handlers

import (
	//"bytes"
	"encoding/json"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
)

func RouteHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Header.Get("Content-Length"))
		// Close the body of the request
		//defer utils.CloseTheCloser(request.Body)  //TODO to add request body close in all handlers across projects
		defer r.Body.Close()
		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		log.Println(host)
		log.Println(url)
		vars := mux.Vars(r)
		projectId := vars["project"]
		routeName := vars["routename"]

		// Lookup a route based on host and url
		route, err := s.GetAndValidateRoute(routeName, projectId, host, url, r.Method, r.Header)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		response, _, err := route.Execute(r, url)
		if err != nil {
			log.Println(" httpClient.Do error ")
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
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
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}

}
