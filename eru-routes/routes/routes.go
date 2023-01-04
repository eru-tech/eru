package routes

/// this is in eru-routes redesign branch
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-crypto/jwt"
	utils "github.com/eru-tech/eru/eru-utils"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	encodedForm   = "application/x-www-form-urlencoded"
	multiPartForm = "multipart/form-data"
)
const MatchTypePrefix = "PREFIX"
const MatchTypeExact = "EXACT"
const ConditionFailActionError = "ERROR"
const ConditionFailActionIgnore = "IGNORE"

type Authorizer struct {
	AuthorizerName string
	TokenHeaderKey string
	SecretAlgo     string
	JwkUrl         string
	Audience       []string
	Issuer         []string
}

type TokenSecret struct {
	HeaderKey  string
	SecretAlgo string
	SecretKey  string
	JwkUrl     string
	Audience   []string
	Issuer     []string
}
type Route struct {
	Condition            string
	ConditionFailMessage string
	ConditionFailAction  string
	Async                bool
	AsyncMessage         string
	LoopVariable         string
	LoopInParallel       bool
	RouteName            string `eru:"required"`
	RouteCategoryName    string
	Url                  string `eru:"required"`
	MatchType            string `eru:"required"`
	RewriteUrl           string
	TargetHosts          []TargetHost `eru:"required"`
	AllowedHosts         []string
	AllowedMethods       []string
	RequiredHeaders      []Headers
	EnableCache          bool
	RequestHeaders       []Headers
	QueryParams          []Headers
	FormData             []Headers
	FileData             []FilePart
	ResponseHeaders      []Headers
	TransformRequest     string
	TransformResponse    string
	IsPublic             bool
	Authorizer           string
	AuthorizerException  []string
	TokenSecret          TokenSecret `json:"-"`
	RemoveParams         RemoveParams
	OnError              string
	Redirect             bool
	RedirectUrl          string
	FinalRedirectUrl     string `json:"-"`
	RedirectScheme       string
	RedirectParams       []Headers
}

type RemoveParams struct {
	RequestHeaders  []string
	QueryParams     []string
	FormData        []string
	ResponseHeaders []string
}

type TargetHost struct {
	Host       string
	Port       string
	Method     string
	Scheme     string
	Allocation int64
}

type Headers struct {
	Key        string `eru:"required"`
	Value      string `eru:"required"`
	IsTemplate bool
}

type FilePart struct {
	FileName    string `eru:"required"`
	FileVarName string `eru:"required"`
	FileContent string `eru:"required"`
}

type TemplateVars struct {
	Headers          map[string]interface{}
	FormData         map[string]interface{}
	Params           map[string]interface{}
	Vars             map[string]interface{}
	Body             interface{}
	OrgBody          interface{}
	Token            interface{}
	FormDataKeyArray []string
	LoopVars         interface{}
	//ReqVars map[string]*TemplateVars
	//ResVars map[string]*TemplateVars
}

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func (authorizer Authorizer) VerifyToken(token string) (claims interface{}, err error) {
	claims, err = jwt.DecryptTokenJWK(token, authorizer.JwkUrl)
	if err != nil {
		return
	}
	return
}

func (route *Route) CheckPathException(path string) (bypass bool) {
	for _, v := range route.AuthorizerException {
		if v == path {
			return true
			break
		}
	}
	return false
}

func (route *Route) GetTargetSchemeHostPortPath(url string) (scheme string, host string, port string, path string, method string, err error) {
	targetHost, err := route.getTargetHost()
	if err != nil {
		log.Println(err)
		return
	}
	scheme = targetHost.Scheme
	port = targetHost.Port
	host = targetHost.Host
	method = targetHost.Method
	switch route.MatchType {
	case MatchTypePrefix:
		if url == "" {
			path = route.RewriteUrl
		} else {
			urlSplit := strings.Split(url, route.RouteName)
			if len(urlSplit) > 0 {
				path = fmt.Sprint(route.RewriteUrl, strings.TrimPrefix(urlSplit[1], route.Url))
			} else {
				path = route.RewriteUrl
			}
		}
	case MatchTypeExact:
		path = route.RewriteUrl
	default:
		//do nothing
	}
	log.Println("path = ", path)
	return
}

