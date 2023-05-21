package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type FuncGroup struct {
	FuncCategoryName string
	FuncGroupName    string
	FuncSteps        map[string]*FuncStep //routename is the key
	TokenSecret      TokenSecret          `json:"-"`
}

type FuncTemplateVars struct {
	Vars    *TemplateVars
	ReqVars map[string]*TemplateVars
	ResVars map[string]*TemplateVars
}

type FuncStep struct {
	Condition            string
	ConditionFailMessage string
	ConditionFailAction  string
	Async                bool
	AsyncMessage         string
	LoopVariable         string
	LoopInParallel       bool
	RouteName            string
	FunctionName         string
	QueryName            string
	QueryOutput          string
	Api                  TargetHost
	ApiPath              string
	Path                 string
	Route                Route     `json:"-"`
	FuncGroup            FuncGroup `json:"-"`
	RequestHeaders       []Headers
	QueryParams          []Headers
	FormData             []Headers
	FileData             []FilePart
	ResponseHeaders      []Headers
	TransformRequest     string
	TransformResponse    string
	IsPublic             bool
	RemoveParams         RemoveParams
	FuncSteps            map[string]*FuncStep
}

func (funcGroup *FuncGroup) Execute(ctx context.Context, request *http.Request, FuncThreads int, LoopThreads int) (response *http.Response, err error) {
	logs.WithContext(ctx).Debug("FuncGroup Execute - Start")
	reqVars := make(map[string]*TemplateVars)
	resVars := make(map[string]*TemplateVars)
	response, err = RunFuncSteps(ctx, funcGroup.FuncSteps, request, reqVars, resVars, "", FuncThreads, LoopThreads)
	return
}

