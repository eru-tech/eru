package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
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

func (funcGroup *FuncGroup) Execute(request *http.Request) (response *http.Response, err error) {
	log.Println("inside funGroup Execute")
	reqVars := make(map[string]*TemplateVars)
	resVars := make(map[string]*TemplateVars)
	log.Print("len(funcGroup.FuncSteps) = ", len(funcGroup.FuncSteps))
	var responses []*http.Response
	var errs []error

	var funcJobs = make(chan FuncJob, 10)
	var funcResults = make(chan FuncResult, 10)
	startTime := time.Now()
	go allocateFunc(request, funcGroup.FuncSteps, reqVars, resVars, funcJobs)
	done := make(chan bool)

	go func(done chan bool, funcResults chan FuncResult) {
		defer func() {
			if r := recover(); r != nil {
				log.Print("goroutine paniqued Func Execute: ", r)
			}
		}()
		for res := range funcResults {
			responses = append(responses, res.response)
			//trResVars = append(trResVars, res.responseVars)
			errs = append(errs, res.responseErr)
		}
		done <- true
	}(done, funcResults)

	//set it to one to run synchronously - change it if LoopInParallel is true to run in parallel
	noOfWorkers := 1 //TODO make to configurable
	if len(funcGroup.FuncSteps) < noOfWorkers {
		noOfWorkers = len(funcGroup.FuncSteps)
	}

	log.Print("noOfWorkers = ", noOfWorkers)
	createWorkerPoolFunc(noOfWorkers, funcJobs, funcResults)
	<-done
	log.Print("after done")
	log.Print("len(responses) = ", len(responses))
	//log.Print("len(trVars) = ", len(trResVars))
	log.Print("len(errs) = ", len(errs))
	endTime := time.Now()
	diff := endTime.Sub(startTime)
	fmt.Println("total time taken ", diff.Seconds(), "seconds")
	//for _, v := range funcGroup.FuncSteps {
	//	response, err = v.execute(request, reqVars, resVars, v.RouteName)
	//	utils.PrintResponseBody(response,"printing response from (funcGroup *FuncGroup) Execute")
	//}
	response, _, err = clubResponses(responses, nil, errs)
	return
}

