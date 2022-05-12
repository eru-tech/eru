package handlers

import (
	"bytes"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	"mime/multipart"
	"os"
	"strconv"

	//"bytes"
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-routes/module_model"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"github.com/eru-tech/eru/eru-routes/routes"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	//"strconv"
	"strings"
)

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

const (
	encodedForm   = "application/x-www-form-urlencoded"
	multiPartForm = "multipart/form-data"
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
		route, err := s.GetAndValidateRoute(routeName, projectId, host, url, r.Method)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		trReqVars, err := transformRequest(r, route, url)
		if err != nil {
			log.Println("error from transformRequest")
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		log.Println("r.Header passed to httpClient.Do")
		log.Println(r.Header)
		log.Println(r.MultipartForm)
		response, err := httpClient.Do(r)
		if err != nil {
			log.Println(" httpClient.Do error ")
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		defer response.Body.Close()
		err = transformResponse(response, route, trReqVars)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		for k, v := range response.Header {
			log.Println(k, " = ", v)
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

		//w.WriteHeader(http.StatusOK)
		//w.Header().Set("Content")
		//_ = json.NewEncoder(w).Encode(respBody)

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

func transformRequest(request *http.Request, route routes.Route, url string) (vars module_model.TemplateVars, err error) {
	log.Println("inside transformRequest")
	reqVarsLoaded := false
	vars.FormData = make(map[string]interface{})

	scheme, host, port, path, err := route.GetTargetSchemeHostPortPath(url)
	if err != nil {
		return
	}

	// http: Request.RequestURI can't be set in client requests.
	// http://golang.org/src/pkg/net/http/client.go
	if port != "" {
		port = fmt.Sprint(":", port)
	}
	request.RequestURI = ""
	request.Host = host
	request.URL.Host = fmt.Sprint(host, port)
	request.URL.Path = path
	request.URL.Scheme = scheme

	for _, h := range route.RequestHeaders {
		if !h.IsTemplate {
			request.Header.Set(h.Key, h.Value)
		}
	}
	reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	log.Print("reqContentType = ", reqContentType)
	if reqContentType == encodedForm || reqContentType == multiPartForm {
		log.Println("inside encodedForm || multiPartForm")
		var reqBody bytes.Buffer
		multipartWriter := multipart.NewWriter(&reqBody)
		multiPart, err := request.MultipartReader()
		for {
			part, errPart := multiPart.NextRawPart()
			if errPart == io.EOF {
				break
			}
			if part.FileName() != "" {
				log.Println(part.FileName())
				log.Println(part)
				var tempFile *os.File
				tempFile, err = ioutil.TempFile(os.TempDir(), "spa")
				defer tempFile.Close()
				if err != nil {
					log.Println("Temp file creation failed")
				}
				//_, err = io.Copy(tempFile, part)
				//if err != nil {
				//	log.Println(err)
				//	return
				//}
				fileWriter, err := multipartWriter.CreateFormFile(part.FormName(), part.FileName())

				log.Println(part.Header)
				if err != nil {
					log.Println(err)
					return module_model.TemplateVars{}, err
				}
				_, err = io.Copy(fileWriter, part)
				if err != nil {
					log.Println(err)
					return module_model.TemplateVars{}, err
				}

			} else {
				log.Println(part.FormName())
				buf := new(bytes.Buffer)
				buf.ReadFrom(part)
				log.Println(buf.String())
				fieldWriter, err := multipartWriter.CreateFormField(part.FormName())
				if err != nil {
					log.Println(err)
					return module_model.TemplateVars{}, err
				}
				_, err = fieldWriter.Write(buf.Bytes())
				if err != nil {
					log.Println(err)
					return module_model.TemplateVars{}, err
				}
				vars.FormData[part.FormName()] = buf.String()
				vars.FormDataKeyArray = append(vars.FormDataKeyArray, part.FormName())
			}
		}
		for _, fd := range route.FormData {
			fieldWriter, err := multipartWriter.CreateFormField(fd.Key)
			if err != nil {
				log.Println(err)
				return module_model.TemplateVars{}, err
			}
			_, err = fieldWriter.Write([]byte(fd.Value))
			if err != nil {
				log.Println(err)
				return module_model.TemplateVars{}, err
			}
			vars.FormData[fd.Key] = fd.Value
			vars.FormDataKeyArray = append(vars.FormDataKeyArray, fd.Key)
		}
		//multipartWriter.Close()
		request.Body = ioutil.NopCloser(&reqBody)

		//request.Header.Set("Content-Type","application/pdf" )
		//multipartWriter.FormDataContentType()
		request.Header.Set("Content-Length", strconv.Itoa(reqBody.Len()))
		request.ContentLength = int64(reqBody.Len())
	}

	params := request.URL.Query()
	for _, p := range route.QueryParams {
		params.Set(p.Key, p.Value)
	}
	request.URL.RawQuery = params.Encode()

	log.Println("route.TransformRequest = ", route.TransformRequest)
	//vars := module_model.TemplateVars{}
	if route.TransformRequest != "" {

		if !reqVarsLoaded {
			err = loadRequestVars(&vars, request)
			if err != nil {
				log.Println(err)
				return
			}
			reqVarsLoaded = true
		}
		output, err := processTemplate(route, route.TransformRequest, vars, "json")
		if err != nil {
			log.Println(err)
			return module_model.TemplateVars{}, err
		}
		log.Println(string(output))
		request.Body = ioutil.NopCloser(bytes.NewBuffer(output))
		request.Header.Set("Content-Length", strconv.Itoa(len(output)))
		request.ContentLength = int64(len(output))
	}

	for _, h := range route.RequestHeaders {
		if h.IsTemplate {
			if !reqVarsLoaded {
				err = loadRequestVars(&vars, request)
				if err != nil {
					log.Println(err)
					return
				}
				reqVarsLoaded = true
			}
			log.Println("processTemplate called for header value")
			output, err := processTemplate(route, h.Value, vars, "string")
			if err != nil {
				log.Println(err)
				return module_model.TemplateVars{}, err
			}
			outputStr := string(output)
			if str, err := strconv.Unquote(outputStr); err == nil {
				log.Println("inside HasPrefix \"")
				outputStr = str
			}
			request.Header.Set(h.Key, outputStr)
		}
	}

	return
}

func transformResponse(response *http.Response, route routes.Route, trReqVars module_model.TemplateVars) (err error) {
	log.Println("inside transformResponse")
	for _, h := range route.ResponseHeaders {
		response.Header.Set(h.Key, h.Value)
	}
	log.Println("route.TransformResponse = ", route.TransformResponse)
	vars := module_model.TemplateVars{}
	if route.TransformResponse != "" {
		vars.Headers = make(map[string]interface{})
		for k, v := range response.Header {
			vars.Headers[k] = v
		}
		vars.Params = make(map[string]interface{})

		tmplBodyFromRes := json.NewDecoder(response.Body)
		tmplBodyFromRes.DisallowUnknownFields()
		if err = tmplBodyFromRes.Decode(&vars.Body); err != nil {
			log.Println(err)
			return err
		}
		vars.Vars = make(map[string]interface{})
		vars.Vars = trReqVars.Vars
		output, err := processTemplate(route, route.TransformResponse, vars, "json")
		if err != nil {
			log.Println(err)
			return err
		}
		response.Body = ioutil.NopCloser(bytes.NewBuffer(output))
		response.Header.Set("Content-Length", strconv.Itoa(len(output)))
		response.ContentLength = int64(len(output))
		log.Println("vars.Vars")
		log.Println(vars.Vars)
	}
	return
}

func processTemplate(route routes.Route, templateString string, vars module_model.TemplateVars, outputType string) (output []byte, err error) {
	log.Println("inside processTemplate")
	if strings.Contains(templateString, "{{.token") {
		strToken := vars.Headers[route.TokenSecret.HeaderKey]
		log.Println("strToken = ", strToken)
		log.Println("JwkUrl = ", route.TokenSecret.JwkUrl)
		vars.Token, err = route.FetchClaimsFromToken(strToken.(string))
		if err != nil {
			return
		}
	}
	goTmpl := gotemplate.GoTemplate{route.RouteName, templateString}
	outputObj, err := goTmpl.Execute(vars, outputType)
	if err != nil {
		log.Println(err)
		return nil, err
	} else {
		output, err = json.Marshal(outputObj)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		return
	}
}

func loadRequestVars(vars *module_model.TemplateVars, request *http.Request) (err error) {
	vars.Headers = make(map[string]interface{})
	for k, v := range request.Header {
		vars.Headers[k] = v
	}
	vars.Params = make(map[string]interface{})
	for k, v := range request.URL.Query() {
		vars.Params[k] = v
	}

	// if formData is found, no need to add body to vars
	if len(vars.FormData) <= 0 {
		tmplBodyFromReq := json.NewDecoder(request.Body)
		tmplBodyFromReq.DisallowUnknownFields()
		if err = tmplBodyFromReq.Decode(&vars.Body); err != nil {
			log.Println("error decode request body")
			log.Println(err)
			//return err
		}
	}
	vars.Vars = make(map[string]interface{})
	return
}