func (route *Route) Validate(host string, url string, method string, headers http.Header) (err error) {
	safeHost := true
	safeMethod := true

	for _, v := range route.RequiredHeaders {
		if headers.Get(v.Key) != v.Value {
			err = errors.New(fmt.Sprint("Wrong Value for Header Key : ", v.Key))
			return
		}
	}

	if len(route.AllowedHosts) > 0 {
		safeHost = false
		for i := 0; i < len(route.AllowedHosts); i++ {
			if route.AllowedHosts[i] == host {
				safeHost = true
				break
			}
		}
	}
	if !safeHost {
		err = errors.New("Host not allowed")
		return
	}
	//log.Println(route.AllowedMethods)
	//log.Println(method)
	if len(route.AllowedMethods) > 0 {
		safeMethod = false
		for i := 0; i < len(route.AllowedMethods); i++ {
			if route.AllowedMethods[i] == method {
				safeMethod = true
				break
			}
		}
	}
	if !safeMethod {
		err = errors.New("Method not allowed")
		return
	}

	if route.MatchType != MatchTypePrefix && route.MatchType != MatchTypeExact {
		err = errors.New(fmt.Sprint("Incorrect MatchType - needed ", MatchTypePrefix, " or ", MatchTypeExact, "."))
		return
	}

	if strings.HasPrefix(strings.ToUpper(url), "/PUBLIC") && !route.IsPublic {
		err = errors.New("route is not public")
		return
	}
	if route.MatchType == MatchTypePrefix && !strings.HasPrefix(strings.ToUpper(strings.Split(url, route.RouteName)[1]), strings.ToUpper(route.Url)) {
		err = errors.New("URL Prefix mismatch")
		return
	}

	if route.MatchType == MatchTypeExact && !strings.EqualFold(strings.Split(url, route.RouteName)[1], route.Url) {
		log.Println(url, " - ", route.RouteName, " - ", route.Url)
		err = errors.New("URL mismatch")
		return
	}
	return
}

func (route *Route) getTargetHost() (targetHost TargetHost, err error) {
	//log.Println(route)
	//TODO Random selection of target based on allocation
	if len(route.TargetHosts) > 0 {
		return route.TargetHosts[0], err
	}
	err = errors.New(fmt.Sprint("No Target Host defined for this route :", route.RouteName))
	return
}

