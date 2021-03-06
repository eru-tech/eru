package routes

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	RouteName         string
	Path              string
	Route             Route `json:"-"`
	RequestHeaders    []Headers
	QueryParams       []Headers
	FormData          []Headers
	FileData          []FilePart
	ResponseHeaders   []Headers
	TransformRequest  string
	TransformResponse string
	IsPublic          bool
	RemoveParams      RemoveParams
	FuncSteps         map[string]*FuncStep
}

func (funcGroup *FuncGroup) Execute(request *http.Request) (response *http.Response, err error) {
	log.Println("inside funGroup Execute")
	reqVars := make(map[string]*TemplateVars)
	resVars := make(map[string]*TemplateVars)
	for _, v := range funcGroup.FuncSteps {
		response, err = v.execute(request, reqVars, resVars, v.RouteName)
	}
	return
}

func (funcStep *FuncStep) execute(request *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string) (response *http.Response, err error) {
	log.Println("*******************funcStep execute start for ", funcStep.RouteName, " *******************")
	req, vars, err := funcStep.transformRequest(request, reqVars, resVars, mainRouteName)
	if err != nil {
		log.Println(err)
		return
	}
	reqVars[funcStep.RouteName] = vars

	//printRequestBody(req,"body from funcStep execute")

	routevars := &TemplateVars{}
	_ = routevars
	response, routevars, err = funcStep.Route.Execute(req, funcStep.Path)
	if err != nil {
		return
	}
	if funcStep.Route.OnError == "STOP" && response.StatusCode >= 400 {
		return
	}
	resVars[funcStep.RouteName] = routevars
	//log.Println("resVars[funcStep.RouteName] for ",funcStep.RouteName)
	//log.Println(resVars[funcStep.RouteName])

	//response *http.Response, trReqVars TemplateVars, resHeaders []Headers, removeHeaders []string, templateName string, templateString string,tokenHeaderKey string,jwkUrl string) (trResVars TemplateVars , err error)
	resVars[funcStep.RouteName], err = funcStep.transformResponse(response, resVars[funcStep.RouteName], reqVars, resVars)
	if err != nil {
		return
	}

	//log.Println("resVars[funcStep.RouteName] for ",funcStep.RouteName, " after funcStep.transformResponse")
	//log.Println(resVars[funcStep.RouteName])

	for _, cv := range funcStep.FuncSteps {
		response, err = cv.execute(request, reqVars, resVars, mainRouteName)
	}
	log.Println("*******************funcStep execute end for ", funcStep.RouteName, " *******************")
	return
}

func cloneRequest(request *http.Request) (req *http.Request, err error) {
	log.Println("clone request")
	req = request.Clone(request.Context())

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
		body, err3 := ioutil.ReadAll(request.Body)
		if err3 != nil {
			err = err3
			log.Println(err)
			return
		}
		//log.Println("body from clonerequest - else part")
		//log.Println(string(body))
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
	}

	return
}

func (funcStep *FuncStep) transformRequest(request *http.Request, reqVars map[string]*TemplateVars, resVars map[string]*TemplateVars, mainRouteName string) (req *http.Request, vars *TemplateVars, err error) {
	log.Println("inside funcStep.transformRequest")
	makeMultiPartCalled := false
	//printRequestBody(request,"body from funcStep transformRequest")
	defer request.Body.Close()
	vars = &TemplateVars{}
	if mainRouteName == funcStep.RouteName {
		err = loadRequestVars(vars, request)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		log.Println("vars = reqVars[mainRouteName]")
		vars = reqVars[mainRouteName]
	}

	oldContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	req, err = cloneRequest(request)

	//printRequestBody(req,"body from funcstep transformRequest - after clone")
	if err != nil {
		log.Println(err)
		return req, &TemplateVars{}, err
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

	if (newContentType == encodedForm || newContentType == multiPartForm) && newContentType != oldContentType {
		log.Println(vars.FormData)
		vars.FormData, vars.FormDataKeyArray, err = makeMultipart(req, funcStep.FormData, funcStep.FileData, vars, reqVars, resVars, funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
		makeMultiPartCalled = true
		if err != nil {
			return
		}
	}

	err = processParams(req, funcStep.RemoveParams.QueryParams, funcStep.QueryParams)
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
		output, err := processTemplate(funcStep.RouteName, funcStep.TransformRequest, fvars, "json", funcStep.Route.TokenSecret.HeaderKey, funcStep.Route.TokenSecret.JwkUrl)
		if err != nil {
			log.Println(err)
			return req, &TemplateVars{}, err
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(output))
		req.Header.Set("Content-Length", strconv.Itoa(len(output)))
		req.ContentLength = int64(len(output))
	} else if !makeMultiPartCalled {
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

	printRequestBody(req, "body from funcstep transformRequest - after else part")
	defer req.Body.Close()
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
		log.Println("tmplBodyFromRes.Decode error")
		log.Println(err)
		return
	}
	if funcStep.TransformResponse != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = vars
		fvars.ResVars = resVars
		fvars.ReqVars = reqVars
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
