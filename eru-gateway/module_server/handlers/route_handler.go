package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-gateway/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	utils "github.com/eru-tech/eru/eru-utils"
	"io"
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
		logs.WithContext(r.Context()).Debug("RouteHandler - Start")
		host, url := extractHostUrl(r)
		logs.WithContext(r.Context()).Info(host)
		logs.WithContext(r.Context()).Info(url)
		tg, authorizer, addHeaders, err := s.GetTargetGroupAuthorizer(r.Context(), r)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		logs.WithContext(r.Context()).Info(fmt.Sprint("authorizer.AuthorizerName = ", authorizer.AuthorizerName))
		if authorizer.AuthorizerName != "" {
			token := r.Header.Get(authorizer.TokenHeaderKey)
			if token == "" {
				logs.WithContext(r.Context()).Info("token = \"\"")
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized Request"})
				logs.WithContext(r.Context()).Info(fmt.Sprint(http.StatusUnauthorized))
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

		for _, v := range addHeaders {
			headerValue := ""
			if v.IsTemplate {
				goTmpl := gotemplate.GoTemplate{v.Key, v.Value}
				outputObj, err := goTmpl.Execute(r.Context(), *r, "string")
				if err != nil {
					logs.WithContext(r.Context()).Error(err.Error())
					server_handlers.FormatResponse(w, http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				} else {
					output, err := json.Marshal(outputObj)
					if err != nil {
						logs.WithContext(r.Context()).Error(err.Error())
						server_handlers.FormatResponse(w, http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
						return
					}
					if str, err := strconv.Unquote(string(output)); err == nil {
						headerValue = str
					} else {
						headerValue = string(output)
					}
				}
			} else {
				headerValue = v.Value
			}
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
		//response, err := httpClient.Do(r)
		response, err := utils.ExecuteHttp(r.Context(), r)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		//defer response.Body.Close()
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
	}
}
func extractHostUrl(request *http.Request) (string, string) {
	return strings.Split(request.Host, ":")[0], request.URL.Path
}
