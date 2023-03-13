package routes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
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
	QueryName            string
	Api                  TargetHost
	ApiPath              string
	Path                 string
	Route                Route `json:"-"`
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

func (funcGroup *FuncGroup) Execute(request *http.Request, FuncThreads int, LoopThreads int) (response *http.Response, err error) {
	log.Println("inside funGroup Execute")
	reqVars := make(map[string]*TemplateVars)
	resVars := make(map[string]*TemplateVars)
	response, err = RunFuncSteps(funcGroup.FuncSteps, request, reqVars, resVars, "", FuncThreads, LoopThreads)
	return
}

func RunFuncSteps(funcSteps map[string]*FuncStep, request *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, funcThreads int, loopThreads int) (response *http.Response, err error) {
	var responses []*http.Response
	var errs []error

	//for _, cv := range funcSteps {
	//	response, err = cv.RunFuncStep(request, reqVars, resVars, mainRouteName)
	//}

	var funcJobs = make(chan FuncJob, 10)
	var funcResults = make(chan FuncResult, 10)
	startTime := time.Now()
	go allocateFunc(request, funcSteps, reqVars, resVars, funcJobs, mainRouteName, funcThreads, loopThreads)
	done := make(chan bool)

	go func(done chan bool, funcResults chan FuncResult) {
		defer func() {
			if r := recover(); r != nil {
				log.Print("goroutine paniqued Func Execute: ", r)
			}
		}()
		for res := range funcResults {
			//utils.PrintResponseBody(res.response, "printning res.response from funcResults of RunFuncSteps")

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

	createWorkerPoolFunc(noOfWorkers, funcJobs, funcResults)
	<-done
	endTime := time.Now()
	diff := endTime.Sub(startTime)
	fmt.Println("total time taken ", diff.Seconds(), "seconds")
	response, _, err = clubResponses(responses, nil, errs)

	return
}

func (funcStep *FuncStep) GetRouteName() (routeName string) {
	if funcStep.QueryName != "" {
		routeName = funcStep.QueryName
	} else if funcStep.Api.Host != "" {
		routeName = funcStep.Api.Host
	} else if funcStep.RouteName != "" {
		routeName = funcStep.RouteName
	}
	return
}
func (funcStep *FuncStep) RunFuncStep(req *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, FuncThread int, LoopThread int) (response *http.Response, err error) {
	log.Println("*******************funcStep execute start for ", funcStep.GetRouteName(), " *******************")
	log.Print("mainRouteName for ", funcStep.GetRouteName(), " is ", mainRouteName)

	//for k, v := range resVars {
	//	log.Print(k)
	//	log.Print(v)
	//}

	//first step is to transform the request which in turn will clone the request before transforming keeping original request as is for further use.
	request, vars, trErr := funcStep.transformRequest(req, reqVars, resVars, mainRouteName)
	if trErr != nil {
		err = trErr
		log.Println(err)
		return
	}

	var responses []*http.Response
	var errs []error
	log.Print("printing vars after transformRequest")
	log.Print(vars.FormData)

	//log.Print("adding vars to = ", funcStep.GetRouteName())
	//log.Print(vars)
	reqVars[funcStep.GetRouteName()] = vars

	//reqVars[funcStep.GetRouteName()] = &TemplateVars{}
	//reqVars[funcStep.GetRouteName()].FormData = make(map[string]interface{})
	//reqVars[funcStep.GetRouteName()].Body = make(map[string]interface{})
	//reqVars[funcStep.GetRouteName()].OrgBody = make(map[string]interface{})

	if funcStep.Condition != "" {
		avars := &FuncTemplateVars{}
		avars.Vars = reqVars[funcStep.GetRouteName()]
		log.Print(avars.Vars.OrgBody)
		log.Print("evaluating condition = ", funcStep.Condition)
		log.Print(avars.Vars.LoopVar)
		log.Print(avars.Vars.LoopVars)
		output, outputErr := processTemplate(funcStep.GetRouteName(), funcStep.Condition, avars, "string", "", "")
		log.Print(string(output))
		if outputErr != nil {
			log.Println(outputErr)
			err = outputErr
			response = errorResponse(err.Error(), request)
			return
		}
		strCond, strCondErr := strconv.Unquote(string(output))
		if strCondErr != nil {
			log.Println(strCondErr)
			err = strCondErr
			response = errorResponse(err.Error(), request)
			return
		}
		log.Print("strCond == ", strCond)
		if strCond == "false" {
			cfmBody := "{}"
			if funcStep.ConditionFailMessage != "" {
				cfmvars := &FuncTemplateVars{}
				cfmvars.Vars = reqVars[funcStep.GetRouteName()]
				cfmOutput, cfmOutputErr := processTemplate(funcStep.GetRouteName(), funcStep.ConditionFailMessage, avars, "json", "", "")
				log.Print(string(cfmOutput))
				if cfmOutputErr != nil {
					log.Println(cfmOutputErr)
					err = cfmOutputErr
					response = errorResponse(err.Error(), request)
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
				Body:          ioutil.NopCloser(bytes.NewBufferString(cfmBody)),
				ContentLength: int64(len(cfmBody)),
				Request:       request,
				Header:        condRespHeader,
			}
			responses = append(responses, response)
			return
		}
	}
	asyncMessage := ""
	if funcStep.Async && funcStep.AsyncMessage != "" {
		avars := &FuncTemplateVars{}
		avars.Vars = reqVars[funcStep.GetRouteName()]
		output, outputErr := processTemplate(funcStep.GetRouteName(), funcStep.AsyncMessage, avars, "json", "", "")
		log.Print(string(output))
		if outputErr != nil {
			log.Println(outputErr)
			err = outputErr
			response = errorResponse(err.Error(), request)
			return
		}
		asyncMessage = string(output)
	}
	var loopArray []interface{}
	lerr := false
	log.Print("reqVars[funcStep.GetRouteName()].LoopVars ", reqVars[funcStep.GetRouteName()].LoopVars)
	log.Print("vars.LoopVars ", vars.LoopVars)
	if funcStep.LoopVariable != "" {
		loopArray, lerr = vars.LoopVars.([]interface{})
		if !lerr {
			err = errors.New("func loop variable is not an array")
			log.Println(err)
			response = errorResponse(err.Error(), request)
			return
		}
	}
	/*var loopArray []interface{}
	if funcStep.LoopVariable != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = reqVars[funcStep.GetRouteName()]
		output, outputErr := processTemplate(funcStep.GetRouteName(), funcStep.LoopVariable, fvars, "json", "", "")
		log.Print("loop variable after template processing")
		log.Print(string(output))
		if outputErr != nil {
			log.Println(outputErr)
			err = outputErr
			response = errorResponse(err.Error(), request)
			return
		}
		var loopJson interface{}
		loopJsonErr := json.Unmarshal(output, &loopJson)
		if loopJsonErr != nil {
			err = errors.New("func loop variable is not a json")
			response = errorResponse(err.Error(), request)
			log.Print(loopJsonErr)
		}

		ok := false
		if loopArray, ok = loopJson.([]interface{}); !ok {
			err = errors.New("func loop variable is not an array")
			log.Print(err)
			response = errorResponse(err.Error(), request)
			return
		}
		log.Print("loopArray = ", loopArray)

	} else {
		//dummy row added to create a job
		loopArray = append(loopArray, make(map[string]interface{}))
	}
	*/
	if len(loopArray) == 0 {
		//dummy row added to create a job
		loopArray = append(loopArray, make(map[string]interface{}))
	}
	var jobs = make(chan FuncJob, 10)
	var results = make(chan FuncResult, 10)
	startTime := time.Now()

	go allocateFuncInner(request, funcStep, reqVars, resVars, loopArray, asyncMessage, jobs, mainRouteName, FuncThread, LoopThread)
	done := make(chan bool)
	//go result(done,results,responses, trResVars,errs)
	var trResVars []*TemplateVars
	go func(done chan bool, results chan FuncResult) {
		defer func() {
			if r := recover(); r != nil {
				log.Print("goroutine paniqued RunFuncStep: ", r)
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

	createWorkerPoolFuncInner(noOfWorkers, jobs, results)
	<-done
	response, _, err = clubResponses(responses, trResVars, errs)
	endTime := time.Now()
	diff := endTime.Sub(startTime)
	fmt.Println("total time taken ", diff.Seconds(), "seconds")
	log.Println("*******************funcStep execute end for ", funcStep.GetRouteName(), " *******************")
	return
}
func (funcStep *FuncStep) RunFuncStepInner(req *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, asyncMsg string, funcThread int, loopThread int) (response *http.Response, err error) {
	log.Print("inside RunFuncStepInner for ", funcStep.GetRouteName())

	request := req
	if funcStep.LoopVariable != "" {
		request, _, err = funcStep.transformRequest(req, reqVars, resVars, mainRouteName)
		if err != nil {
			log.Println(err)
			return
		}
	}
	routevars := &TemplateVars{}
	_ = routevars
	//utils.PrintRequestBody(request, "before funcStep.Route.Execute")
	log.Print(reqVars[funcStep.GetRouteName()].LoopVars)
	log.Print(reqVars[funcStep.GetRouteName()].LoopVar)
	log.Print(reqVars[funcStep.GetRouteName()].Body)
	log.Print(reqVars[funcStep.GetRouteName()].OrgBody)

	response, routevars, err = funcStep.Route.Execute(request, funcStep.Path, funcStep.Async, asyncMsg, reqVars[funcStep.GetRouteName()], loopThread)
	if err != nil {
		log.Print(err)
	}
	resVars[funcStep.GetRouteName()] = routevars

	if funcStep.Route.OnError == "STOP" && response.StatusCode >= 400 {
		log.Print("inside funcStep.Route.OnError == \"STOP\" && response.StatusCode >= 400")
		return
	} else {
		log.Print("Ignoring route execution error : ", err.Error())
		err = nil
		cfmBody := "{}"
		response = &http.Response{
			StatusCode:    http.StatusOK,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Body:          ioutil.NopCloser(bytes.NewBufferString(cfmBody)),
			ContentLength: int64(len(cfmBody)),
			Request:       request,
			Header:        http.Header{},
		}
	}

	// in case of error - no need to call  transformResponse
	if err == nil {
		var trespErr error
		resVars[funcStep.GetRouteName()], trespErr = funcStep.transformResponse(response, resVars[funcStep.GetRouteName()], reqVars, resVars)
		if trespErr != nil {
			err = trespErr
			log.Print(err)
			return
		}
	}
	log.Print("funcStep.Route.Redirect = ", funcStep.Route.Redirect)
	if funcStep.Route.Redirect {
		log.Print(funcStep.Route.FinalRedirectUrl)
		response.StatusCode = http.StatusSeeOther
		response.Header.Set("Location", funcStep.Route.FinalRedirectUrl)
		//http.Redirect(w, r, route.FinalRedirectUrl, http.StatusSeeOther)
		return
	}
	//utils.PrintResponseBody(response, fmt.Sprint("printing response for func ", funcStep.GetRouteName()))
	//log.Print("funcStep.FuncSteps != nil = " , funcStep.FuncSteps != nil )
	if len(funcStep.FuncSteps) > 0 {
		response, err = RunFuncSteps(funcStep.FuncSteps, request, reqVars, resVars, mainRouteName, funcThread, loopThread)
	}

	/*for _, cv := range funcStep.FuncSteps {
		//if oldContentType == encodedForm || oldContentType == multiPartForm {
		// in case of multipart or form data, send cloned request to child as multipart cannot be processed twice
		//	response, err = cv.RunFuncStep(req, reqVars, resVars, mainRouteName)
		//} else {
		response, err = cv.RunFuncStep(request, reqVars, resVars, mainRouteName)
		//}
	}*/
	return
}

func (funcStep *FuncStep) transformRequest(request *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string) (req *http.Request, vars *TemplateVars, err error) {
	log.Println("inside funcStep.transformRequest")
	//first step in transforming is to make a clone of the original request
	req, err = CloneRequest(request)
	if err != nil {
		log.Print(err)
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
		err = loadRequestVars(vars, req)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		v, _ := cloneInterface(reqVars[mainRouteName])
		vars = v.(*TemplateVars)
	}

	var loopArray []interface{}
	if funcStep.LoopVariable != "" {
		fvars := &FuncTemplateVars{}
		log.Print(vars.LoopVars)
		log.Print(vars.LoopVar)
		log.Print("funcStep.LoopVariable = ", funcStep.LoopVariable)
		fvars.Vars = vars
		output, outputErr := processTemplate(funcStep.GetRouteName(), funcStep.LoopVariable, fvars, "json", "", "")
		log.Print("loop variable after template processing")
		log.Print(string(output))
		if outputErr != nil {
			log.Println(outputErr)
			err = outputErr
			return
		}
		var loopJson interface{}
		loopJsonErr := json.Unmarshal(output, &loopJson)
		if loopJsonErr != nil {
			err = errors.New("func loop variable is not a json")
			log.Print(loopJsonErr)
		}

		ok := false
		if loopArray, ok = loopJson.([]interface{}); !ok {
			err = errors.New("func loop variable is not an array")
			log.Print(err)
			return
		}
		log.Print("loopArray = ", loopArray)

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

	log.Print("newContentType = ", newContentType)
	log.Print("oldContentType = ", oldContentType)

	//first check if original request is not multipart but the new request to be forwarded to target host is multipart - then make multipart body from json body
	// else if original request is multipart/form , we process the same
	makeMultiPartCalled := false
	if (newContentType == encodedForm || newContentType == multiPartForm) && newContentType != oldContentType {
		vars.FormData, vars.FormDataKeyArray, err = makeMultipart(req, funcStep.FormData, funcStep.FileData, vars, reqVars, resVars, funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
		makeMultiPartCalled = true
		if err != nil {
			log.Print(err)
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
				log.Print("inside route.FormData")
				log.Print(fd.Key)
				output, fdErr := processTemplate(fd.Key, fd.Value, mpvars, "string", "", "")
				if fdErr != nil {
					err = fdErr
					log.Println(err)
					return
				}
				outputStr, fduErr := strconv.Unquote(string(output))
				if fduErr != nil {
					err = fduErr
					log.Println(err)
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
			vars.FormData, vars.FormDataKeyArray, err = processMultipart(oldContentType, req, funcStep.RemoveParams.FormData, vars.FormData)
			if err != nil {
				log.Print("printing error recd from processMultipart")
				log.Print(err)
				return
			}
			//changing it back to new content type once process multipart has read the request body and loaded vars.formdata
			req.Header.Set("Content-type", newContentType)
		} else if oldContentType == encodedForm {
			rpfErr := req.ParseForm()
			if rpfErr != nil {
				err = rpfErr
				log.Print("error from request.ParseForm() = ", err.Error())
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
	err = processParams(req, funcStep.RemoveParams.QueryParams, funcStep.QueryParams, vars, reqVars, resVars)
	if err != nil {
		log.Print(err)
		return
	}

	//next we process and transform request body only if it is not multipart and set it in request
	log.Println("funcStep.TransformRequest = ", funcStep.TransformRequest)
	if funcStep.TransformRequest != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = vars
		fvars.ResVars = resVars
		fvars.ReqVars = reqVars
		log.Print("vars.LoopVars")
		log.Print(vars.LoopVars)
		if reqVars["cygnetauthtoken"] != nil {
			log.Print(reqVars["cygnetauthtoken"])
		}
		output, err := processTemplate(funcStep.GetRouteName(), funcStep.TransformRequest, fvars, "json", funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
		if err != nil {
			log.Println(err)
			return req, &TemplateVars{}, err
		}
		err = json.Unmarshal(output, &vars.Body)
		if err != nil {
			log.Println(err)
			return req, &TemplateVars{}, err
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(output))
		req.Header.Set("Content-Length", strconv.Itoa(len(output)))
		req.ContentLength = int64(len(output))
	} else if !makeMultiPartCalled {
		log.Print("inside !makeMultiPartCalled")
		rb, err1 := json.Marshal(vars.Body)
		if err1 != nil {
			err = err1
			log.Println(err)
			return
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(rb))
		req.Header.Set("Content-Length", strconv.Itoa(len(rb)))
		req.ContentLength = int64(len(rb))
	}

	// lastly we process and transform template based headers and set it in request
	err = processHeaderTemplates(req, funcStep.RemoveParams.RequestHeaders, funcStep.RequestHeaders, false, vars, funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl, reqVars, resVars)
	if err != nil {
		log.Println(err)
		return
	}
	return req, vars, err
}

func (funcStep *FuncStep) transformResponse(response *http.Response, trResVars *TemplateVars, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars) (vars *TemplateVars, err error) {
	log.Println("inside funcStep transformResponse")
	//utils.PrintResponseBody(response, "response printed from inside funcStep transformResponse")
	vars = trResVars

	for _, h := range funcStep.ResponseHeaders {
		response.Header.Set(h.Key, h.Value)
	}
	log.Println("TransformResponse = ", funcStep.TransformResponse)

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

	tmplBodyFromRes := json.NewDecoder(response.Body)
	tmplBodyFromRes.DisallowUnknownFields()
	if err = tmplBodyFromRes.Decode(&vars.Body); err != nil {
		body, readErr := ioutil.ReadAll(tmplBodyFromRes.Buffered())
		if readErr != nil {
			err = readErr
			log.Println("ioutil.ReadAll(response.Body) error")
			log.Println(err)
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

		output, err := processTemplate(funcStep.GetRouteName(), funcStep.TransformResponse, fvars, "json", funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
		if err != nil {
			log.Println(err)
			return &TemplateVars{}, err
		}
		response.Body = ioutil.NopCloser(bytes.NewBuffer(output))
		response.Header.Set("Content-Length", strconv.Itoa(len(output)))
		response.ContentLength = int64(len(output))
		err = json.Unmarshal(output, &vars.Body)
		if err != nil {
			log.Println(err)
			return &TemplateVars{}, err
		}
	} else {
		rb, err := json.Marshal(vars.Body)
		if err != nil {
			log.Println(err)
			return &TemplateVars{}, err
		}
		response.Body = ioutil.NopCloser(bytes.NewReader(rb))
		response.Header.Set("Content-Length", strconv.Itoa(len(rb)))
		response.ContentLength = int64(len(rb))
	}
	if funcStep.RemoveParams.ResponseHeaders != nil {
		for _, v := range funcStep.RemoveParams.ResponseHeaders {
			response.Header.Del(v)
		}
	}
	return
}
