package handlers

import (
	"bufio"
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-events/events"
	"github.com/eru-tech/eru/eru-functions/functions"
	"github.com/eru-tech/eru/eru-functions/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func WfHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("WfHandler - Start")
		// Close the body of the request
		//TODO to add request body close in all handlers across projects
		defer r.Body.Close()
		ctx := context.WithValue(r.Context(), "allowed_origins", server_handlers.AllowedOrigins)
		ctx = context.WithValue(ctx, "origin", r.Header.Get("Origin"))
		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		vars := mux.Vars(r)
		projectId := vars["project"]
		wfName := vars["wfname"]

		wfObj, err := s.GetWf(ctx, wfName, projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		logs.WithContext(r.Context()).Info(fmt.Sprint(wfObj))
		// Lookup a functions in a function based on host and url
		fn := ""
		for _, v := range wfObj.WfEvents {
			fn = v.Function_Name
		}
		funcGroup, err := s.GetAndValidateFunc(ctx, fn, projectId, host, url, r.Method, r.Header, nil, s, false)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		reqVars := make(map[string]*functions.TemplateVars)
		resVars := make(map[string]*functions.TemplateVars)
		response, _, err := funcGroup.Execute(ctx, r, module_store.FuncThreads, module_store.LoopThreads, "", "", false, reqVars, resVars)

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

func AsyncFuncHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Info("AsyncFuncHandler - Start")
		//logs.FileLogger.Info(fmt.Sprint("AsyncFuncHandler started "))
		// Close the body of the request
		//TODO to add request body close in all handlers across projects
		startTime := time.Now()
		endTime := time.Now()
		defer r.Body.Close()
		ctx := context.WithValue(r.Context(), "allowed_origins", server_handlers.AllowedOrigins)
		ctx = context.WithValue(ctx, "origin", r.Header.Get("Origin"))
		// Extract the host and url from incoming request
		host, url := extractHostUrl(r)
		vars := mux.Vars(r)
		projectId := vars["project"]
		eventName := vars["eventname"]
		eventId := vars["eventid"]
		eventI, err := s.FetchEvent(r.Context(), projectId, eventName)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "event not found"})
			return
		}
		endTime = time.Now()
		diff := endTime.Sub(startTime)
		logs.WithContext(r.Context()).Info(fmt.Sprint("total time taken for is FetchEvent ", diff.Milliseconds(), "seconds"))
		var eventMsgs []events.EventMsg
		if eventId == "" {
			logs.WithContext(ctx).Info("polling events")
			eventMsgs, err = eventI.Poll(r.Context())
			if err != nil {
				server_handlers.FormatResponse(w, http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "could not fetch messages from event queue"})
				return
			}
		} else {
			eventMsgs = append(eventMsgs, events.EventMsg{Msg: eventId})
		}

		processedCount := 0
		failedCount := 0
		aStatus := "PENDING"
		if eventId != "" {
			aStatus = "ALL"
		}
		for _, m := range eventMsgs {
			asyncStatus := "PROCESSED"
			var asyncFuncData module_store.AsyncFuncData
			asyncFuncData, err = s.FetchAsyncEvent(ctx, m.Msg, aStatus, s)
			//	logs.FileLogger.Info(fmt.Sprint("AsyncFuncHandler for FetchAsyncEvent "))
			if err != nil || asyncFuncData.AsyncId == "" {
				failedCount = failedCount + 1
				asyncStatus = "FAILED"
				logs.WithContext(ctx).Error("event not found")
			} else {
				bodyMap := make(map[string]interface{})
				eventResponseBytes := []byte("")
				bodyMapOk := false
				if bodyMap, bodyMapOk = asyncFuncData.EventMsg.Vars.Body.(map[string]interface{}); !bodyMapOk {
					logs.WithContext(ctx).Error("Request Body count not be retrieved, setting it as blank")
				}
				funcGroup, err := s.GetAndValidateFunc(ctx, asyncFuncData.FuncName, projectId, host, url, r.Method, r.Header, bodyMap, s, true)
				//	logs.FileLogger.Info(fmt.Sprint("AsyncFuncHandler for GetAndValidateFunc"))
				if err != nil {
					failedCount = failedCount + 1
					asyncStatus = "FAILED"
					logs.WithContext(ctx).Error("Function validation failed")
				} else {
					reqBytes := []byte("")
					reqBytes, err = b64.StdEncoding.DecodeString(asyncFuncData.EventRequest)
					if err != nil {
						failedCount = failedCount + 1
						asyncStatus = "FAILED"
						logs.WithContext(ctx).Error("event request decoding failed")
					} else {
						var newReq *http.Request
						if newReq, err = http.ReadRequest(bufio.NewReader(bytes.NewReader(reqBytes))); err != nil { // deserialize request
							failedCount = failedCount + 1
							asyncStatus = "FAILED"
							logs.WithContext(ctx).Error("event request deserialization failed")
						}
						reqVars := make(map[string]*functions.TemplateVars)
						resVars := make(map[string]*functions.TemplateVars)
						if asyncFuncData.EventMsg.ReqVars != nil {
							reqVars = asyncFuncData.EventMsg.ReqVars
						}
						if asyncFuncData.EventMsg.ResVars != nil {
							resVars = asyncFuncData.EventMsg.ResVars
						}
						//logs.FileLogger.Info(fmt.Sprint("AsyncFuncHandler Before funcGroup.Execute "))
						response, funcVarsMap, err := funcGroup.Execute(ctx, newReq, module_store.FuncThreads, module_store.LoopThreads, asyncFuncData.FuncStepName, "", true, reqVars, resVars)
						//logs.FileLogger.Info(fmt.Sprint("AsyncFuncHandler After funcGroup.Execute "))
						if err != nil {
							failedCount = failedCount + 1
							asyncStatus = "FAILED"
							logs.WithContext(ctx).Error(err.Error())
							logs.WithContext(ctx).Error("Function execution failed")
						} else {
							responseBytes := []byte("")
							responseBytes, err = io.ReadAll(response.Body)
							if err != nil {
								logs.WithContext(ctx).Error(err.Error())
								failedCount = failedCount + 1
								asyncStatus = "FAILED"
							} else {
								response.Body = io.NopCloser(bytes.NewBuffer(responseBytes))
								responseStr := string(responseBytes)
								eventResponse := make(map[string]interface{})
								eventResponse["response"] = responseStr
								eventResponse["func_vars"] = funcVarsMap
								eventResponseBytes, err = json.Marshal(eventResponse)
								if err != nil {
									logs.WithContext(ctx).Error(err.Error())
									failedCount = failedCount + 1
									asyncStatus = "FAILED"
								} else {
									logs.WithContext(ctx).Info(fmt.Sprint(response))
									utils.PrintResponseBody(ctx, response, "printing response from async handler")
									processedCount = processedCount + 1
								}
							}
						}
						defer func() {
							if response != nil {
								response.Body.Close()
							}
						}()
					}
				}
				//	logs.FileLogger.Info(fmt.Sprint("AsyncFuncHandler Before UpdateAsyncEvent "))
				_ = s.UpdateAsyncEvent(ctx, m.Msg, asyncStatus, string(eventResponseBytes), s)
				//		logs.FileLogger.Info(fmt.Sprint("AsyncFuncHandler After UpdateAsyncEvent "))
				_ = eventI.DeleteMessage(ctx, m.MsgIdentifer)
				//		logs.FileLogger.Info(fmt.Sprint("AsyncFuncHandler After DeleteMessage "))

			}
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"processed": processedCount, "failed": failedCount})
		//logs.FileLogger.Info(fmt.Sprint("AsyncFuncHandler ended "))
		return
	}
}

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
		endfuncStepName := vars["endfuncstepname"]
		// Lookup a functions in a function based on host and url

		reqContentType := strings.Split(r.Header.Get("Content-type"), ";")[0]
		bodyMap := make(map[string]interface{})
		if reqContentType == "application/json" && r.ContentLength > 0 {

			tmplBodyFromReq := json.NewDecoder(r.Body)
			tmplBodyFromReq.DisallowUnknownFields()
			if err := tmplBodyFromReq.Decode(&bodyMap); err != nil {
				logs.WithContext(r.Context()).Error(fmt.Sprint("error decode request body : ", err.Error()))
				server_handlers.FormatResponse(w, http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to decode request body"})
				return
			}
			body, err := json.Marshal(bodyMap)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("json.Marshal(vars.Body) error : ", err.Error()))
				server_handlers.FormatResponse(w, http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to marshal request body"})
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.Header.Set("Content-Length", strconv.Itoa(len(body)))
			r.ContentLength = int64(len(body))
		}

		funcGroup, err := s.GetAndValidateFunc(ctx, funcName, projectId, host, url, r.Method, r.Header, bodyMap, s, false)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		errStatusCode := http.StatusBadRequest
		if funcGroup.ResponseStatusCode > 0 {
			errStatusCode = funcGroup.ResponseStatusCode
		}
		reqVars := make(map[string]*functions.TemplateVars)
		resVars := make(map[string]*functions.TemplateVars)
		response, funcVarsMap, err := funcGroup.Execute(ctx, r, module_store.FuncThreads, module_store.LoopThreads, funcStepName, endfuncStepName, false, reqVars, resVars)
		if err != nil {
			server_handlers.FormatResponse(w, errStatusCode)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		for k, v := range funcVarsMap {
			logs.WithContext(ctx).Info(fmt.Sprint(k))
			logs.WithContext(ctx).Info(fmt.Sprint(v))

		}
		defer response.Body.Close()
		if response.StatusCode >= 300 && response.StatusCode <= 399 {
			http.Redirect(w, r, response.Header.Get("Location"), response.StatusCode)
		} else {

			for k, v := range response.Header {
				w.Header()[k] = v
			}
			respStatusCode := response.StatusCode
			logs.WithContext(r.Context()).Info(fmt.Sprint(respStatusCode))
			if funcGroup.ResponseStatusCode > 0 {
				respStatusCode = funcGroup.ResponseStatusCode
			}
			logs.WithContext(r.Context()).Info(fmt.Sprint(respStatusCode))
			w.WriteHeader(respStatusCode)
			_, err = io.Copy(w, response.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				w.WriteHeader(errStatusCode)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			return
		}
	}
}

func SFuncHandler(s module_store.ModuleStoreI) http.HandlerFunc {
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
		endfuncStepName := vars["endfuncstepname"]
		// Lookup a functions in a function based on host and url

		reqContentType := strings.Split(r.Header.Get("Content-type"), ";")[0]

		type reqBody struct {
			Body    map[string]interface{}             `json:"body"`
			ReqVars map[string]*functions.TemplateVars `json:"req_vars"`
			ResVars map[string]*functions.TemplateVars `json:"res_vars"`
		}

		bodyMap := reqBody{}
		if reqContentType == "application/json" && r.ContentLength > 0 {

			tmplBodyFromReq := json.NewDecoder(r.Body)
			tmplBodyFromReq.DisallowUnknownFields()
			if err := tmplBodyFromReq.Decode(&bodyMap); err != nil {
				logs.WithContext(r.Context()).Error(fmt.Sprint("error decode request body : ", err.Error()))
				server_handlers.FormatResponse(w, http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to decode request body"})
				return
			}
			body, err := json.Marshal(bodyMap.Body)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("json.Marshal(vars.Body) error : ", err.Error()))
				server_handlers.FormatResponse(w, http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to marshal request body"})
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.Header.Set("Content-Length", strconv.Itoa(len(body)))
			r.ContentLength = int64(len(body))
		}

		funcGroup, err := s.GetAndValidateFunc(ctx, funcName, projectId, host, url, r.Method, r.Header, bodyMap.Body, s, false)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		errStatusCode := http.StatusBadRequest
		if funcGroup.ResponseStatusCode > 0 {
			errStatusCode = funcGroup.ResponseStatusCode
		}
		reqVars := bodyMap.ReqVars
		resVars := bodyMap.ResVars
		response, funcVarsMap, err := funcGroup.Execute(ctx, r, module_store.FuncThreads, module_store.LoopThreads, funcStepName, endfuncStepName, false, reqVars, resVars)
		if err != nil {
			server_handlers.FormatResponse(w, errStatusCode)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		for k, v := range funcVarsMap {
			logs.WithContext(ctx).Info(fmt.Sprint(k))
			logs.WithContext(ctx).Info(fmt.Sprint(v))

		}
		defer response.Body.Close()
		if response.StatusCode >= 300 && response.StatusCode <= 399 {
			http.Redirect(w, r, response.Header.Get("Location"), response.StatusCode)
		} else {

			for k, v := range response.Header {
				w.Header()[k] = v
			}
			respStatusCode := response.StatusCode
			logs.WithContext(r.Context()).Info(fmt.Sprint(respStatusCode))
			if funcGroup.ResponseStatusCode > 0 {
				respStatusCode = funcGroup.ResponseStatusCode
			}
			logs.WithContext(r.Context()).Info(fmt.Sprint(respStatusCode))
			w.WriteHeader(respStatusCode)
			_, err = io.Copy(w, response.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				w.WriteHeader(errStatusCode)
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
		endFuncStepName := vars["endfuncstepname"]

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
				var funcObj functions.FuncGroup
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

					funcGroup, err := s.ValidateFunc(ctx, funcObj, projectId, host, url, r.Method, r.Header, nil, s, false)
					if err != nil {
						server_handlers.FormatResponse(w, http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
						return
					}
					reqVars := make(map[string]*functions.TemplateVars)
					resVars := make(map[string]*functions.TemplateVars)
					response, _, err := funcGroup.Execute(ctx, r, module_store.FuncThreads, module_store.LoopThreads, funcStepName, endFuncStepName, false, reqVars, resVars)
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
