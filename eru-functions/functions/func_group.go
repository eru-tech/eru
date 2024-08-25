package functions

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-db/db"
	"github.com/eru-tech/eru/eru-events/events"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	eru_utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	INSERT_FUNC_ASYNC = "insert into erufunctions_async (async_id,event_id,func_group_name,func_step_name,event_msg,request_id, event_request) values (???,???,???,???,???,???,???)"
	SELECT_FUNC_ASYNC = "update erufunctions_async set async_status='IN PROGRESS', processed_date=now() where async_id = ??? and (async_status=??? or 'ALL'=???) returning async_id, event_id, func_group_name func_name, func_step_name,  event_msg, event_request, request_id"
	UPDATE_FUNC_ASYNC = "update erufunctions_async set async_status=???, processed_date=now(), event_response=??? where async_id = ???"
)

type AsyncFuncData struct {
	AsyncId      string           `json:"async_id"`
	EventId      string           `json:"event_id"`
	FuncName     string           `json:"func_group_name"`
	FuncStepName string           `json:"func_step_name"`
	EventMsg     FuncTemplateVars `json:"event_msg"`
	EventRequest string           `json:"event_request"`
	RequestId    string           `json:"request_id"`
}

type FuncGroup struct {
	FuncCategoryName   string               `json:"func_category_name"`
	FuncGroupName      string               `json:"func_group_name"`
	FuncSteps          map[string]*FuncStep `json:"func_steps"`
	TokenSecretKey     string               `json:"-"`
	ResponseStatusCode int                  `json:"response_status_code"`
}

type FuncTemplateVars struct {
	Vars    *TemplateVars
	ReqVars map[string]*TemplateVars
	ResVars map[string]*TemplateVars
}

type FuncStep struct {
	Condition              string        `json:"condition"`
	ConditionFailMessage   string        `json:"condition_fail_message"`
	ConditionFailAction    string        `json:"condition_fail_action"`
	Delay                  int64         `json:"delay"`
	Async                  bool          `json:"async"`
	AsyncMessage           string        `json:"async_message"`
	AsyncEventName         string        `json:"async_event"`
	AsyncEvent             events.EventI `json:"-"`
	LoopVariable           string        `json:"loop_variable"`
	LoopInParallel         bool          `json:"loop_in_parallel"`
	RouteName              string        `json:"route_name"`
	FunctionName           string        `json:"function_name"`
	QueryName              string        `json:"query_name"`
	QueryOutput            string        `json:"query_output"`
	QueryOutputEncode      bool          `json:"query_output_encode"`
	Api                    TargetHost    `json:"api"`
	ApiPath                string        `json:"api_path"`
	Path                   string        `json:"path"`
	Route                  Route         `json:"-"`
	FuncKey                string        `json:"-"`
	ParentFuncGroupName    string        `json:"-"`
	FuncGroup              FuncGroup     `json:"-"`
	RequestHeaders         []Headers     `json:"request_headers"`
	QueryParams            []Headers     `json:"query_params"`
	FormData               []Headers     `json:"form_data"`
	FileData               []FilePart    `json:"file_data"`
	ResponseHeaders        []Headers     `json:"response_headers"`
	TransformRequest       string        `json:"transform_request"`
	TransformRequestOutput string        `json:"transform_request_output"`
	TransformResponse      string        `json:"transform_response"`
	//IsPublic             bool                 `json:"is_public"`
	RemoveParams         RemoveParams         `json:"remove_params"`
	FuncSteps            map[string]*FuncStep `json:"func_steps"`
	FsDb                 db.DbI               `json:"-"`
	RemoveRequestFields  []string             `json:"remove_request_fields"`
	RemoveResponseFields []string             `json:"remove_response_fields"`
}

