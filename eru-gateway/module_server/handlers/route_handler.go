package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-gateway/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func RouteHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		log.Println(host)
		log.Println(url)
		tg, authorizer, addHeaders, err := s.GetTargetGroupAuthorizer(r)
		log.Println(fmt.Sprint("Error from s.GetTargetGroupAuthorizer = ", err))
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		if authorizer.AuthorizerName != "" {
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
		log.Println(tg)

		for _, v := range addHeaders {
			log.Println("inside addHeaders loop")
			headerValue := ""
			if v.IsTemplate {
				goTmpl := gotemplate.GoTemplate{v.Key, v.Value}
				outputObj, err := goTmpl.Execute(*r, "string")
				if err != nil {
					log.Println(err)
					server_handlers.FormatResponse(w, http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				} else {
					output, err := json.Marshal(outputObj)
					if err != nil {
						log.Println(err)
						server_handlers.FormatResponse(w, http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
						return
					}
					log.Println(string(output))
					if str, err := strconv.Unquote(string(output)); err == nil {
						headerValue = str
					} else {
						headerValue = string(output)
					}
				}
			} else {
				headerValue = v.Value
			}
			log.Println(v.Key, " ", headerValue)
			r.Header.Set(v.Key, headerValue)
		}

		port := ""
		if tg.Port != "" {
			port = fmt.Sprint(":", tg.Port)
		}
		r.RequestURI = ""
		r.Host = tg.Host
		r.URL.Host = fmt.Sprint(tg.Host, port)
		r.URL.Scheme = tg.Scheme
		if tg.Method != "" {
			r.Method = tg.Method
		}
		log.Println(r)
		response, err := httpClient.Do(r)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
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
func extractHostUrl(request *http.Request) (string, string) {
	return strings.Split(request.Host, ":")[0], request.URL.Path
}
