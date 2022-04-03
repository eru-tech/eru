package handlers

import (
	//"bytes"
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"github.com/eru-tech/eru/eru-routes/routes"
	"github.com/gorilla/mux"
	"net/http"
	//"strconv"
	"strings"
)

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func RouteHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Close the body of the request
		//defer utils.CloseTheCloser(request.Body)  //TODO to add request body close in all handlers across projects

		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		vars := mux.Vars(r)
		projectId := vars["project"]
		routeName := vars["routename"]

		// Lookup a route based on host and url
		route, err := s.GetAndValidateRoute(routeName, projectId, host, url, r.Method)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		//v["host"] = host1
		//v["path"] = path1

		err = transformRequest(r, route, url)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		response, err := httpClient.Do(r)
		defer response.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		var respBody map[string]interface{}
		if err = json.NewDecoder(response.Body).Decode(&respBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(respBody)

		/*


			token, claims, status, err := r.modifyRequest(request.Context(), modules, route, request)
			if err != nil {
				writer.WriteHeader(status)
				_ = json.NewEncoder(writer).Encode(map[string]string{"error": err.Error()})
				return
			}

			helpers.Logger.LogDebug(helpers.GetRequestID(request.Context()), fmt.Sprintf("selected route (%v) for request (%s)", route, request.URL.String()), nil)

			// Apply the rewrite url if provided. It is the users responsibility to make sure both url
			// and rewrite url starts with a '/'
			url = rewriteURL(url, route)

			// Proxy the request

			if err := setRequest(request.Context(), request, route, url); err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(writer).Encode(map[string]string{"error": err.Error()})
				_ = helpers.Logger.LogError(helpers.GetRequestID(request.Context()), fmt.Sprintf("Failed set request for route (%v)", route), err, nil)
				return
			}

			var redisKey string
			if route.IsRouteCacheable && request.Method == http.MethodGet {
				cacheOptionsArray := make([]interface{}, 0)
				for _, key := range route.CacheOptions {
					value, err := utils.LoadValue(key, map[string]interface{}{"args": map[string]interface{}{"auth": claims, "token": token, "url": request.URL.String()}})
					if err != nil {
						_ = helpers.Response.SendErrorResponse(request.Context(), writer, http.StatusBadRequest, err)
						return
					}
					cacheOptionsArray = append(cacheOptionsArray, value)
				}

				key, isCacheHit, result, err := r.caching.GetIngressRoute(request.Context(), route.ID, cacheOptionsArray)
				if err != nil {
					_ = helpers.Response.SendErrorResponse(request.Context(), writer, http.StatusBadRequest, err)
					return
				}
				if isCacheHit {
					for k, v := range result.Headers {
						writer.Header()[k] = v
					}
					writer.WriteHeader(http.StatusOK)
					n, err := io.Copy(writer, ioutil.NopCloser(bytes.NewBuffer(result.Body)))
					if err != nil {
						_ = helpers.Logger.LogError(helpers.GetRequestID(request.Context()), fmt.Sprintf("Failed to copy upstream (%s) response to downstream", request.URL.String()), err, nil)
					}
					helpers.Logger.LogDebug(helpers.GetRequestID(request.Context()), fmt.Sprintf("Successfully copied %d bytes from upstream server (%s)", n, request.URL.String()), nil)
					return
				}
				redisKey = key
			}

			// TODO: Use http2 client if that was the incoming request protocol
			response, err := httpClient.Do(request)
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(writer).Encode(map[string]string{"error": err.Error()})
				_ = helpers.Logger.LogError(helpers.GetRequestID(request.Context()), fmt.Sprintf("Failed to make request for route (%v)", route), err, nil)
				return
			}
			defer utils.CloseTheCloser(response.Body)

			if err := r.modifyResponse(request.Context(), response, route, token, claims); err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(writer).Encode(map[string]string{"error": err.Error()})
				return
			}

			values := response.Header.Get("cache-control")
			if values != "" && route.IsRouteCacheable && redisKey != "" && request.Method == http.MethodGet {
				var cacheTime string
				for _, value := range strings.Split(values, ",") {
					if value == "no-cache" {
						break
					}
					value = strings.TrimSpace(value)
					if strings.HasPrefix(value, "max-age") {
						cacheTime = strings.Split(value, "=")[1]
						break
					}
					if strings.HasPrefix(value, "s-maxage") {
						cacheTime = strings.Split(value, "=")[1]
						break
					}
				}
				if cacheTime != "" {
					duration, err := strconv.Atoi(cacheTime)
					if err != nil {
						_ = helpers.Logger.LogError(helpers.GetRequestID(request.Context()), fmt.Sprintf("Failed to copy upstream (%s) response to downstream", request.URL.String()), err, nil)
					}
					data, err := ioutil.ReadAll(response.Body)
					if err != nil {
						_ = helpers.Logger.LogError(helpers.GetRequestID(request.Context()), fmt.Sprintf("Failed to copy upstream (%s) response to downstream", request.URL.String()), err, nil)
					}
					if err := r.caching.SetIngressRouteKey(request.Context(), redisKey, &config.ReadCacheOptions{TTL: int64(duration)}, &model.CacheIngressRoute{Headers: response.Header, Body: data}); err != nil {
						_ = helpers.Logger.LogError(helpers.GetRequestID(request.Context()), fmt.Sprintf("Failed to copy upstream (%s) response to downstream", request.URL.String()), err, nil)
					}
					response.Body = ioutil.NopCloser(bytes.NewBuffer(data))
				}
			}

			// Copy headers and status code
			for k, v := range response.Header {
				writer.Header()[k] = v
			}
			writer.WriteHeader(response.StatusCode)

			// Copy the body
			n, err := io.Copy(writer, response.Body)
			if err != nil {
				_ = helpers.Logger.LogError(helpers.GetRequestID(request.Context()), fmt.Sprintf("Failed to copy upstream (%s) response to downstream", request.URL.String()), err, nil)
			}

			helpers.Logger.LogDebug(helpers.GetRequestID(request.Context()), fmt.Sprintf("Successfully copied %d bytes from upstream server (%s)", n, request.URL.String()), nil)
		*/
	}

}

func extractHostUrl(request *http.Request) (string, string) {
	return strings.Split(request.Host, ":")[0], request.URL.Path
}

func transformRequest(request *http.Request, route routes.Route, url string) (err error) {
	scheme, host, port, path, err := route.GetTargetSchemeHostPortPath(url)
	if err != nil {
		return
	}

	// http: Request.RequestURI can't be set in client requests.
	// http://golang.org/src/pkg/net/http/client.go
	request.RequestURI = ""
	request.Host = host
	request.URL.Host = fmt.Sprint(host, ":", port)
	request.URL.Path = path
	request.URL.Scheme = scheme
	//log.Println(request.URL)
	for _, h := range route.RequestHeaders {
		request.Header.Set(h.Key, h.Value)
	}

	return
}
