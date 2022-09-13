package handlers

import (
	//"bytes"
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
)

func RouteForwardHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		log.Println(host)
		log.Println(url)
	}
}

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

		if route.Authorizer != "" {

			authorizer, err := s.GetProjectAuthorizer(projectId, route.Authorizer)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			log.Println("r.URL.Path   = ", r.URL.Path)
			if !route.CheckPathException(r.URL.Path) {

				token := r.Header.Get(authorizer.TokenHeaderKey)
				if token == "" {
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized Request"})
					return
				}
				log.Println("token  == ", token)
				claims, err := authorizer.VerifyToken(r.Header.Get(authorizer.TokenHeaderKey))
				if err != nil {
					log.Println(err)
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				claimsBytes, err := json.Marshal(claims)
				if err != nil {
					log.Println(err)
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				r.Header.Add("claims", string(claimsBytes))
			}
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
		//server_handlers.FormatResponse(w, response.StatusCode)
		log.Print("before range response.Header")

		if route.Redirect {
			log.Print("before redirect")
			paramStr := ""
			for _, v := range route.RedirectParams {
				if paramStr == "" {
					paramStr = "?"
				} else {
					paramStr = fmt.Sprint(paramStr, "&")
				}
				log.Print(v.Key, " = ", v.Value)
				paramStr = fmt.Sprint(paramStr, v.Key, "=", v.Value)
			}
			log.Print(route.RedirectUrl)
			http.Redirect(w, r, fmt.Sprint(route.RedirectScheme, "://", route.RedirectUrl, paramStr), http.StatusSeeOther)
			log.Print(w.Header())
			log.Print(w)
			log.Print("after redirect")
		} else {
			for k, v := range response.Header {
				for _, h := range v {
					w.Header().Set(k, h)
				}
			}
			log.Print("inside else route.Redirect = false")
			w.WriteHeader(response.StatusCode)
			_, err = io.Copy(w, response.Body)
			if err != nil {
				log.Println("================")
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			log.Print(w.Header())
		}
		return
	}

}