func (funcGroup *FuncGroup) Clone(ctx context.Context) (cloneFuncGroup *FuncGroup, err error) {
	cloneFuncGroupI, cloneFuncGroupIErr := cloneInterface(ctx, funcGroup)
	if cloneFuncGroupIErr != nil {
		err = cloneFuncGroupIErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	cloneFuncGroupOk := false
	cloneFuncGroup, cloneFuncGroupOk = cloneFuncGroupI.(*FuncGroup)
	if !cloneFuncGroupOk {
		err = errors.New("FuncGroup cloning failed")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	for k, v := range funcGroup.FuncSteps {
		childFs, childFsErr := v.Clone(ctx)
		if childFsErr != nil {
			return
		}
		cloneFuncGroup.FuncSteps[k] = childFs
	}
	return
}
func (funcStep *FuncStep) Clone(ctx context.Context) (cloneFuncStep *FuncStep, err error) {
	cloneFuncStepI, cloneFuncStepIErr := cloneInterface(ctx, funcStep)
	if cloneFuncStepIErr != nil {
		err = cloneFuncStepIErr
		logs.WithContext(ctx).Error(cloneFuncStepIErr.Error())
		return
	}
	cloneFSk := false
	cloneFuncStep, cloneFSk = cloneFuncStepI.(*FuncStep)
	if !cloneFSk {
		err = errors.New("funcStep clone failed")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	routeClone, routeCloneErr := funcStep.Route.Clone(ctx)
	if routeCloneErr != nil {
		return
	}
	cloneFuncStep.Route = *routeClone
	cloneFuncStep.FuncKey = funcStep.FuncKey

	cloneFuncStep.AsyncEvent = funcStep.AsyncEvent
	cloneFuncStep.FsDb = funcStep.FsDb
	cloneFuncStep.ParentFuncGroupName = funcStep.ParentFuncGroupName

	funcGroupClone, funcGroupCloneErr := funcStep.FuncGroup.Clone(ctx)
	if funcGroupCloneErr != nil {
		return
	}
	cloneFuncStep.FuncGroup = *funcGroupClone
	cloneFuncStep.FuncKey = funcStep.FuncKey

	for k, v := range funcStep.FuncSteps {
		childFs, childFsErr := v.Clone(ctx)
		if childFsErr != nil {
			return
		}
		cloneFuncStep.FuncSteps[k] = childFs
	}

	return
}

func (funcGroup *FuncGroup) Execute(ctx context.Context, request *http.Request, FuncThreads int, LoopThreads int, funcStepName string, endFuncStepName string, fromAsync bool, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars) (response *http.Response, funcVarsMap map[string]FuncTemplateVars, err error) {
	logs.WithContext(ctx).Debug("FuncGroup Execute - Start")
	//reqVars := make(map[string]*TemplateVars)
	//resVars := make(map[string]*TemplateVars)
	response, funcVarsMap, _, err = RunFuncSteps(ctx, funcGroup.FuncSteps, request, reqVars, resVars, "", FuncThreads, LoopThreads, funcStepName, endFuncStepName, false, fromAsync, false)
	return
}

func RunFuncSteps(ctx context.Context, funcSteps map[string]*FuncStep, request *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, funcThreads int, loopThreads int, funcStepName string, endFuncStepName string, started bool, fromAsync bool, inLoop bool) (response *http.Response, funcVarsMap map[string]FuncTemplateVars, asyncFuncDataBatch []AsyncFuncData, err error) {
	logs.WithContext(ctx).Debug("RunFuncSteps - Start")
	funcVarsMap = make(map[string]FuncTemplateVars)
	var responses []*http.Response
	var errs []error
	//for _, cv := range funcSteps {
	//	response, err = cv.RunFuncStep(request, reqVars, resVars, mainRouteName)
	//}
	var funcJobs = make(chan FuncJob, 10)
	var funcResults = make(chan FuncResult, 10)
	//startTime := time.Now()
	go allocateFunc(ctx, request, funcSteps, reqVars, resVars, funcJobs, mainRouteName, funcThreads, loopThreads, funcStepName, endFuncStepName, started, fromAsync, inLoop)
	done := make(chan bool)

	go func(done chan bool, funcResults chan FuncResult) {
		defer func() {
			if r := recover(); r != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in RunFuncSteps: ", r, " : ", string(debug.Stack())))
			}
		}()
		for res := range funcResults {
			if res.response != nil {
				if res.response.ContentLength > 0 {
					logs.WithContext(ctx).Info(fmt.Sprint("adding response to array"))
					responses = append(responses, res.response)
				}
			}
			if res.asyncFuncDataBatch != nil {
				asyncFuncDataBatch = append(asyncFuncDataBatch, res.asyncFuncDataBatch...)
			}

			for k, v := range res.responseVarsMap {
				funcVarsMap[k] = v
			}

			if res.responseErr != nil {
				errs = append(errs, res.responseErr)
			}
		}
		done <- true
	}(done, funcResults)

	//set it to one to run synchronously
	noOfWorkers := funcThreads
	if len(funcSteps) < noOfWorkers {
		noOfWorkers = len(funcSteps)
	}

	createWorkerPoolFunc(ctx, noOfWorkers, funcJobs, funcResults)
	<-done
	//endTime := time.Now()
	//diff := endTime.Sub(startTime)
	//logs.WithContext(ctx).Info(fmt.Sprint("total time taken ", diff.Seconds(), "seconds"))
	response, err = clubResponses(ctx, responses, errs)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint(err.Error()))
	}
	eru_utils.PrintResponseBody(ctx, response, "after club responses")
	return
}

func (funcStep *FuncStep) GetRouteName() (routeName string) {
	if funcStep.FunctionName != "" {
		routeName = funcStep.FunctionName
	} else if funcStep.QueryName != "" {
		routeName = funcStep.QueryName
	} else if funcStep.Api.Host != "" {
		routeName = strings.Replace(strings.Replace(funcStep.Api.Host, ".", "", -1), ":", "", -1)
	} else if funcStep.RouteName != "" {
		routeName = funcStep.RouteName
	}
	return
}