func (funcStep *FuncStep) RunFuncStep(req *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string) (response *http.Response, err error) {
	log.Println("*******************funcStep execute start for ", funcStep.RouteName, " *******************")
	log.Print(funcStep.Route)
	request, err := cloneRequest(req)
	var responses []*http.Response
	var errs []error
	reqVars[funcStep.RouteName] = &TemplateVars{}
	//var trResVars []*TemplateVars
	//var errs []error
	lrErr := loadRequestVars(reqVars[funcStep.RouteName], request)
	if lrErr != nil {
		log.Println(lrErr)
		err = lrErr
		response = errorResponse(err.Error(), request)
		return
	}

	if funcStep.Condition != "" {
		avars := &FuncTemplateVars{}
		avars.Vars = reqVars[funcStep.RouteName]
		output, outputErr := processTemplate(funcStep.RouteName, funcStep.Condition, avars, "string", "", "")
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
		if strCond == "false" {
			cfmBody := "{}"
			if funcStep.ConditionFailMessage != "" {
				cfmvars := &FuncTemplateVars{}
				cfmvars.Vars = reqVars[funcStep.RouteName]
				cfmOutput, cfmOutputErr := processTemplate(funcStep.RouteName, funcStep.ConditionFailMessage, avars, "json", "", "")
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
		avars.Vars = reqVars[funcStep.RouteName]
		output, outputErr := processTemplate(funcStep.RouteName, funcStep.AsyncMessage, avars, "json", "", "")
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
	if funcStep.LoopVariable != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = reqVars[funcStep.RouteName]
		output, outputErr := processTemplate(funcStep.RouteName, funcStep.LoopVariable, fvars, "json", "", "")
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

	var jobs = make(chan FuncJob, 10)
	var results = make(chan FuncResult, 10)
	startTime := time.Now()
	go allocateFuncInner(request, funcStep, reqVars, resVars, loopArray, asyncMessage, jobs)
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
			errs = append(errs, res.responseErr)
		}
		done <- true
	}(done, results)

	//set it to one to run synchronously - change it if LoopInParallel is true to run in parallel
	noOfWorkers := 1
	if funcStep.LoopInParallel && funcStep.LoopVariable != "" {
		noOfWorkers = 5
		if len(loopArray) < noOfWorkers {
			noOfWorkers = len(loopArray)
		}
	}

	log.Print("noOfWorkers = ", noOfWorkers)
	createWorkerPoolFuncInner(noOfWorkers, jobs, results)
	<-done
	log.Print("after done")
	log.Print("len(responses) = ", len(responses))
	log.Print("len(trVars) = ", len(trResVars))
	log.Print(&trResVars)
	log.Print("len(errs) = ", len(errs))
	log.Print("calling clubResponses from route")
	response, _, err = clubResponses(responses, trResVars, errs)
	endTime := time.Now()
	diff := endTime.Sub(startTime)
	fmt.Println("total time taken ", diff.Seconds(), "seconds")
	log.Println("*******************funcStep execute end for ", funcStep.RouteName, " *******************")
	return
}
func (funcStep *FuncStep) RunFuncStepInner(req *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string, asyncMsg string) (response *http.Response, err error) {
	//utils.PrintRequestBody(request,"printing from RunFuncStepInner")
	log.Print("inside RunFuncStepInner")
	log.Print(reqVars[funcStep.RouteName].LoopVars)
	request, vars, trErr := funcStep.transformRequest(req, reqVars, resVars, mainRouteName)
	if trErr != nil {
		err = trErr
		log.Println(err)
		return
	}
	reqVars[funcStep.RouteName] = vars
	defer req.Body.Close()

	routevars := &TemplateVars{}
	_ = routevars
	response, routevars, err = funcStep.Route.Execute(request, funcStep.Path, funcStep.Async, asyncMsg)
	log.Print("print after funcStep.Route.Execute")
	log.Print(response)
	log.Print(routevars)

	if err != nil {
		log.Print(err)
	}

	if funcStep.Route.OnError == "STOP" && response.StatusCode >= 400 {
		log.Print("inside funcStep.Route.OnError == \"STOP\" && response.StatusCode >= 400")
		return
	}
	resVars[funcStep.RouteName] = routevars
	//log.Println("resVars[funcStep.RouteName] for ",funcStep.RouteName)
	//log.Println(resVars[funcStep.RouteName])

	//response *http.Response, trReqVars TemplateVars, resHeaders []Headers, removeHeaders []string, templateName string, templateString string,tokenHeaderKey string,jwkUrl string) (trResVars TemplateVars , err error)

	// in case of error - no need to call  transformResponse
	if err == nil {
		var trespErr error
		resVars[funcStep.RouteName], trespErr = funcStep.transformResponse(response, resVars[funcStep.RouteName], reqVars, resVars)
		if trespErr != nil {
			err = trespErr
			log.Print(err)
			return
		}
	}
	//log.Println("resVars[funcStep.RouteName] for ",funcStep.RouteName, " after funcStep.transformResponse")
	//log.Println(resVars[funcStep.RouteName])
	log.Print("len(funcStep.FuncSteps) = ", len(funcStep.FuncSteps))
	for _, cv := range funcStep.FuncSteps {
		response, err = cv.RunFuncStep(request, reqVars, resVars, mainRouteName)
	}
	log.Print("after loop of funcStep.FuncSteps")
	log.Print(err)
	log.Print(response)
	//		loopCounter++
	//}
	return
}

func cloneRequest(request *http.Request) (req *http.Request, err error) {
	log.Println("clone request")
	req = request.Clone(context.Background())

	//req, err = http.NewRequest(request.Method, request.URL.String(), nil)
	//if err != nil {
	//	log.Println(err)
	//}
	//req.Header=request.Header

	reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	log.Print("reqContentType = ", reqContentType)
	if reqContentType == encodedForm || reqContentType == multiPartForm {
		log.Println("inside encodedForm || multiPartForm")
		var reqBody bytes.Buffer
		multipartWriter := multipart.NewWriter(&reqBody)
		multiPart, err1 := request.MultipartReader()
		if err1 != nil {
			err = err1
			log.Println("----------------------------")
			log.Println(err)
			return
		}
		for {
			part, errPart := multiPart.NextRawPart()
			log.Println(errPart)
			if errPart == io.EOF {
				log.Println("inside EOF error")
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
				fileWriter, err2 := createFormFileCopy(multipartWriter, part)
				//fileWriter, err := multipartWriter.CreateFormFile(part.FormName(), part.FileName())
				if err2 != nil {
					err = err2
					log.Println(err)
					return
				}
				//_, err = fileWriter.Write()
				_, err = io.Copy(fileWriter, part)
				if err != nil {
					log.Println(err)
					return
				}

			} else {
				buf := new(bytes.Buffer)
				buf.ReadFrom(part)
				fieldWriter, err3 := multipartWriter.CreateFormField(part.FormName())
				if err3 != nil {
					err = err3
					log.Println(err)
					return
				}
				_, err = fieldWriter.Write(buf.Bytes())
				if err != nil {
					log.Println(err)
					return
				}
			}
		}
		multipartWriter.Close()
		req.Body = ioutil.NopCloser(&reqBody)
		//request.Header.Set("Content-Type","application/pdf" )
		log.Println(multipartWriter.FormDataContentType())
		req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		req.Header.Set("Content-Length", strconv.Itoa(reqBody.Len()))
		req.ContentLength = int64(reqBody.Len())
	} else {
		log.Print("inside else - cloneRequest")
		body, err3 := ioutil.ReadAll(request.Body)
		if err3 != nil {
			err = err3
			log.Println(err)
			return
		}
		//log.Println("body from clonerequest - else part")
		//log.Println(string(body))
		request.Body = ioutil.NopCloser(bytes.NewReader(body))
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
	}

	return
}

func (funcStep *FuncStep) transformRequest(request *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string) (req *http.Request, vars *TemplateVars, err error) {
	log.Println("inside funcStep.transformRequest")

	oldContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	req, err = cloneRequest(request)

	//printRequestBody(req,"body from funcstep transformRequest - after clone")
	if err != nil {
		log.Println(err)
		return req, &TemplateVars{}, err
	}

	makeMultiPartCalled := false
	//printRequestBody(request,"body from funcStep transformRequest")
	//defer request.Body.Close()
	vars = &TemplateVars{}
	log.Print(mainRouteName, " == ", funcStep.RouteName)
	if mainRouteName == funcStep.RouteName {
		err = loadRequestVars(vars, req)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		log.Println("vars = reqVars[mainRouteName]")
		vars = reqVars[mainRouteName]
	}

	//reqVarsLoaded := false
	//vars.FormData = make(map[string]interface{})
	//vars.FormData = reqVars[funcStep.RouteName].FormData
	//vars.FormDataKeyArray = reqVars[funcStep.RouteName].FormDataKeyArray
	//vars.Headers = reqVars[funcStep.RouteName].Headers
	//vars.Params = reqVars[funcStep.RouteName].Params
	//vars.ReqVars = reqVars
	//vars.ResVars = resVars
	for _, h := range funcStep.RequestHeaders {
		if !h.IsTemplate {
			req.Header.Set(h.Key, h.Value)
		}
	}
	newContentType := strings.Split(req.Header.Get("Content-type"), ";")[0]
	//vars.FormData , vars.FormDataKeyArray, err = processMultipart(request,funcStep.RemoveParams.FormData,funcStep.FormData)
	//if err != nil {
	//	return
	//}
	log.Print("newContentType = ", newContentType)
	log.Print("oldContentType = ", oldContentType)
	if (newContentType == encodedForm || newContentType == multiPartForm) && newContentType != oldContentType {
		log.Println(vars.FormData)
		vars.FormData, vars.FormDataKeyArray, err = makeMultipart(req, funcStep.FormData, funcStep.FileData, vars, reqVars, resVars, funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
		makeMultiPartCalled = true
		if err != nil {
			return
		}
	}

	err = processParams(req, funcStep.RemoveParams.QueryParams, funcStep.QueryParams, vars, reqVars, resVars)
	if err != nil {
		return
	}

	log.Println("funcStep.TransformRequest = ", funcStep.TransformRequest)
	//vars := module_model.TemplateVars{}
	if funcStep.TransformRequest != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = vars
		fvars.ResVars = resVars
		fvars.ReqVars = reqVars
		log.Print("vars.LoopVars")
		log.Print(vars.LoopVars)
		output, err := processTemplate(funcStep.RouteName, funcStep.TransformRequest, fvars, "json", funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
		if err != nil {
			log.Println(err)
			return req, &TemplateVars{}, err
		}
		err = json.Unmarshal(output, &vars.Body)
		if err != nil {
			log.Println(err)
			return req, &TemplateVars{}, err
		}
		log.Print("printing output from func transform request")
		log.Print(string(output))
		req.Body = ioutil.NopCloser(bytes.NewBuffer(output))
		req.Header.Set("Content-Length", strconv.Itoa(len(output)))
		req.ContentLength = int64(len(output))
	} else if !makeMultiPartCalled {
		vars.OrgBody = vars.Body
		rb, err1 := json.Marshal(vars.Body)
		if err1 != nil {
			err = err1
			log.Println(err)
			return
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(rb))
		req.Header.Set("Content-Length", strconv.Itoa(len(rb)))
		req.ContentLength = int64(len(rb))
		//log.Println(req.Header.Get("Content-Length"))
	}

	//printRequestBody(req, "body from funcstep transformRequest - after else part")
	//defer req.Body.Close()
	err = processHeaderTemplates(req, funcStep.RemoveParams.RequestHeaders, funcStep.RequestHeaders, false, vars, funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl, reqVars, resVars)
	if err != nil {
		log.Println(err)
		return
	}

	return req, vars, err
}

func (funcStep *FuncStep) transformResponse(response *http.Response, trReqVars *TemplateVars, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars) (vars *TemplateVars, err error) {
	log.Println("inside funcStep transformResponse")
	vars = trReqVars
	log.Print(response)
	//printResponseBody(response,"printing response from funcStep TransformResponse")
	//vars.ReqVars = reqVars
	//vars.ResVars = resVars

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
	//log.Println("++++++++++++++++++++++++++++++")
	for k, v := range vars.Vars {
		log.Println(k)
		log.Println(v)
	}

	tmplBodyFromRes := json.NewDecoder(response.Body)
	tmplBodyFromRes.DisallowUnknownFields()
	if err = tmplBodyFromRes.Decode(&vars.Body); err != nil {
		log.Println("tmplBodyFromRes.Decode error from func")
		log.Println(err)
		body, readErr := ioutil.ReadAll(tmplBodyFromRes.Buffered())
		if readErr != nil {
			err = readErr
			log.Println("ioutil.ReadAll(response.Body) error")
			log.Println(err)
			return
		}
		err = nil
		log.Print("string(body) = ", string(body))
		tempBody := make(map[string]string)
		tempBody["data"] = string(body)
		vars.Body = tempBody
	}
	vars.OrgBody = vars.Body
	//log.Print(vars.Body)
	if funcStep.TransformResponse != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = vars
		fvars.ResVars = resVars
		fvars.ReqVars = reqVars

		log.Print("fvars.ReqVars")
		log.Print(fvars.ReqVars["generateotp"])

		output, err := processTemplate(funcStep.RouteName, funcStep.TransformResponse, fvars, "json", funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
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
