package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/eru-tech/eru/eru-gateway/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	utils "github.com/eru-tech/eru/eru-utils"
)

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func RouteHandler(sh *module_store.StoreHolder, rh *RegistryHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		logs.WithContext(r.Context()).Debug("RouteHandler - Start")
		host, url := extractHostUrl(r)
		logs.WithContext(r.Context()).Info(host)
		logs.WithContext(r.Context()).Info(url)
		tg, authorizer, addHeaders, instanceId, err := sh.Store.GetTargetGroupAuthorizer(r.Context(), r)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "suspicious activity"})
			return
		}
		logs.WithContext(r.Context()).Info(fmt.Sprint("authorizer.AuthorizerName = ", authorizer.AuthorizerName))
		if authorizer.AuthorizerName != "" {
			accessToken := r.Header.Get(authorizer.TokenHeaderKey)
			idToken := r.Header.Get(authorizer.IdTokenKey)
			token := ""
			if idToken != "" {
				token = idToken
			} else {
				token = accessToken
			}
			if token == "" || accessToken == "" {
				logs.WithContext(r.Context()).Info("token = \"\"")
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized Request"})
				logs.WithContext(r.Context()).Info(fmt.Sprint(http.StatusUnauthorized))
				return
			}

			accessClaims, err := authorizer.VerifyToken(r.Context(), accessToken, r.Header.Get(authorizer.KidHeaderKey))
			if err != nil {
				logs.WithContext(r.Context()).Error("access token verification failed")
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			accessClaimsMap, accessClaimsMapOk := accessClaims.(map[string]interface{})
			if !accessClaimsMapOk {
				logs.WithContext(r.Context()).Error("access token is not a map")
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			accessSub, accessSubOk := accessClaimsMap["sub"]
			if !accessSubOk {
				err = fmt.Errorf("access token sub is not set")
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			claims, err := authorizer.VerifyToken(r.Context(), token, r.Header.Get(authorizer.KidHeaderKey))
			if err != nil {
				logs.WithContext(r.Context()).Error("id token verification failed")
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			claimsMap, claimsMapOk := claims.(map[string]interface{})
			if !claimsMapOk {
				logs.WithContext(r.Context()).Error("id token is not a map")
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			idSub, idSubOk := claimsMap["sub"]
			if !idSubOk {
				err = fmt.Errorf("id token sub is not set")
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			if idSub.(string) != accessSub.(string) {
				err = fmt.Errorf("sub mismatch")
				logs.WithContext(r.Context()).Error(err.Error())
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
			r.Header.Set("claims", string(claimsBytes))

			if authorizer.KidHeaderKey == "" {
				valid := authorizer.VerifyAccessToken(r.Context(), accessToken)
				if !valid {
					logs.WithContext(r.Context()).Info("invalid access token")
					server_handlers.FormatResponse(w, http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized Request"})
					return
				}
			}
		}

		for _, v := range addHeaders {
			headerValue := ""
			if v.IsTemplate {
				goTmpl := gotemplate.GoTemplate{v.Key, v.Value}
				outputObj, err := goTmpl.Execute(r.Context(), *r, "string")
				if err != nil {
					err = logs.Err(r.Context(), err, "")
					server_handlers.FormatResponse(w, http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				} else {
					output, err := json.Marshal(outputObj)
					if err != nil {
						err = logs.Err(r.Context(), err, "")
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

		if instanceId != "" {
			instances, err := rh.Registry.ListAllServices(r.Context())
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			} else {
				instanceFound := false
				for _, instance := range instances {
					if instance.Id == instanceId {
						instanceFound = true
						tmpSplit1 := strings.Split(instance.Address, "://")
						if tmpSplit1[0] == "http" || tmpSplit1[0] == "https" {
							tmpSplit2 := strings.Split(tmpSplit1[1], ":")
							logs.WithContext(r.Context()).Info(fmt.Sprint("tmpSplit2 = ", tmpSplit2))
							logs.WithContext(r.Context()).Info(fmt.Sprint("tmpSplit1 = ", tmpSplit1))
							r.Host = tmpSplit2[0]
							if len(tmpSplit1) > 1 {
								r.URL.Host = fmt.Sprint(tmpSplit1[1])
							} else {
								r.URL.Host = fmt.Sprint(tmpSplit1[0])
							}
							r.URL.Scheme = tmpSplit1[0]
							break
						}
					}
				}
				if !instanceFound {
					err = fmt.Errorf("instance not found")
					logs.WithContext(r.Context()).Error(err.Error())
					server_handlers.FormatResponse(w, 400)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
					return
				}
			}
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

		//remove CORS headers from w else it is getting passed even if rsponse is not sedning any header
		w.Header().Del("Access-Control-Allow-Credentials")
		w.Header().Del("Access-Control-Allow-Origin")
		w.Header().Del("Access-Control-Allow-Headers")
		w.Header().Del("Access-Control-Allow-Methods")

		for k, v := range response.Header {
			//logs.WithContext(r.Context()).Info(fmt.Sprint(k, " - ", v))
			w.Header()[k] = v
		}
		w.WriteHeader(response.StatusCode)
		if strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
			flusher, canFlush := w.(http.Flusher)
			if canFlush {
				flusher.Flush()
			}
			buf := make([]byte, 4096)
			for {
				n, readErr := response.Body.Read(buf)
				if n > 0 {
					if _, writeErr := w.Write(buf[:n]); writeErr != nil {
						logs.WithContext(r.Context()).Error(writeErr.Error())
						break
					}
					if canFlush {
						flusher.Flush()
					}
				}
				if readErr != nil {
					break
				}
			}
		} else {
			_, err = io.Copy(w, response.Body)
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
		}
		//logs.WithContext(r.Context()).Info(fmt.Sprint("---------------------------"))
		//logs.WithContext(r.Context()).Info(fmt.Sprint(w.Header()))
	}
}
func extractHostUrl(request *http.Request) (string, string) {
	return strings.Split(request.Host, ":")[0], request.URL.Path
}