func (funcStep *FuncStep) RunFuncStep(octx context.Context, req *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, FuncThread int, LoopThread int, funcStepName string, endFuncStepName string, started bool, fromAsync bool, inLoop bool) (response *http.Response, funcVarsMap map[string]FuncTemplateVars, asyncFuncDataBatch []AsyncFuncData, err error) {
	pspan := oteltrace.SpanFromContext(req.Context())
	ctx, span := otel.Tracer(server_handlers.ServerName).Start(octx, funcStep.FuncKey, oteltrace.WithAttributes(attribute.String("requestID", req.Header.Get(server_handlers.RequestIdKey)), attribute.String("traceID", pspan.SpanContext().TraceID().String()), attribute.String("spanID", pspan.SpanContext().SpanID().String())))
	defer span.End()
	ctx = logs.NewContext(ctx, zap.String("funcStepName", funcStep.FuncKey))
	req = req.WithContext(ctx)
	request := req
	var loopArray []interface{}
	asyncMessage := ""
	var responses []*http.Response
	var errs []error
	var vars *TemplateVars
	strCond := "true"
	logs.WithContext(ctx).Info(fmt.Sprint("started from RunFuncStep for , ", funcStep.FuncKey, " = ", started))

	if funcStep.FuncKey == funcStepName || started || funcStepName == "" {
		//started = true

		//first step is to transform the request which in turn will clone the request before transforming keeping original request as is for further use.
		request, vars, err = funcStep.transformRequest(ctx, req, reqVars, resVars, mainRouteName, true, fromAsync, funcStepName)
		if err != nil {
			logs.WithContext(ctx).Info("inside error of transformRequest of RunFuncStep")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		reqVars[funcStep.GetRouteName()] = vars
		reqVars[funcStep.FuncKey] = vars
		var strCondErr error

		if funcStep.Condition != "" {
			avars := &FuncTemplateVars{}
			avars.Vars = reqVars[funcStep.FuncKey]
			avars.ResVars = resVars
			avars.ReqVars = reqVars

			output, outputErr := processTemplate(ctx, funcStep.FuncKey, funcStep.Condition, avars, "string", funcStep.Route.TokenSecretKey)
			if outputErr != nil {
				err = outputErr
				response = errorResponse(ctx, err.Error(), request)
				return
			}
			strCond, strCondErr = strconv.Unquote(string(output))
			if strCondErr != nil {
				err = strCondErr
				logs.WithContext(ctx).Error(err.Error())
				response = errorResponse(ctx, err.Error(), request)
				return
			}
			logs.WithContext(ctx).Info(fmt.Sprint("strCond == ", strCond))
			if strCond == "false" && (funcStep.ConditionFailAction == ConditionFailActionError || funcStep.ConditionFailAction == ConditionFailActionStop || (funcStep.ConditionFailAction == ConditionFailActionIgnore) && len(funcStep.FuncSteps) == 0) {
				cfmBody := "{}"
				if funcStep.ConditionFailMessage != "" {
					cfmvars := &FuncTemplateVars{}
					cfmvars.Vars = reqVars[funcStep.FuncKey]
					cfmOutput, cfmOutputErr := processTemplate(ctx, funcStep.FuncKey, funcStep.ConditionFailMessage, avars, "json", funcStep.Route.TokenSecretKey)
					logs.WithContext(ctx).Info(string(cfmOutput))
					if cfmOutputErr != nil {
						err = cfmOutputErr
						response = errorResponse(ctx, err.Error(), request)
						return
					}
					cfmBody = string(cfmOutput)
				}
				statusCode := http.StatusOK
				if funcStep.ConditionFailAction == ConditionFailActionError {
					statusCode = http.StatusBadRequest
				}

				condRespHeader := http.Header{}
				condRespHeader.Set("Content-Type", "application/json")
				response = &http.Response{
					StatusCode:    statusCode,
					Proto:         "HTTP/1.1",
					ProtoMajor:    1,
					ProtoMinor:    1,
					Body:          io.NopCloser(bytes.NewBufferString(cfmBody)),
					ContentLength: int64(len(cfmBody)),
					Request:       request,
					Header:        condRespHeader,
				}
				responses = append(responses, response)
				if err != nil {
					logs.WithContext(ctx).Error(fmt.Sprint("error for  false condition : ", err.Error()))
				}
				return
			}
		}
		if strCond == "true" {
			if (!fromAsync || (fromAsync && funcStepName != funcStep.FuncKey)) && funcStep.Async && funcStep.AsyncMessage != "" && funcStep.LoopVariable == "" {
				avars := &FuncTemplateVars{}
				avars.Vars = reqVars[funcStep.FuncKey]
				avars.ReqVars = reqVars
				avars.ResVars = resVars
				output, outputErr := processTemplate(ctx, funcStep.FuncKey, funcStep.AsyncMessage, avars, "json", funcStep.Route.TokenSecretKey)
				if len(string(output)) < 1000 {
					logs.WithContext(ctx).Info(string(output))
				}
				if outputErr != nil {
					err = outputErr
					response = errorResponse(ctx, err.Error(), request)
					return
				}
				asyncMessage = string(output)

				asyncRespHeader := http.Header{}
				asyncRespHeader.Set("Content-Type", "application/json")
				response = &http.Response{
					StatusCode:    http.StatusOK,
					Proto:         "HTTP/1.1",
					ProtoMajor:    1,
					ProtoMinor:    1,
					Body:          io.NopCloser(bytes.NewBufferString(asyncMessage)),
					ContentLength: int64(len(asyncMessage)),
					Request:       request,
					Header:        asyncRespHeader,
				}
				responses = append(responses, response)

				async_id := uuid.New().String()
				msgId := ""

				eventMsgBytes := []byte("")
				_ = eventMsgBytes
				eventMsgRequest := []byte("")
				if vars.Headers != nil {
					requestId := vars.Headers["Request_id"]
					if requestIdSlice, requestIdSliceok := requestId.([]string); requestIdSliceok {
						eventMsgRequest = []byte(requestIdSlice[0])
					}
				}
				var requestBytes = &bytes.Buffer{}
				err = request.Write(requestBytes)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return
				}
				requestStr := b64.StdEncoding.EncodeToString(requestBytes.Bytes())

				eventMsgBytes, err = json.Marshal(avars)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return
				}

				if inLoop {
					asyncFuncData := AsyncFuncData{}
					asyncFuncData.FuncName = funcStep.ParentFuncGroupName
					asyncFuncData.FuncStepName = funcStep.FuncKey
					asyncFuncData.AsyncId = async_id
					asyncFuncData.EventMsg = *avars
					asyncFuncData.EventRequest = requestStr
					asyncFuncData.RequestId = string(eventMsgRequest)
					asyncFuncData.EventId = msgId
					asyncFuncDataBatch = append(asyncFuncDataBatch, asyncFuncData)
				} else {
					msgId, err = funcStep.AsyncEvent.Publish(ctx, async_id, funcStep.AsyncEvent)
					if err != nil {
						return
					}

					var insertQueries []*models.Queries
					insertQueryFuncAsync := models.Queries{}
					insertQueryFuncAsync.Query = funcStep.FsDb.GetDbQuery(ctx, INSERT_FUNC_ASYNC)

					insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, async_id, msgId, funcStep.ParentFuncGroupName, funcStep.FuncKey, string(eventMsgBytes), string(eventMsgRequest), requestStr)
					insertQueryFuncAsync.Rank = 1
					insertQueries = append(insertQueries, &insertQueryFuncAsync)
					logs.WithContext(ctx).Info("calling async insert from here")
					_, insertOutputErr := eru_utils.ExecuteDbSave(ctx, funcStep.FsDb.GetConn(), insertQueries)
					if insertOutputErr != nil {
						err = insertOutputErr
						logs.WithContext(ctx).Error(err.Error())
						return
					}
				}
				return
			}

			lerr := false
			if (!fromAsync && funcStep.LoopVariable != "") || (fromAsync && funcStep.FuncKey != funcStepName && funcStep.LoopVariable != "") {
				loopArray, lerr = vars.LoopVars.([]interface{})
				if !lerr {
					err = errors.New("func loop variable is not an array")
					logs.WithContext(ctx).Error(err.Error())
					response = errorResponse(ctx, err.Error(), request)
					return
				}
			}
		}
	}

	if len(loopArray) == 0 {
		//dummy row added to create a job
		loopArray = append(loopArray, make(map[string]interface{}))
	}

	var jobs = make(chan FuncJob, 10)
	var results = make(chan FuncResult, 10)
	startTime := time.Now()

	//adding delay
	time.Sleep(time.Duration(funcStep.Delay) * time.Millisecond)

	go allocateFuncInner(ctx, request, funcStep, reqVars, resVars, loopArray, asyncMessage, jobs, mainRouteName, FuncThread, LoopThread, strCond, funcStepName, endFuncStepName, started, fromAsync, inLoop)
	done := make(chan bool)
	//go result(done,results,responses, trResVars,errs)
	funcVarsMap = make(map[string]FuncTemplateVars)

	go func(done chan bool, results chan FuncResult) {
		defer func() {
			if r := recover(); r != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in RunFuncStep: ", r, " : ", string(debug.Stack())))
			}
		}()
		var asyncBatch []AsyncFuncData
		for res := range results {
			if res.response != nil {
				responses = append(responses, res.response)
			}
			funcVarsMap[funcStep.FuncKey] = res.responseVars
			if res.responseErr != nil {
				errs = append(errs, res.responseErr)
			}
			if res.asyncFuncDataBatch != nil {
				asyncBatch = append(asyncBatch, res.asyncFuncDataBatch...)
			}
			if len(asyncBatch) == 10 {
				logs.WithContext(ctx).Info(fmt.Sprint("calling async insert for batch size ", len(asyncBatch)))
				err = funcStep.insertAsyncBatch(ctx, asyncBatch)
				asyncBatch = nil
			}
		}
		if len(asyncBatch) > 0 {
			logs.WithContext(ctx).Info(fmt.Sprint("calling residual async insert for batch size ", len(asyncBatch)))
			err = funcStep.insertAsyncBatch(ctx, asyncBatch)
		}
		done <- true
	}(done, results)

	//set it to one to run synchronously - change it if LoopInParallel is true to run in parallel
	noOfWorkers := 1
	if funcStep.LoopInParallel && funcStep.LoopVariable != "" {
		noOfWorkers = LoopThread
		if len(loopArray) < noOfWorkers {
			noOfWorkers = len(loopArray)
		}
	}
	createWorkerPoolFuncInner(ctx, noOfWorkers, jobs, results)
	<-done
	response, err = clubResponses(ctx, responses, errs)
	endTime := time.Now()
	diff := endTime.Sub(startTime)
	logs.WithContext(ctx).Info(fmt.Sprint("total time taken ", diff.Seconds(), "seconds"))
	logs.WithContext(ctx).Info(fmt.Sprint("RunFuncStep - End : ", funcStep.FuncKey))

	return
}
func (funcStep *FuncStep) RunFuncStepInner(ctx context.Context, req *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, asyncMsg string, funcThread int, loopThread int, strCond string, funcStepName string, endFuncStepName string, started bool, fromAsync bool, inLoop bool) (response *http.Response, funcVars FuncTemplateVars, asyncFuncDataBatch []AsyncFuncData, err error) {
	logs.WithContext(ctx).Info(fmt.Sprint("RunFuncStepInner - Start : ", funcStep.FuncKey))
	request := req
	if started || funcStepName == "" || funcStepName == funcStep.FuncKey {
		if strCond == "true" {
			if funcStep.LoopVariable != "" {
				request, _, err = funcStep.transformRequest(ctx, req, reqVars, resVars, mainRouteName, false, fromAsync, funcStepName)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return
				}
			}

			/////////////////////////
			asyncMessage := ""
			if (!fromAsync || (fromAsync && funcStepName != funcStep.FuncKey)) && funcStep.Async && funcStep.AsyncMessage != "" {
				avars := &FuncTemplateVars{}
				avars.Vars = reqVars[funcStep.FuncKey]
				avars.ReqVars = reqVars
				avars.ResVars = resVars
				output, outputErr := processTemplate(ctx, funcStep.FuncKey, funcStep.AsyncMessage, avars, "json", funcStep.Route.TokenSecretKey)
				if outputErr != nil {
					err = outputErr
					response = errorResponse(ctx, err.Error(), request)
					return
				}
				asyncMessage = string(output)

				asyncRespHeader := http.Header{}
				asyncRespHeader.Set("Content-Type", "application/json")
				response = &http.Response{
					StatusCode:    http.StatusOK,
					Proto:         "HTTP/1.1",
					ProtoMajor:    1,
					ProtoMinor:    1,
					Body:          io.NopCloser(bytes.NewBufferString(asyncMessage)),
					ContentLength: int64(len(asyncMessage)),
					Request:       request,
					Header:        asyncRespHeader,
				}

				async_id := uuid.New().String()
				msgId := ""

				//msgId, err = funcStep.AsyncEvent.Publish(ctx, async_id, funcStep.AsyncEvent)
				//if err != nil {
				//	return
				//}
				//eventMsgBytes := []byte("")
				//_ = eventMsgBytes
				eventMsgRequest := []byte("")
				if avars.Vars.Headers != nil {
					requestId := avars.Vars.Headers["Request_id"]
					if requestIdSlice, requestIdSliceok := requestId.([]string); requestIdSliceok {
						eventMsgRequest = []byte(requestIdSlice[0])
					}
				}

				var requestBytes = &bytes.Buffer{}
				err = req.Write(requestBytes)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return
				}
				requestStr := b64.StdEncoding.EncodeToString(requestBytes.Bytes())

				//eventMsgBytes, err = json.Marshal(avars)
				//if err != nil {
				//	logs.WithContext(ctx).Error(err.Error())
				//	return
				//}

				asyncFuncData := AsyncFuncData{}
				asyncFuncData.FuncName = funcStep.ParentFuncGroupName
				asyncFuncData.FuncStepName = funcStep.FuncKey
				asyncFuncData.AsyncId = async_id
				asyncFuncData.EventMsg = *avars
				asyncFuncData.EventRequest = requestStr
				asyncFuncData.RequestId = string(eventMsgRequest)
				asyncFuncData.EventId = msgId
				asyncFuncDataBatch = append(asyncFuncDataBatch, asyncFuncData)
				//var insertQueries []*models.Queries
				//insertQueryFuncAsync := models.Queries{}
				//insertQueryFuncAsync.Query = funcStep.FsDb.GetDbQuery(ctx, INSERT_FUNC_ASYNC)

				//insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, async_id, msgId, funcStep.ParentFuncGroupName, funcStep.FuncKey, string(eventMsgBytes), string(eventMsgRequest), requestStr)
				//insertQueryFuncAsync.Rank = 1
				//insertQueries = append(insertQueries, &insertQueryFuncAsync)
				//_, insertOutputErr := eru_utils.ExecuteDbSave(ctx, funcStep.FsDb.GetConn(), insertQueries)
				//if insertOutputErr != nil {
				//	err = insertOutputErr
				//	logs.WithContext(ctx).Error(err.Error())
				//	return
				//}
				return
			}
			////////////////////////////

			routevars := &TemplateVars{}
			_ = routevars
			subFuncVarsMap := map[string]FuncTemplateVars{}
			if funcStep.FunctionName != "" {
				asyncInnerFuncData := []AsyncFuncData{}
				_ = asyncInnerFuncData
				response, subFuncVarsMap, asyncInnerFuncData, err = RunFuncSteps(ctx, funcStep.FuncGroup.FuncSteps, request, reqVars, resVars, "", funcThread, loopThread, funcStep.FuncKey, "", started, fromAsync, inLoop)
				if asyncInnerFuncData != nil {
					asyncFuncDataBatch = append(asyncFuncDataBatch, asyncInnerFuncData...)
				}
				for _, v := range subFuncVarsMap {
					for kk, vv := range v.ResVars {
						resVars[fmt.Sprint(funcStep.FuncKey, ".", kk)] = vv
					}
					for kk, vv := range v.ReqVars {
						reqVars[fmt.Sprint(funcStep.FuncKey, ".", kk)] = vv
					}
				}
			} else {
				response, routevars, err = funcStep.Route.Execute(ctx, request, funcStep.Path, funcStep.Async, asyncMsg, reqVars[funcStep.FuncKey], loopThread)
			}

			resVars[funcStep.GetRouteName()] = routevars
			resVars[funcStep.FuncKey] = routevars
			if funcStep.Route.OnError == "STOP" && response.StatusCode >= 400 {
				logs.WithContext(ctx).Info("inside funcStep.Route.OnError == \"STOP\" && response.StatusCode >= 400")
				return
			} else {
				if err != nil {
					logs.WithContext(ctx).Error(fmt.Sprint("Ignoring route execution error : ", err.Error()))
					err = nil
					cfmBody := "{}"
					response = &http.Response{
						StatusCode:    http.StatusOK,
						Proto:         "HTTP/1.1",
						ProtoMajor:    1,
						ProtoMinor:    1,
						Body:          io.NopCloser(bytes.NewBufferString(cfmBody)),
						ContentLength: int64(len(cfmBody)),
						Request:       request,
						Header:        http.Header{},
					}
				}
			}

			// in case of error - no need to call  transformResponse
			if err == nil {
				var trespErr error
				var trVars *TemplateVars
				trVars, trespErr = funcStep.transformResponse(ctx, response, resVars[funcStep.FuncKey], reqVars, resVars)
				if trespErr != nil {
					err = trespErr
					return
				}
				resVars[funcStep.GetRouteName()] = trVars
				resVars[funcStep.FuncKey] = trVars
			}
			if funcStep.Route.Redirect {
				logs.WithContext(ctx).Info(fmt.Sprint("Redirect URl = ", funcStep.Route.FinalRedirectUrl))
				response.StatusCode = http.StatusSeeOther
				response.Header.Set("Location", funcStep.Route.FinalRedirectUrl)
				return
			}
		}
	}

	if len(funcStep.FuncSteps) > 0 && funcStep.FuncKey != endFuncStepName {
		childFuncVarsMap := map[string]FuncTemplateVars{}

		//remove unwanted fields from reqvars and resvars
		err = removeFieldsFromTemplateVars(ctx, funcStep.RemoveRequestFields, reqVars)
		err = removeFieldsFromTemplateVars(ctx, funcStep.RemoveResponseFields, resVars)

		if funcStep.LoopVariable != "" {
			inLoop = true
		}
		asyncChilFuncData := []AsyncFuncData{}
		response, childFuncVarsMap, asyncChilFuncData, err = RunFuncSteps(ctx, funcStep.FuncSteps, request, reqVars, resVars, mainRouteName, funcThread, loopThread, funcStepName, endFuncStepName, started, fromAsync, inLoop)
		for _, v := range childFuncVarsMap {
			for kk, vv := range v.ResVars {
				resVars[kk] = vv
			}
			for kk, vv := range v.ReqVars {
				reqVars[kk] = vv
			}
		}
		if asyncChilFuncData != nil {
			asyncFuncDataBatch = append(asyncFuncDataBatch, asyncChilFuncData...)
		}
	}

	funcVars.ResVars = resVars
	funcVars.ReqVars = reqVars
	return
}
func (funcStep *FuncStep) insertAsyncBatch(ctx context.Context, asyncBatch []AsyncFuncData) (err error) {
	logs.WithContext(ctx).Debug("insertAsyncBatch - Start")
	var valueStrings []string
	asyncIds := []string{}
	for i, asyncFuncData := range asyncBatch {
		asyncIds = append(asyncIds, asyncFuncData.AsyncId)
		if i > 0 {
			valueStrings = append(valueStrings, " (???,???,???,???,???,???,???) ")
		}
	}
	stmt := INSERT_FUNC_ASYNC
	if len(valueStrings) > 0 {
		stmt = fmt.Sprint(stmt, " , ", strings.Join(valueStrings, ","))
	}

	var insertQueries []*models.Queries
	insertQueryFuncAsync := models.Queries{}
	insertQueryFuncAsync.Query = funcStep.FsDb.GetDbQuery(ctx, stmt)

	batch_id := uuid.New().String()
	for _, asyncFuncData := range asyncBatch {
		asyncFuncData.EventId = batch_id
		eventMsgBytes, eventErr := json.Marshal(asyncFuncData.EventMsg)
		if eventErr != nil {
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, asyncFuncData.AsyncId, asyncFuncData.EventId, asyncFuncData.FuncName, asyncFuncData.FuncStepName, string(eventMsgBytes), asyncFuncData.RequestId, asyncFuncData.EventRequest)
	}

	insertQueryFuncAsync.Rank = 1
	insertQueries = append(insertQueries, &insertQueryFuncAsync)
	_, insertOutputErr := eru_utils.ExecuteDbSave(ctx, funcStep.FsDb.GetConn(), insertQueries)
	if insertOutputErr != nil {
		err = insertOutputErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	msgId := ""
	_ = msgId
	msgId, err = funcStep.AsyncEvent.Publish(ctx, strings.Join(asyncIds, ","), funcStep.AsyncEvent)
	if err != nil {
		return
	}

	return
}
func (funcStep *FuncStep) transformRequest(ctx context.Context, request *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, supressTemplateErrors bool, fromAsync bool, funcStepName string) (req *http.Request, vars *TemplateVars, err error) {
	logs.WithContext(ctx).Debug("transformRequest - Start")
	var tErrs []string
	//first step in transforming is to make a clone of the original request
	req, err = CloneRequest(ctx, request)
	if err != nil {
		return
	}

	// next we read the request and loads vars to be used for transforming request
	// we load vars only once for one thread of func step - thus the check of mainroutename
	vars = &TemplateVars{}
	vars.FormData = make(map[string]interface{})
	vars.Body = make(map[string]interface{})
	vars.OrgBody = make(map[string]interface{})
	vars.Params = make(map[string]interface{})
	logs.WithContext(ctx).Info(fmt.Sprint("mainRouteName for ", funcStep.FuncKey, " = ", mainRouteName))
	if fromAsync && funcStep.FuncKey == funcStepName {
		v, _ := cloneInterface(ctx, reqVars[funcStep.FuncKey])
		vars = v.(*TemplateVars)
	} else if reqVars[mainRouteName] == nil {
		err = loadRequestVars(ctx, vars, req, funcStep.Route.TokenSecretKey)
		if err != nil {
			return
		}
	} else {
		v, _ := cloneInterface(ctx, reqVars[mainRouteName])
		vars = v.(*TemplateVars)
	}
	//utils.PrintRequestBody(req, "printing request in transformRequest")
	var loopArray []interface{}
	if funcStep.LoopVariable != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = vars
		fvars.ResVars = resVars
		fvars.ReqVars = reqVars

		output, outputErr := processTemplate(ctx, funcStep.FuncKey, funcStep.LoopVariable, fvars, "json", funcStep.Route.TokenSecretKey)
		if outputErr != nil {
			tErrs = append(tErrs, outputErr.Error())
		}
		var loopJson interface{}
		loopJsonErr := json.Unmarshal(output, &loopJson)
		if loopJsonErr != nil {
			err = errors.New("func loop variable is not a json")
			logs.WithContext(ctx).Error(loopJsonErr.Error())
		}

		ok := false
		if loopArray, ok = loopJson.([]interface{}); !ok {
			err = errors.New("func loop variable is not an array")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	} else {
		//dummy row added to create a job
		loopArray = append(loopArray, make(map[string]interface{}))
	}
	vars.LoopVars = loopArray
	// setting new headers in request which are not template based - direct values.
	oldContentTypeFull := req.Header.Get("Content-type")
	oldContentType := strings.Split(oldContentTypeFull, ";")[0]
	for _, h := range funcStep.RequestHeaders {
		if !h.IsTemplate {
			req.Header.Set(h.Key, h.Value)
		}
	}
	newContentTypeFull := req.Header.Get("Content-type")
	newContentType := strings.Split(newContentTypeFull, ";")[0]

	//first check if original request is not multipart but the new request to be forwarded to target host is multipart - then make multipart body from json body
	// else if original request is multipart/form , we process the same
	makeMultiPartCalled := false
	if (newContentType == encodedForm || newContentType == multiPartForm) && newContentType != oldContentType {
		vars.FormData, vars.FormDataKeyArray, err = makeMultipart(ctx, req, funcStep.FormData, funcStep.FileData, vars, reqVars, resVars, funcStep.Route.TokenSecretKey)
		makeMultiPartCalled = true
		if err != nil {
			return
		}
	} else if oldContentType == multiPartForm || oldContentType == encodedForm {
		makeMultiPartCalled = true
		mpvars := &FuncTemplateVars{}
		mpvars.ResVars = resVars
		mpvars.ReqVars = reqVars
		mpvars.Vars = vars
		for _, fd := range funcStep.FormData {
			if fd.IsTemplate {
				output, fdErr := processTemplate(ctx, fd.Key, fd.Value, mpvars, "string", funcStep.Route.TokenSecretKey)
				if fdErr != nil {
					tErrs = append(tErrs, fdErr.Error())
				}
				outputStr, fduErr := strconv.Unquote(string(output))
				if fduErr != nil {
					err = fduErr
					logs.WithContext(ctx).Error(err.Error())
					return
				}
				vars.FormData[fd.Key] = outputStr
				vars.FormDataKeyArray = append(vars.FormDataKeyArray, fd.Key)
				//commented as this was getting stored in store and picked up in next request
				//funcStep.FormData[i].Value = outputStr
			} else {
				vars.FormData[fd.Key] = fd.Value
				vars.FormDataKeyArray = append(vars.FormDataKeyArray, fd.Key)
			}
		}
		if oldContentType == multiPartForm {
			//resetting it back to old content type as processMultipart will not be able to read the request body
			req.Header.Set("Content-type", oldContentTypeFull)
			vars.FormData, vars.FormDataKeyArray, vars.FileData, err = processMultipart(ctx, oldContentType, req, funcStep.RemoveParams.FormData, vars.FormData, vars.FileData)
			if err != nil {
				return
			}
			//changing it back to new content type once process multipart has read the request body and loaded vars.formdata
			req.Header.Set("Content-type", newContentTypeFull)
		} else if oldContentType == encodedForm {
			rpfErr := request.ParseForm()
			if rpfErr != nil {
				err = rpfErr
				logs.WithContext(ctx).Info(fmt.Sprint("error from request.ParseForm() = ", err.Error()))
				return
			}
			if request.PostForm != nil {
				for k, v := range request.PostForm {
					vars.FormData[k] = strings.Join(v, ",")
					vars.FormDataKeyArray = append(vars.FormDataKeyArray, k)
				}
			}
		}
	}

	//next we process and transform query params and set it in request
	var ppErrs []string
	err, ppErrs = processParams(ctx, req, funcStep.RemoveParams.QueryParams, funcStep.QueryParams, vars, reqVars, resVars, funcStep.Route.TokenSecretKey)
	if err != nil {
		return
	}
	if ppErrs != nil {
		tErrs = append(tErrs, ppErrs...)
	}
	//utils.PrintRequestBody(req, "printing request in transformRequest after processParams")
	//next we process and transform request body only if it is not multipart and set it in request
	if funcStep.TransformRequest != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = vars
		fvars.ResVars = resVars
		fvars.ReqVars = reqVars
		ot := "json"
		if funcStep.TransformRequestOutput != "" {
			ot = funcStep.TransformRequestOutput
		}

		output, oErr := processTemplate(ctx, funcStep.FuncKey, funcStep.TransformRequest, fvars, ot, funcStep.Route.TokenSecretKey)
		if oErr != nil {
			tErrs = append(tErrs, oErr.Error())
		}
		if string(output) != "" {
			err = json.Unmarshal(output, &vars.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return req, &TemplateVars{}, err
			}
		}
		req.Body = io.NopCloser(bytes.NewBuffer(output))
		req.Header.Set("Content-Length", strconv.Itoa(len(output)))
		req.ContentLength = int64(len(output))

	} else if !makeMultiPartCalled && req.ContentLength > 0 {
		rb, err1 := json.Marshal(vars.Body)
		if err1 != nil {
			err = err1
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		req.Body = io.NopCloser(bytes.NewBuffer(rb))
		req.Header.Set("Content-Length", strconv.Itoa(len(rb)))
		req.ContentLength = int64(len(rb))
	}
	//utils.PrintRequestBody(req, "printing request in transformRequest after body process")

	//check and transform api host if set as template
	for rk, ro := range funcStep.Route.TargetHosts {
		if strings.HasPrefix(ro.Host, "{{") {
			avars := &FuncTemplateVars{}
			avars.Vars = vars
			avars.ResVars = resVars
			avars.ReqVars = reqVars

			output, thErr := processTemplate(ctx, "api_host", ro.Host, avars, "string", funcStep.Route.TokenSecretKey)
			if thErr != nil {
				// ignore error if it is no value
				if thErr.Error() != "Template returned <no value>" {
					tErrs = append(tErrs, thErr.Error())
				}
			}
			if string(output) != "" {
				host, hErr := strconv.Unquote(string(output))
				if hErr != nil {
					logs.WithContext(ctx).Info(hErr.Error())
					host = string(output)
				}

				hostArray := strings.Split(host, "://")
				if len(hostArray) > 1 {
					host = hostArray[1]
					funcStep.Route.TargetHosts[rk].Scheme = hostArray[0]
				} else {
					host = hostArray[0]
				}

				hostPathArray := strings.Split(host, "/")
				funcStep.Route.TargetHosts[rk].Host = hostPathArray[0]
				if len(hostPathArray) > 1 {
					funcStep.Route.RewriteUrl = fmt.Sprint("/", strings.Join(hostPathArray[1:], "/"))
				}
			}
		}
	}

	if strings.HasPrefix(funcStep.ApiPath, "{{") {
		avars := &FuncTemplateVars{}
		avars.Vars = vars
		avars.ResVars = resVars
		avars.ReqVars = reqVars
		output, apErr := processTemplate(ctx, "api_host", funcStep.ApiPath, avars, "string", funcStep.Route.TokenSecretKey)
		if apErr != nil {
			// ignore error if it is no value
			if apErr.Error() != "Template returned <no value>" {
				tErrs = append(tErrs, apErr.Error())
			}
		}
		if string(output) != "" {
			path, pErr := strconv.Unquote(string(output))
			if pErr != nil {
				logs.WithContext(ctx).Info(pErr.Error())
				path = string(output)
			}
			funcStep.Route.RewriteUrl = path
		}
	}

	// lastly we process and transform template based headers and set it in request
	var hErrs []string
	err, hErrs = processHeaderTemplates(ctx, req, funcStep.RemoveParams.RequestHeaders, funcStep.RequestHeaders, false, vars, funcStep.Route.TokenSecretKey, reqVars, resVars)
	if err != nil {
		return
	}
	tErrs = append(tErrs, hErrs...)
	//set cookies from previous steps
	for _, v := range resVars {
		for _, c := range v.Cookies {
			req.AddCookie(c)
		}
	}
	if err != nil {
		logs.WithContext(ctx).Info(err.Error())
		tErrs = append(tErrs, err.Error())
	}
	if len(tErrs) > 0 && !supressTemplateErrors {
		err = errors.New(strings.Join(tErrs, " ; "))
	}
	return req, vars, err
}

func (funcStep *FuncStep) transformResponse(ctx context.Context, response *http.Response, trResVars *TemplateVars, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars) (vars *TemplateVars, err error) {
	logs.WithContext(ctx).Debug("transformResponse - Start")
	//utils.PrintResponseBody(response, "response printed from inside funcStep transformResponse")
	vars = trResVars
	for _, h := range funcStep.ResponseHeaders {
		response.Header.Set(h.Key, h.Value)
	}
	logs.WithContext(ctx).Info(fmt.Sprint("TransformResponse = ", funcStep.TransformResponse))

	if vars.Headers == nil {
		vars.Headers = make(map[string]interface{})
	}
	for k, v := range response.Header {
		vars.Headers[k] = v
	}

	if vars.Params == nil {
		vars.Params = make(map[string]interface{})
	}
	if vars.Vars == nil {
		vars.Vars = make(map[string]interface{})
	}
	reqContentType := strings.Split(response.Header.Get("Content-type"), ";")[0]
	if reqContentType == "application/json" {

		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			err = readErr
			logs.WithContext(ctx).Error(fmt.Sprint("io.ReadAll(response.Body) error : ", err.Error()))
			return
		}

		tmplBodyFromRes := json.NewDecoder(bytes.NewReader(body))
		tmplBodyFromRes.DisallowUnknownFields()

		//if err = json.Unmarshal(body, &vars.Body); err != nil {

		if err = tmplBodyFromRes.Decode(&vars.Body); err != nil {

			err = nil
			tempBody := make(map[string]string)

			tempBody["data"] = strings.TrimSpace(string(body))
			vars.Body = tempBody
		}
		vars.OrgBody = vars.Body
		if funcStep.TransformResponse != "" {
			fvars := &FuncTemplateVars{}
			fvars.Vars = vars
			fvars.ResVars = resVars
			fvars.ReqVars = reqVars

			output, err := processTemplate(ctx, funcStep.FuncKey, funcStep.TransformResponse, fvars, "json", funcStep.Route.TokenSecretKey)
			if err != nil {
				return &TemplateVars{}, err
			}
			response.Body = io.NopCloser(bytes.NewBuffer(output))
			response.Header.Set("Content-Length", strconv.Itoa(len(output)))
			response.ContentLength = int64(len(output))
			err = json.Unmarshal(output, &vars.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return &TemplateVars{}, err
			}
		} else {
			rb, err := json.Marshal(vars.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return &TemplateVars{}, err
			}
			response.Body = io.NopCloser(bytes.NewReader(rb))
			response.Header.Set("Content-Length", strconv.Itoa(len(rb)))
			response.ContentLength = int64(len(rb))
		}
	}

	if funcStep.RemoveParams.ResponseHeaders != nil {
		for _, v := range funcStep.RemoveParams.ResponseHeaders {
			response.Header.Del(v)
		}
	}
	vars.Cookies = response.Cookies()
	return
}
