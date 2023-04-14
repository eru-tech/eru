package handlers

import (
	//"bytes"
	"encoding/json"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
)

func RouteForwardHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RouteForwardHandler - Start")
		defer r.Body.Close()
		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		logs.WithContext(r.Context()).Debug(host)
		logs.WithContext(r.Context()).Debug(url)
	}
}

func RouteAsyncTestHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RouteAsyncTestHandler - Start")
		defer r.Body.Close()
		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		logs.WithContext(r.Context()).Debug(host)
		logs.WithContext(r.Context()).Debug(url)
	}
}

func RouteHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RouteHandler - Start")
		// Close the body of the request
		//defer file_utils.CloseTheCloser(request.Body)  //TODO to add request body close in all handlers across projects
		defer r.Body.Close()
		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		vars := mux.Vars(r)
		projectId := vars["project"]
		routeName := vars["routename"]

		// Lookup a route based on host and url
		route, err := s.GetAndValidateRoute(r.Context(), routeName, projectId, host, url, r.Method, r.Header)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if route.Authorizer != "" {

			authorizer, err := s.GetProjectAuthorizer(r.Context(), projectId, route.Authorizer)
			if err != nil {
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			if !route.CheckPathException(r.URL.Path) {

				token := r.Header.Get(authorizer.TokenHeaderKey)
				if token == "" {
					server_handlers.FormatResponse(w, http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized Request"})
					return
				}
				claims, err := authorizer.VerifyToken(r.Context(), r.Header.Get(authorizer.TokenHeaderKey))
				if err != nil {
					server_handlers.FormatResponse(w, http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				claimsBytes, err := json.Marshal(claims)
				if err != nil {
					logs.WithContext(r.Context()).Error(err.Error())
					server_handlers.FormatResponse(w, http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				r.Header.Add("claims", string(claimsBytes))
			}
		}
		response, _, err := route.Execute(r.Context(), r, url, false, "", nil, module_store.LoopThreads)
		if route.Redirect {
			logs.WithContext(r.Context()).Info(route.FinalRedirectUrl)
			http.Redirect(w, r, route.FinalRedirectUrl, http.StatusSeeOther)
		} else {
			if err != nil {
				server_handlers.FormatResponse(w, http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			for k, v := range response.Header {
				for _, h := range v {
					w.Header().Set(k, h)
				}
			}
			w.WriteHeader(response.StatusCode)
			_, err = io.Copy(w, response.Body)
			log.Println(err)
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
			}
		}
	}
}
