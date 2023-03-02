package handlers

import (
	//"bytes"
	"encoding/json"
	"github.com/eru-tech/eru/eru-routes/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
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

func RouteAsyncTestHandler(s module_store.ModuleStoreI) http.HandlerFunc {
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
		//defer file_utils.CloseTheCloser(request.Body)  //TODO to add request body close in all handlers across projects
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
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if route.Authorizer != "" {

			authorizer, err := s.GetProjectAuthorizer(projectId, route.Authorizer)
			if err != nil {
				log.Println(err)
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			log.Println("r.URL.Path   = ", r.URL.Path)
			if !route.CheckPathException(r.URL.Path) {

				token := r.Header.Get(authorizer.TokenHeaderKey)
				if token == "" {
					server_handlers.FormatResponse(w, http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized Request"})
					return
				}
				log.Println("token  == ", token)
				claims, err := authorizer.VerifyToken(r.Header.Get(authorizer.TokenHeaderKey))
				if err != nil {
					log.Println(err)
					server_handlers.FormatResponse(w, http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				claimsBytes, err := json.Marshal(claims)
				if err != nil {
					log.Println(err)
					server_handlers.FormatResponse(w, http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}
				r.Header.Add("claims", string(claimsBytes))
			}
		}
		response, _, err := route.Execute(r, url, false, "", nil, module_store.LoopThreads)
		log.Print("err = ", err)
		if route.Redirect {
			log.Print(route.FinalRedirectUrl)
			http.Redirect(w, r, route.FinalRedirectUrl, http.StatusSeeOther)
		} else {
			if err != nil {
				log.Println("================")
				log.Println(err)
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
		}

		/*
			//check error of first record only as it is same host
			log.Print("len(responses) = ", len(responses))
			log.Print("len(trVars) = ", len(trVars))
			log.Print("len(errs) = ", len(errs))

			if len(errs) > 0 {
				if errs[0] != nil {
					log.Println(" httpClient.Do error ")
					log.Println(err)
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": errs[0].Error()})
					return
				}
			}

			defer func(resps []*http.Response) {
				for _, resp := range resps {
					resp.Body.Close()
				}
			}(responses)

			//server_handlers.FormatResponse(w, response.StatusCode)
			log.Print("before range response.Header")

			if route.Redirect {
				log.Print(route.FinalRedirectUrl)
				http.Redirect(w, r, route.FinalRedirectUrl, http.StatusSeeOther)
			} else {
				//pick up header values of first response as it is all from same host
				log.Print("responses[0].Header = ", responses[0].Header)

				for k, v := range responses[0].Header {
					// for loop, content length is calculcated below based on all responses
					if k != "Content-Length" || route.LoopVariable == "" {
						for _, h := range v {
							w.Header().Set(k, h)
						}
					}
				}
				w.WriteHeader(responses[0].StatusCode)

				if route.LoopVariable != "" {
					var rJsonArray []interface{}
					for _, rp := range responses {
						var rJson interface{}
						err = json.NewDecoder(rp.Body).Decode(&rJson)
						if err != nil {
							log.Println("================")
							log.Println(err)
							w.WriteHeader(http.StatusBadRequest)
							_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
							return
						}
						rJsonArray = append(rJsonArray, rJson)
					}
					log.Print(rJsonArray)
					//err1 := json.NewEncoder(w).Encode(rJsonArray)
					rJsonArrayBytes, eee := json.Marshal(rJsonArray)
					log.Print(eee)
					log.Print(fmt.Sprint(len(rJsonArrayBytes)))
					w.Header().Set("Content-Length", fmt.Sprint(len(rJsonArrayBytes)))
					log.Print(w.Header().Get("Content-Length"))
					_, err = io.Copy(w, ioutil.NopCloser(bytes.NewBuffer(rJsonArrayBytes)))

					log.Print(err)
				} else {
					_, err = io.Copy(w, responses[0].Body)
					if err != nil {
						log.Println("================")
						log.Println(err)
						w.WriteHeader(http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
						return
					}
				}
				log.Print(w.Header())
			}
			return
		*/
	}

}