func RunFuncSteps(ctx context.Context, funcSteps map[string]*FuncStep, request *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, funcThreads int, loopThreads int) (response *http.Response, err error) {
	logs.WithContext(ctx).Debug("RunFuncSteps - Start")
	var responses []*http.Response
	var errs []error

	//for _, cv := range funcSteps {
	//	response, err = cv.RunFuncStep(request, reqVars, resVars, mainRouteName)
	//}

	var funcJobs = make(chan FuncJob, 10)
	var funcResults = make(chan FuncResult, 10)
	startTime := time.Now()
	go allocateFunc(ctx, request, funcSteps, reqVars, resVars, funcJobs, mainRouteName, funcThreads, loopThreads)
	done := make(chan bool)

	go func(done chan bool, funcResults chan FuncResult) {
		defer func() {
			if r := recover(); r != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in RunFuncSteps: ", r))
			}
		}()
		for res := range funcResults {
			responses = append(responses, res.response)
			//trResVars = append(trResVars, res.responseVars)
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
	endTime := time.Now()
	diff := endTime.Sub(startTime)
	logs.WithContext(ctx).Info(fmt.Sprint("total time taken ", diff.Seconds(), "seconds"))
	response, _, err = clubResponses(ctx, responses, nil, errs)

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
func (funcStep *FuncStep) RunFuncStep(octx context.Context, req *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, FuncThread int, LoopThread int) (response *http.Response, err error) {
	pspan := oteltrace.SpanFromContext(req.Context())
	ctx, span := otel.Tracer(server_handlers.ServerName).Start(octx, funcStep.GetRouteName(), oteltrace.WithAttributes(attribute.String("requestID", req.Header.Get(server_handlers.RequestIdKey)), attribute.String("traceID", pspan.SpanContext().TraceID().String()), attribute.String("spanID", pspan.SpanContext().SpanID().String())))
	defer span.End()
	logs.WithContext(ctx).Info(fmt.Sprint("RunFuncStep - Start : ", funcStep.GetRouteName()))
	logs.WithContext(ctx).Info(fmt.Sprint("mainRouteName for ", funcStep.GetRouteName(), " is ", mainRouteName))
	req = req.WithContext(ctx)
	//first step is to transform the request which in turn will clone the request before transforming keeping original request as is for further use.
	request, vars, trErr := funcStep.transformRequest(ctx, req, reqVars, resVars, mainRouteName)
	if trErr != nil {
		err = trErr
		return
	}

	var responses []*http.Response
	var errs []error

	reqVars[funcStep.GetRouteName()] = vars

	logs.WithContext(ctx).Info(fmt.Sprint("funcStep.Condition = ", funcStep.Condition))
	strCond := "true"
	var strCondErr error
	if funcStep.Condition != "" {
		avars := &FuncTemplateVars{}
		avars.Vars = reqVars[funcStep.GetRouteName()]
		avars.ResVars = resVars
		avars.ReqVars = reqVars
		output, outputErr := processTemplate(ctx, funcStep.GetRouteName(), funcStep.Condition, avars, "string", "", "")
		logs.WithContext(ctx).Info(string(output))
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
		if strCond == "false" && (funcStep.ConditionFailAction == ConditionFailActionError || (funcStep.ConditionFailAction == ConditionFailActionIgnore) && len(funcStep.FuncSteps) == 0) {
			cfmBody := "{}"
			if funcStep.ConditionFailMessage != "" {
				cfmvars := &FuncTemplateVars{}
				cfmvars.Vars = reqVars[funcStep.GetRouteName()]
				cfmOutput, cfmOutputErr := processTemplate(ctx, funcStep.GetRouteName(), funcStep.ConditionFailMessage, avars, "json", "", "")
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
	var loopArray []interface{}
	asyncMessage := ""
	if strCond == "true" {

		if funcStep.Async && funcStep.AsyncMessage != "" {
			avars := &FuncTemplateVars{}
			avars.Vars = reqVars[funcStep.GetRouteName()]
			output, outputErr := processTemplate(ctx, funcStep.GetRouteName(), funcStep.AsyncMessage, avars, "json", "", "")
			logs.WithContext(ctx).Info(string(output))
			if outputErr != nil {
				err = outputErr
				response = errorResponse(ctx, err.Error(), request)
				return
			}
			asyncMessage = string(output)
		}

		lerr := false
		logs.WithContext(ctx).Info(fmt.Sprint("vars.LoopVars ", vars.LoopVars))
		logs.WithContext(ctx).Info(fmt.Sprint("funcStep.LoopVariable = ", funcStep.LoopVariable))
		if funcStep.LoopVariable != "" {
			loopArray, lerr = vars.LoopVars.([]interface{})
			if !lerr {
				err = errors.New("func loop variable is not an array")
				logs.WithContext(ctx).Error(err.Error())
				response = errorResponse(ctx, err.Error(), request)
				return
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

	go allocateFuncInner(ctx, request, funcStep, reqVars, resVars, loopArray, asyncMessage, jobs, mainRouteName, FuncThread, LoopThread, strCond)
	done := make(chan bool)
	//go result(done,results,responses, trResVars,errs)
	var trResVars []*TemplateVars
	go func(done chan bool, results chan FuncResult) {
		defer func() {
			if r := recover(); r != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in RunFuncStep: ", r))
			}
		}()
		for res := range results {
			responses = append(responses, res.response)
			trResVars = append(trResVars, res.responseVars)
			if res.responseErr != nil {
				errs = append(errs, res.responseErr)
			}
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
	response, _, err = clubResponses(ctx, responses, trResVars, errs)
	endTime := time.Now()
	diff := endTime.Sub(startTime)
	logs.WithContext(ctx).Info(fmt.Sprint("total time taken ", diff.Seconds(), "seconds"))
	logs.WithContext(ctx).Info(fmt.Sprint("RunFuncStep - End : ", funcStep.GetRouteName()))
	return
}
func (funcStep *FuncStep) RunFuncStepInner(ctx context.Context, req *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, asyncMsg string, funcThread int, loopThread int, strCond string) (response *http.Response, err error) {
	logs.WithContext(ctx).Info(fmt.Sprint("RunFuncStepInner - Start : ", funcStep.GetRouteName()))
	request := req
	if strCond == "true" {
		if funcStep.LoopVariable != "" {
			request, _, err = funcStep.transformRequest(ctx, req, reqVars, resVars, mainRouteName)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return
			}
		}
		routevars := &TemplateVars{}
		_ = routevars

		if funcStep.FunctionName != "" {
			//TODO - we have to return routevars
			response, err = RunFuncSteps(ctx, funcStep.FuncGroup.FuncSteps, request, reqVars, resVars, "", funcThread, loopThread)
		} else {
			response, routevars, err = funcStep.Route.Execute(ctx, request, funcStep.Path, funcStep.Async, asyncMsg, reqVars[funcStep.GetRouteName()], loopThread)
		}

		resVars[funcStep.GetRouteName()] = routevars
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
			resVars[funcStep.GetRouteName()], trespErr = funcStep.transformResponse(ctx, response, resVars[funcStep.GetRouteName()], reqVars, resVars)
			if trespErr != nil {
				err = trespErr
				return
			}
		}
		logs.WithContext(ctx).Info(fmt.Sprint("funcStep.Route.Redirect = ", funcStep.Route.Redirect))
		if funcStep.Route.Redirect {
			logs.WithContext(ctx).Info(funcStep.Route.FinalRedirectUrl)
			response.StatusCode = http.StatusSeeOther
			response.Header.Set("Location", funcStep.Route.FinalRedirectUrl)
			return
		}
	}
	if len(funcStep.FuncSteps) > 0 {
		response, err = RunFuncSteps(ctx, funcStep.FuncSteps, request, reqVars, resVars, mainRouteName, funcThread, loopThread)
	}
	return
}

func (funcStep *FuncStep) transformRequest(ctx context.Context, request *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string) (req *http.Request, vars *TemplateVars, err error) {
	logs.WithContext(ctx).Debug("transformRequest - Start")
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
	if reqVars[mainRouteName] == nil {
		err = loadRequestVars(ctx, vars, req)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
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
		logs.WithContext(ctx).Info(fmt.Sprint(vars.LoopVars))
		logs.WithContext(ctx).Info(fmt.Sprint(vars.LoopVar))
		logs.WithContext(ctx).Info(fmt.Sprint("funcStep.LoopVariable = ", funcStep.LoopVariable))
		fvars.Vars = vars
		fvars.ResVars = resVars
		fvars.ReqVars = reqVars
		output, outputErr := processTemplate(ctx, funcStep.GetRouteName(), funcStep.LoopVariable, fvars, "json", "", "")
		logs.WithContext(ctx).Info(fmt.Sprint("loop variable after template processing : ", string(output)))
		if outputErr != nil {
			err = outputErr
			return
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
		logs.WithContext(ctx).Info(fmt.Sprint("loopArray = ", loopArray))

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

	logs.WithContext(ctx).Debug(fmt.Sprint("newContentType = ", newContentType))
	logs.WithContext(ctx).Debug(fmt.Sprint("oldContentType = ", oldContentType))

	//first check if original request is not multipart but the new request to be forwarded to target host is multipart - then make multipart body from json body
	// else if original request is multipart/form , we process the same
	makeMultiPartCalled := false
	if (newContentType == encodedForm || newContentType == multiPartForm) && newContentType != oldContentType {
		vars.FormData, vars.FormDataKeyArray, err = makeMultipart(ctx, req, funcStep.FormData, funcStep.FileData, vars, reqVars, resVars, funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
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
				output, fdErr := processTemplate(ctx, fd.Key, fd.Value, mpvars, "string", "", "")
				if fdErr != nil {
					err = fdErr
					return
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
			vars.FormData, vars.FormDataKeyArray, err = processMultipart(ctx, oldContentType, req, funcStep.RemoveParams.FormData, vars.FormData)
			if err != nil {
				return
			}
			//changing it back to new content type once process multipart has read the request body and loaded vars.formdata
			req.Header.Set("Content-type", newContentType)
		} else if oldContentType == encodedForm {
			rpfErr := req.ParseForm()
			if rpfErr != nil {
				err = rpfErr
				logs.WithContext(ctx).Info(fmt.Sprint("error from request.ParseForm() = ", err.Error()))
				return
			}
			if req.PostForm != nil {
				for k, v := range req.PostForm {
					vars.FormData[k] = strings.Join(v, ",")
					vars.FormDataKeyArray = append(vars.FormDataKeyArray, k)
				}
			}
		}
	}

	//next we process and transform query params and set it in request
	err = processParams(ctx, req, funcStep.RemoveParams.QueryParams, funcStep.QueryParams, vars, reqVars, resVars)
	if err != nil {
		return
	}
	//utils.PrintRequestBody(req, "printing request in transformRequest after processParams")
	//next we process and transform request body only if it is not multipart and set it in request
	logs.WithContext(ctx).Info(fmt.Sprint("funcStep.TransformRequest = ", funcStep.TransformRequest))
	if funcStep.TransformRequest != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = vars
		fvars.ResVars = resVars
		fvars.ReqVars = reqVars
		output, err := processTemplate(ctx, funcStep.GetRouteName(), funcStep.TransformRequest, fvars, "json", funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
		if err != nil {
			return req, &TemplateVars{}, err
		}
		err = json.Unmarshal(output, &vars.Body)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return req, &TemplateVars{}, err
		}
		req.Body = io.NopCloser(bytes.NewBuffer(output))
		req.Header.Set("Content-Length", strconv.Itoa(len(output)))
		req.ContentLength = int64(len(output))
	} else if !makeMultiPartCalled {
		logs.WithContext(ctx).Info("inside !makeMultiPartCalled")
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
	// lastly we process and transform template based headers and set it in request
	err = processHeaderTemplates(ctx, req, funcStep.RemoveParams.RequestHeaders, funcStep.RequestHeaders, false, vars, funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl, reqVars, resVars)
	if err != nil {
		return
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
		tmplBodyFromRes := json.NewDecoder(response.Body)
		tmplBodyFromRes.DisallowUnknownFields()
		if err = tmplBodyFromRes.Decode(&vars.Body); err != nil {
			body, readErr := io.ReadAll(tmplBodyFromRes.Buffered())
			if readErr != nil {
				err = readErr
				logs.WithContext(ctx).Error(fmt.Sprint("io.ReadAll(response.Body) error : ", err.Error()))
				return
			}
			err = nil
			tempBody := make(map[string]string)
			tempBody["data"] = string(body)
			vars.Body = tempBody
		}
		vars.OrgBody = vars.Body
		if funcStep.TransformResponse != "" {
			fvars := &FuncTemplateVars{}
			fvars.Vars = vars
			fvars.ResVars = resVars
			fvars.ReqVars = reqVars

			output, err := processTemplate(ctx, funcStep.GetRouteName(), funcStep.TransformResponse, fvars, "json", funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
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
	return
}