func (route *Route) Execute(request *http.Request, url string, async bool, asyncMsg string) (response *http.Response, trResVar *TemplateVars, resErr error) {
	log.Println("*******************route execute start for ", route.RouteName, "*******************")
	//log.Println("url = ", url)
	//utils.PrintRequestBody(request, "printing request from route Execute")
	//log.Print(request.Header)
	var responses []*http.Response
	trReqVars := &TemplateVars{}
	var trResVars []*TemplateVars
	var errs []error
	err := loadRequestVars(trReqVars, request)
	if err != nil {
		log.Println(err)
		resErr = err
		response = errorResponse(resErr.Error(), request)
		return
	}

	if route.Condition != "" {
		avars := &FuncTemplateVars{}
		avars.Vars = trReqVars
		output, outputErr := processTemplate(route.RouteName, route.Condition, avars, "string", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
		log.Print(string(output))
		if outputErr != nil {
			log.Println(outputErr)
			resErr = outputErr
			response = errorResponse(resErr.Error(), request)
			return
		}
		strCond, strCondErr := strconv.Unquote(string(output))
		if strCondErr != nil {
			log.Println(strCondErr)
			resErr = err
			response = errorResponse(resErr.Error(), request)
			return
		}
		if strCond == "false" {
			cfmBody := "{}"
			if route.ConditionFailMessage != "" {
				cfmvars := &FuncTemplateVars{}
				cfmvars.Vars = trReqVars
				cfmOutput, cfmOutputErr := processTemplate(route.RouteName, route.ConditionFailMessage, avars, "json", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
				log.Print(string(cfmOutput))
				if cfmOutputErr != nil {
					log.Println(cfmOutputErr)
					resErr = err
					response = errorResponse(resErr.Error(), request)
					return
				}
				cfmBody = string(cfmOutput)
			}
			statusCode := http.StatusOK
			if route.ConditionFailAction == ConditionFailActionError {
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
			trResVar = &TemplateVars{}
			responses = append(responses, response)
			return
		}
	}

	var loopArray []interface{}
	if route.LoopVariable != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = trReqVars
		output, outputErr := processTemplate(route.RouteName, route.LoopVariable, fvars, "json", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
		log.Print(string(output))
		if outputErr != nil {
			log.Println(outputErr)
			resErr = err
			response = errorResponse(resErr.Error(), request)
			return
		}
		var loopJson interface{}
		loopJsonErr := json.Unmarshal(output, &loopJson)
		if loopJsonErr != nil {
			err = errors.New("route loop variable is not a json")
			log.Print(loopJsonErr)
			resErr = err
			response = errorResponse(resErr.Error(), request)
		}

		ok := false
		if loopArray, ok = loopJson.([]interface{}); !ok {
			err = errors.New("route loop variable is not an array")
			log.Print(err)
			resErr = err
			response = errorResponse(resErr.Error(), request)
			return
		}
		log.Print("loopArray = ", loopArray)

	} else {
		//dummy row added to create a job
		loopArray = append(loopArray, make(map[string]interface{}))
	}
	var jobs = make(chan Job, 10)
	var results = make(chan Result, 10)
	startTime := time.Now()
	log.Print("url before allocate = ", url)
	go allocate(request, url, trReqVars, loopArray, jobs, async, asyncMsg)
	done := make(chan bool)
	//go result(done,results,responses, trResVars,errs)

	go func(done chan bool, results chan Result) {
		defer func() {
			if r := recover(); r != nil {
				log.Print("goroutine paniqued Route Execute: ", r)
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
	if route.LoopInParallel && route.LoopVariable != "" {
		noOfWorkers = 5
		if len(loopArray) < noOfWorkers {
			noOfWorkers = len(loopArray)
		}
	}

	log.Print("noOfWorkers = ", noOfWorkers)
	createWorkerPool(route, noOfWorkers, jobs, results)
	<-done
	log.Print("after done")
	log.Print("len(responses) = ", len(responses))
	log.Print("len(trVars) = ", len(trResVars))
	log.Print(&trResVars)
	log.Print("len(errs) = ", len(errs))
	log.Print("calling clubResponses from route")
	response, trResVar, resErr = clubResponses(responses, trResVars, errs)
	endTime := time.Now()
	diff := endTime.Sub(startTime)
	fmt.Println("total time taken ", diff.Seconds(), "seconds")
	log.Println("*******************route execute end for ", route.RouteName, "*******************")
	return
}

func (route *Route) RunRoute(req *http.Request, url string, trReqVars *TemplateVars, async bool, asyncMsg string) (response *http.Response, trResVars *TemplateVars, err error) {
	log.Print("inside RunRoute")
	log.Print("url from RunRoute = ", url)
	log.Print(trReqVars.LoopVars)
	//clone request for parallel execution

	request, err := cloneRequest(req)
	if err != nil {
		log.Println("error from cloneRequest")
		log.Println(err)
		return
	}
	err = route.transformRequest(request, url, trReqVars)
	if err != nil {
		log.Println("error from transformRequest")
		log.Println(err)
		return
	}
	log.Println("Before httpClient.Do of route Execute")
	log.Print(trReqVars)

	//TODO commented below line - chk this and take it in route config
	//request.Header.Set("accept-encoding", "identity")

	//printRequestBody(request, "printing request Before httpClient.Do of route Execute")
	//log.Print("request.Host = ", request.Host)

	if request.Host != "" {
		if route.Async || async {
			//creating a new context with 1ms to give sufficient time to execute the http request and
			// timeout without waiting for the response
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()
			request = request.WithContext(ctx)
			_, err = utils.ExecuteHttp(request)

			respHeader := http.Header{}
			respHeader.Set("Content-Type", "application/json")

			if errors.Is(err, context.DeadlineExceeded) {
				//ignoring DeadlineExceeded error as we know it will timeout
				err = nil
			} else {
				return
			}
			body := "{}"

			if asyncMsg != "" {
				body = asyncMsg
			} else if route.AsyncMessage != "" {
				avars := &FuncTemplateVars{}
				avars.Vars = trReqVars
				output, outputErr := processTemplate(route.RouteName, route.AsyncMessage, avars, "json", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
				log.Print(string(output))
				if outputErr != nil {
					err = outputErr
					log.Println(err)
					return
				}
				body = string(output)
			}

			response = &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Body:       ioutil.NopCloser(bytes.NewBufferString(body)),

				ContentLength: int64(len(body)),
				Request:       request,
				Header:        respHeader,
			}

		} else {
			response, err = utils.ExecuteHttp(request)
			if err != nil {
				log.Println(" httpClient.Do error from route execute function")
				log.Println(err)
				return
			}
		}

		//response = <-routeChan

		//log.Println(response.Header)
		//log.Println(response.StatusCode)
		//log.Print(response.ContentLength)
		//printResponseBody(response, "printing response After httpClient.Do of route Execute before transformResponse")
	} else {
		response = &http.Response{Header: http.Header{}, StatusCode: http.StatusOK}
		rb, err := json.Marshal(make(map[string]interface{}))
		if err != nil {
			log.Println(err)
		} else {
			log.Print("making dummy response body")
			response.Body = ioutil.NopCloser(bytes.NewReader(rb))
		}
	}

	trResVars = &TemplateVars{}
	//if route.TransformResponse != "" {
	trResVars, err = route.transformResponse(response, trReqVars)

	if err != nil {
		log.Println(err)
		return
	}
	//printResponseBody(response, "printing response After httpClient.Do of route Execute after transformResponse")
	//}
	//log.Print("printing trResVars")
	//log.Print(trResVars)
	return
}

func (route *Route) transformRequest(request *http.Request, url string, vars *TemplateVars) (err error) {
	log.Println("inside route.transformRequest")
	//printRequestBody(request,"body from route transformRequest")

	//reqVarsLoaded := false
	//vars = &TemplateVars{}

	vars.FormData = make(map[string]interface{})
	vars.Body = make(map[string]interface{})
	vars.OrgBody = make(map[string]interface{})
	reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	if reqContentType == encodedForm || reqContentType == multiPartForm {
		vars.FormData["dummy"] = nil
		// addding the same so loadvars will get length > 0 and avoid processing body
		// this dummy record will get overwritten as part of return value from process multipart
	}

	//TODO check if commenting below block has any impact - loading vars only once now
	/*
		if !reqVarsLoaded {
			err = loadRequestVars(vars, request)
			if err != nil {
				log.Println(err)
				return
			}
			reqVarsLoaded = true
		}
	*/

	log.Print("url = ", url)
	scheme, host, port, path, method, err := route.GetTargetSchemeHostPortPath(url)
	if err != nil {
		return
	}

	if port != "" {
		port = fmt.Sprint(":", port)
	}

	// http: Request.RequestURI can't be set in client requests.
	// http://golang.org/src/pkg/net/http/client.go
	request.RequestURI = ""
	request.Host = host
	request.URL.Host = fmt.Sprint(host, port)
	request.URL.Path = path
	request.URL.Scheme = scheme
	request.Method = method

	for _, h := range route.RequestHeaders {
		if !h.IsTemplate {
			request.Header.Set(h.Key, h.Value)
		}
	}
	multiPart := false
	//reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	log.Print("reqContentType from makeMultipart = ", reqContentType)
	if reqContentType == multiPartForm {
		multiPart = true
		mpvars := &FuncTemplateVars{}

		/*
			// TODO - need to reopen this comment block if converting a json request to formdata
			body, err3 := ioutil.ReadAll(request.Body)
			if err3 != nil {
				err = err3
				log.Print("error in ioutil.ReadAll(request.Body)")
				log.Println(err)
				return
			}
			log.Print(vars)
			err = json.Unmarshal(body, &vars.Body)
			if err != nil {
				log.Print("error in json.Unmarshal(body, &vars.Body)")
				log.Println(err)
				return &TemplateVars{}, err
			}
		*/
		mpvars.Vars = vars
		routesFormData := route.FormData
		for i, fd := range routesFormData {
			if fd.IsTemplate {
				log.Print("inside route.FormData")
				log.Print(fd.Key)
				output, err := processTemplate(fd.Key, fd.Value, mpvars, "string", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
				if err != nil {
					log.Println(err)
					return err
				}
				log.Print("form data template processed")
				outputStr, err := strconv.Unquote(string(output))
				if err != nil {
					log.Println(err)
					return err
				}
				log.Print(outputStr)
				routesFormData[i].Value = outputStr
			}
		}
		vars.FormData, vars.FormDataKeyArray, err = processMultipart(request, route.RemoveParams.FormData, routesFormData)
		if err != nil {
			log.Print("printing error recd from processMultipart")
			log.Print(err)
			return
		}
		log.Print("vars.FormData")
		log.Print(vars.FormData)
	} else if reqContentType == encodedForm {
		rpfErr := request.ParseForm()
		if rpfErr != nil {
			err = rpfErr
			log.Print("error from request.ParseForm() = ", err.Error())
			return
		}
		if request.PostForm != nil {
			for k, v := range request.PostForm {
				vars.FormData[k] = strings.Join(v, ",")
			}
		}
	}

	err = processParams(request, route.RemoveParams.QueryParams, route.QueryParams, vars, nil, nil)
	if err != nil {
		return
	}
	//log.Print("multiPart =" , !multiPart)
	if !multiPart || route.TransformRequest != "" {
		log.Println("route.TransformRequest = ", route.TransformRequest)
		//vars := module_model.TemplateVars{}
		if route.TransformRequest != "" {

			/*if !reqVarsLoaded {
				err = loadRequestVars(vars, request)
				if err != nil {
					log.Println(err)
					return
				}
				reqVarsLoaded = true
			}

			*/
			fvars := &FuncTemplateVars{}
			fvars.Vars = vars
			output, err := processTemplate(route.RouteName, route.TransformRequest, fvars, "json", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
			if err != nil {
				log.Println(err)
				return err
			}
			err = json.Unmarshal(output, &vars.Body)
			if err != nil {
				log.Println(err)
				return err
			}
			vars.OrgBody = vars.Body
			request.Body = ioutil.NopCloser(bytes.NewBuffer(output))
			request.Header.Set("Content-Length", strconv.Itoa(len(output)))
			request.ContentLength = int64(len(output))

		} else {
			//log.Print("inside else")
			body, err3 := ioutil.ReadAll(request.Body)
			if err3 != nil {
				err = err3
				log.Print("error in ioutil.ReadAll(request.Body)")
				log.Println(err)
				return
			}
			vars.OrgBody = vars.Body
			err = json.Unmarshal(body, &vars.Body)
			if err != nil {
				log.Print("error in json.Unmarshal(body, &vars.Body)")
				log.Println(err)
				return err
			}
			//log.Println("body from route transformRequest - else part")
			//log.Println(string(body))
			//log.Println(request.Header.Get("Content-Length"))
			request.Body = ioutil.NopCloser(bytes.NewReader(body))
			//request.Header.Set("Content-Length", strconv.Itoa(len(body)))
			//request.ContentLength = int64(len(body))
		}
	}

	err = processHeaderTemplates(request, route.RemoveParams.RequestHeaders, route.RequestHeaders, true, vars, route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl, nil, nil)
	if err != nil {
		log.Print("error from processHeaderTemplates")
		return
	}
	return
}

func (route *Route) transformResponse(response *http.Response, trReqVars *TemplateVars) (trResVars *TemplateVars, err error) {
	trResVars = &TemplateVars{}
	log.Println("inside transformResponse")
	//a, e := json.Marshal(trReqVars)
	//log.Print(string(a))
	//log.Print(e)
	log.Print("route.Redirect for route ", route.RouteName, "is ", route.Redirect)
	if route.Redirect {
		log.Print("inside route.Redirect")
		finalRedirectUrl := route.RedirectUrl
		fvars := &FuncTemplateVars{}
		fvars.Vars = trReqVars
		redirectUrlBytes, rubErr := processTemplate(route.RouteName, route.RedirectUrl, fvars, "string", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
		if rubErr != nil {
			err = rubErr
			log.Print("error from processHeaderTemplates = ", err.Error())
			return
		}
		finalRedirectUrl, err = strconv.Unquote(string(redirectUrlBytes))
		if err != nil {
			log.Print("error from strconv.Unquote(string(redirectUrlBytes)) = ", err.Error())
			finalRedirectUrl = string(redirectUrlBytes)
		}

		paramStr := ""
		for _, v := range route.RedirectParams {
			if paramStr == "" {
				paramStr = "?"
			} else {
				paramStr = fmt.Sprint(paramStr, "&")
			}
			finalParamValue := v.Value
			if v.IsTemplate {
				paramValue, rptErr := processTemplate(v.Key, v.Value, fvars, "string", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
				if err != nil {
					err = rptErr
					log.Print("error from RedirectParams processTemplate = ", err.Error())
					return
				}
				finalParamValue, err = strconv.Unquote(string(paramValue))
				if err != nil {
					log.Print("error from strconv.Unquote(string(paramValue)) = ", err.Error())
					finalParamValue = string(paramValue)
				}
			}
			paramStr = fmt.Sprint(paramStr, v.Key, "=", finalParamValue)
		}
		route.FinalRedirectUrl = fmt.Sprint(route.RedirectScheme, "://", finalRedirectUrl, paramStr)
		log.Print("route.FinalRedirectUrl =", route.FinalRedirectUrl)
		return
	}

	//printResponseBody(response,"printing response from route TransformResponse")
	for _, h := range route.ResponseHeaders {
		response.Header.Set(h.Key, h.Value)
	}

	log.Println("TransformResponse = ", route.TransformResponse)
	trResVars.Headers = make(map[string]interface{})
	for k, v := range response.Header {
		trResVars.Headers[k] = v
	}
	trResVars.Params = make(map[string]interface{})
	trResVars.Vars = make(map[string]interface{})
	if trReqVars.Vars == nil {
		trReqVars.Vars = make(map[string]interface{})
	}
	trReqVars.Vars["Body"] = trReqVars.Body
	trReqVars.Vars["OrgBody"] = trReqVars.OrgBody

	trResVars.Vars = trReqVars.Vars
	var res interface{}
	//log.Print(response.Body)
	tmplBodyFromRes := json.NewDecoder(response.Body)
	tmplBodyFromRes.DisallowUnknownFields()
	//log.Print("tmplBodyFromRes = ", tmplBodyFromRes)
	if err = tmplBodyFromRes.Decode(&res); err != nil {
		log.Println("tmplBodyFromRes.Decode error from routes")
		log.Println(err)
		body, readErr := ioutil.ReadAll(response.Body)
		if readErr != nil {
			err = readErr
			log.Println("ioutil.ReadAll(response.Body) error")
			log.Println(err)
			return
		}
		tempBody := make(map[string]string)
		tempBody["data"] = string(body)
		res = tempBody
	}
	//log.Print(res)
	rb, err := json.Marshal(res)
	if err != nil {
		log.Println(err)
		return &TemplateVars{}, err
	}
	err = json.Unmarshal(rb, &trResVars.Body)
	if err != nil {
		log.Println(err)
		return &TemplateVars{}, err
	}
	trResVars.OrgBody = trResVars.Body
	//log.Print(trResVars)
	if route.TransformResponse != "" {

		fvars := &FuncTemplateVars{}
		fvars.Vars = trResVars
		//log.Print(fvars.Vars.Body)
		output, err := processTemplate(route.RouteName, route.TransformResponse, fvars, "json", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
		if err != nil {
			log.Println(err)
			return &TemplateVars{}, err
		}
		response.Body = ioutil.NopCloser(bytes.NewBuffer(output))
		response.Header.Set("Content-Length", strconv.Itoa(len(output)))
		response.ContentLength = int64(len(output))

		err = json.Unmarshal(output, &trResVars.Body)
		if err != nil {
			log.Println(err)
			return &TemplateVars{}, err
		}
	} else {
		response.Body = ioutil.NopCloser(bytes.NewBuffer(rb))
		response.Header.Set("Content-Length", strconv.Itoa(len(rb)))
		response.ContentLength = int64(len(rb))
	}
	if route.RemoveParams.ResponseHeaders != nil {
		for _, v := range route.RemoveParams.ResponseHeaders {
			response.Header.Del(v)
		}
	}
	return
}
