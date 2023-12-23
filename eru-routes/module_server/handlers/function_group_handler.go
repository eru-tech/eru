package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"github.com/eru-tech/eru/eru-routes/routes"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"strconv"
)

func FuncHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("FuncHandler - Start")
		// Close the body of the request
		//TODO to add request body close in all handlers across projects
		defer r.Body.Close()
		ctx := context.WithValue(r.Context(), "allowed_origins", server_handlers.AllowedOrigins)
		ctx = context.WithValue(ctx, "origin", r.Header.Get("Origin"))
		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		vars := mux.Vars(r)
		projectId := vars["project"]
		funcName := vars["funcname"]
		funcStepName := vars["funcstepname"]
		// Lookup a routes in a function based on host and url
		funcGroup, err := s.GetAndValidateFunc(ctx, funcName, projectId, host, url, r.Method, r.Header, s)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		response, err := funcGroup.Execute(ctx, r, module_store.FuncThreads, module_store.LoopThreads, funcStepName)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		defer response.Body.Close()
		if response.StatusCode >= 300 && response.StatusCode <= 399 {
			http.Redirect(w, r, response.Header.Get("Location"), response.StatusCode)
		} else {

			for k, v := range response.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(response.StatusCode)
			_, err = io.Copy(w, response.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			return
		}
	}
}

func FuncRunHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("FuncRunHandler - Start")
		ctx := context.WithValue(r.Context(), "allowed_origins", server_handlers.AllowedOrigins)
		ctx = context.WithValue(ctx, "origin", r.Header.Get("Origin"))

		host, url := extractHostUrl(r)
		vars := mux.Vars(r)
		projectId := vars["project"]
		funcStepName := vars["funcstepname"]

		funcFromReq := json.NewDecoder(r.Body)
		funcFromReq.DisallowUnknownFields()

		var funcMap map[string]interface{}

		if err := funcFromReq.Decode(&funcMap); err != nil {
			logs.WithContext(ctx).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			if funcJson, funcJsonOk := funcMap["func"]; funcJsonOk {
				funcJsonBytes, funcJsonBytesErr := json.Marshal(funcJson)
				if funcJsonBytesErr != nil {
					server_handlers.FormatResponse(w, http.StatusBadRequest)
					logs.WithContext(ctx).Error(funcJsonBytesErr.Error())
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "function body could not be read from json"})
					return
				}
				var funcObj routes.FuncGroup
				funcObjD := json.NewDecoder(bytes.NewReader(funcJsonBytes))
				funcObjD.DisallowUnknownFields()

				if err = funcObjD.Decode(&funcObj); err == nil {
					err = utils.ValidateStruct(ctx, funcObj, "")
					if err != nil {
						server_handlers.FormatResponse(w, 400)
						json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
						return
					}
					if rBody, rBodyOk := funcMap["body"]; rBodyOk {
						rBodyBytes, rBodyBytesErr := json.Marshal(rBody)
						if rBodyBytesErr != nil {
							server_handlers.FormatResponse(w, http.StatusBadRequest)
							logs.WithContext(ctx).Error(rBodyBytesErr.Error())
							_ = json.NewEncoder(w).Encode(map[string]string{"error": "function body could not be read"})
							return
						}
						r.Body = io.NopCloser(bytes.NewReader(rBodyBytes))
						r.Header.Set("Content-Length", strconv.Itoa(len(rBodyBytes)))
						r.ContentLength = int64(len(rBodyBytes))
					} else {
						err = errors.New("function body not found")
						logs.WithContext(ctx).Error(err.Error())
						w.WriteHeader(http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					}

					funcGroup, err := s.ValidateFunc(ctx, funcObj, projectId, host, url, r.Method, r.Header, s)
					if err != nil {
						server_handlers.FormatResponse(w, http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
						return
					}

					response, err := funcGroup.Execute(ctx, r, module_store.FuncThreads, module_store.LoopThreads, funcStepName)
					if err != nil {
						server_handlers.FormatResponse(w, http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
						return
					}

					defer response.Body.Close()
					if response.StatusCode >= 300 && response.StatusCode <= 399 {
						http.Redirect(w, r, response.Header.Get("Location"), response.StatusCode)
					} else {

						for k, v := range response.Header {
							w.Header()[k] = v
						}
						w.WriteHeader(response.StatusCode)
						_, err = io.Copy(w, response.Body)
						if err != nil {
							logs.WithContext(ctx).Error(err.Error())
							w.WriteHeader(http.StatusBadRequest)
							_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
							return
						}
						return
					}

				} else {
					err := errors.New("function definition could not be read")
					logs.WithContext(ctx).Error(err.Error())
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				}
			} else {
				err := errors.New("function definition not found")
				logs.WithContext(ctx).Error(err.Error())
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			}
		}
	}
}
