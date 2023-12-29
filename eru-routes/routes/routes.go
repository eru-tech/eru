package routes

/// this is in eru-routes redesign branch
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	//"github.com/eru-tech/eru/eru-crypto/jwt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	encodedForm     = "application/x-www-form-urlencoded"
	multiPartForm   = "multipart/form-data"
	applicationjson = "application/json"
)
const MatchTypePrefix = "PREFIX"
const MatchTypeExact = "EXACT"
const ConditionFailActionError = "ERROR"
const ConditionFailActionIgnore = "IGNORE"

//type Authorizer struct {
//	AuthorizerName string
//	TokenHeaderKey string
//	SecretAlgo     string
//	JwkUrl         string
//	Audience       []string
//	Issuer         []string
//}

//	type TokenSecret struct {
//		HeaderKey  string
//		SecretAlgo string
//		SecretKey  string
//		JwkUrl     string
//		Audience   []string
//		Issuer     []string
//	}
type Route struct {
	Condition            string       `json:"condition"`
	ConditionFailMessage string       `json:"condition_fail_message"`
	ConditionFailAction  string       `json:"condition_fail_action"`
	Async                bool         `json:"async"`
	AsyncMessage         string       `json:"async_message"`
	LoopVariable         string       `json:"loop_variable"`
	LoopInParallel       bool         `json:"loop_in_parallel"`
	RouteName            string       `json:"route_name" eru:"required"`
	RouteCategoryName    string       `json:"route_category_name"`
	Url                  string       `json:"url" eru:"required"`
	MatchType            string       `json:"match_type" eru:"required"`
	RewriteUrl           string       `json:"rewrite_url"`
	TargetHosts          []TargetHost `json:"target_hosts" eru:"required"`
	AllowedHosts         []string     `json:"allowed_hosts"`
	AllowedMethods       []string     `json:"allowed_methods"`
	RequiredHeaders      []Headers    `json:"required_headers"`
	//EnableCache          bool         `json:"enable_cache"`
	RequestHeaders    []Headers  `json:"request_headers"`
	QueryParams       []Headers  `json:"query_params"`
	FormData          []Headers  `json:"form_data"`
	FileData          []FilePart `json:"file_data"`
	ResponseHeaders   []Headers  `json:"response_headers"`
	TransformRequest  string     `json:"transform_request"`
	TransformResponse string     `json:"transform_response"`
	//IsPublic             bool `json:"is_public"`
	//Authorizer           string   `json:"-"`
	//AuthorizerException  []string `json:"-"`
	TokenSecretKey   string       `json:"-"`
	RemoveParams     RemoveParams `json:"remove_params"`
	OnError          string       `json:"on_error"`
	Redirect         bool         `json:"redirect"`
	RedirectUrl      string       `json:"redirect_url"`
	FinalRedirectUrl string       `json:"-"`
	RedirectScheme   string       `json:"redirect_scheme"`
	RedirectParams   []Headers    `json:"redirect_params"`
}

type RemoveParams struct {
	RequestHeaders  []string `json:"request_headers"`
	QueryParams     []string `json:"query_params"`
	FormData        []string `json:"form_data"`
	ResponseHeaders []string `json:"response_headers"`
}

type TargetHost struct {
	Host       string `json:"host"`
	Port       string `json:"port"`
	Method     string `json:"method"`
	Scheme     string `json:"scheme"`
	Allocation int64  `json:"allocation"`
}

type Headers struct {
	Key        string `json:"key" eru:"required"`
	Value      string `json:"value"eru:"required"`
	IsTemplate bool   `json:"is_template"`
}

type FilePart struct {
	FileName    string `json:"file_name" eru:"required"`
	FileVarName string `json:"file_var_name" eru:"required"`
	FileContent string `json:"file_content" eru:"required"`
}

type TemplateVars struct {
	Headers          map[string]interface{}
	FormData         map[string]interface{}
	FileData         []FilePart
	Params           map[string]interface{}
	Vars             map[string]interface{}
	Body             interface{}
	OrgBody          interface{}
	Token            interface{}
	FormDataKeyArray []string
	LoopVars         interface{}
	LoopVar          interface{}
	//ReqVars map[string]*TemplateVars
	//ResVars map[string]*TemplateVars
}

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

//func (authorizer Authorizer) VerifyToken(ctx context.Context, token string) (claims interface{}, err error) {
//	claims, err = jwt.DecryptTokenJWK(ctx, token, authorizer.JwkUrl)
//	if err != nil {
//		return
//	}
//	return
//}

//	func (route *Route) CheckPathException(path string) (bypass bool) {
//		for _, v := range route.AuthorizerException {
//			if v == path {
//				return true
//			}
//		}
//		return false
//	}
func (route *Route) Clone(ctx context.Context) (cloneRoute *Route, err error) {
	cloneRouteI, cloneRouteIErr := cloneInterface(ctx, route)
	if cloneRouteIErr != nil {
		err = cloneRouteIErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	cloneRouteOk := false
	cloneRoute, cloneRouteOk = cloneRouteI.(*Route)
	if !cloneRouteOk {
		err = errors.New("route cloning failed")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return
}
func (route *Route) GetTargetSchemeHostPortPath(ctx context.Context, url string) (scheme string, host string, port string, path string, method string, err error) {
	logs.WithContext(ctx).Debug("GetTargetSchemeHostPortPath - Start")
	targetHost, err := route.getTargetHost(ctx)
	if err != nil {
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
			if len(urlSplit) > 1 {
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
	return
}

func (route *Route) Validate(ctx context.Context, host string, url string, method string, headers http.Header) (err error) {
	logs.WithContext(ctx).Debug("Validate - Start")
	safeHost := true
	safeMethod := true

	for _, v := range route.RequiredHeaders {
		if headers.Get(v.Key) != v.Value {
			err = errors.New(fmt.Sprint("Wrong Value for Header Key : ", v.Key))
			logs.WithContext(ctx).Error(err.Error())
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
		logs.WithContext(ctx).Error(err.Error())
		return
	}

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
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	if route.MatchType != MatchTypePrefix && route.MatchType != MatchTypeExact {
		err = errors.New(fmt.Sprint("Incorrect MatchType - needed ", MatchTypePrefix, " or ", MatchTypeExact, "."))
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	//if strings.HasPrefix(strings.ToUpper(url), "/PUBLIC") && !route.IsPublic {
	//	err = errors.New("route is not public")
	//	logs.WithContext(ctx).Error(err.Error())
	//	return
	//}
	logs.WithContext(ctx).Info(fmt.Sprint(strings.Split(url, route.RouteName)))
	logs.WithContext(ctx).Info(route.Url)
	if route.MatchType == MatchTypePrefix && !strings.HasPrefix(strings.ToUpper(strings.Split(url, route.RouteName)[1]), strings.ToUpper(route.Url)) {
		err = errors.New("URL Prefix mismatch")
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	if route.MatchType == MatchTypeExact && !strings.EqualFold(strings.Split(url, route.RouteName)[1], route.Url) {
		err = errors.New(fmt.Sprint("URL mismatch : ", url, " - ", route.RouteName, " - ", route.Url))
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return
}

func (route *Route) getTargetHost(ctx context.Context) (targetHost TargetHost, err error) {
	logs.WithContext(ctx).Debug("getTargetHost - Start")
	//TODO Random selection of target based on allocation
	if len(route.TargetHosts) > 0 {
		return route.TargetHosts[0], err
	}
	err = errors.New(fmt.Sprint("No Target Host defined for this route :", route.RouteName))
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (route *Route) Execute(ctx context.Context, request *http.Request, url string, async bool, asyncMsg string, trReqVars *TemplateVars, loopThread int) (response *http.Response, trResVar *TemplateVars, resErr error) {
	logs.WithContext(ctx).Info(fmt.Sprint("Route Execute - Start : ", route.RouteName))

	if trReqVars == nil {
		trReqVars = &TemplateVars{}
	}
	var responses []*http.Response
	var trResVars []*TemplateVars
	var errs []error

	resErr = route.transformRequest(ctx, request, url, trReqVars)
	if resErr != nil {
		return
	}

	if route.Condition != "" {
		avars := &FuncTemplateVars{}
		avars.Vars = trReqVars
		output, outputErr := processTemplate(ctx, route.RouteName, route.Condition, avars, "string", route.TokenSecretKey)
		if outputErr != nil {
			resErr = outputErr
			response = errorResponse(ctx, resErr.Error(), request)
			return
		}
		strCond, strCondErr := strconv.Unquote(string(output))
		if strCondErr != nil {
			logs.WithContext(ctx).Error(strCondErr.Error())
			resErr = strCondErr
			response = errorResponse(ctx, resErr.Error(), request)
			return
		}
		if strCond == "false" {
			cfmBody := "{}"
			if route.ConditionFailMessage != "" {
				cfmvars := &FuncTemplateVars{}
				cfmvars.Vars = trReqVars
				cfmOutput, cfmOutputErr := processTemplate(ctx, route.RouteName, route.ConditionFailMessage, avars, "json", route.TokenSecretKey)
				if cfmOutputErr != nil {
					resErr = cfmOutputErr
					response = errorResponse(ctx, resErr.Error(), request)
					return
				}
				cfmBody = string(cfmOutput)
			}
			statusCode := http.StatusOK
			if route.ConditionFailAction == ConditionFailActionError {
				statusCode = http.StatusBadRequest
			}

			condRespHeader := http.Header{}
			condRespHeader.Set("Content-Type", applicationjson)
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
			trResVar = &TemplateVars{}
			responses = append(responses, response)
			return
		}
	}

	var loopArray []interface{}
	lerr := false
	logs.WithContext(ctx).Info(fmt.Sprint("route.LoopVariable = ", route.LoopVariable))
	if route.LoopVariable != "" {
		loopArray, lerr = trReqVars.LoopVars.([]interface{})
		if !lerr {
			resErr = errors.New("func loop variable is not an array")
			logs.WithContext(ctx).Error(resErr.Error())
			response = errorResponse(ctx, resErr.Error(), request)
			return
		}
	} else {
		//dummy row added to create a job
		loopArray = append(loopArray, make(map[string]interface{}))
	}

	var jobs = make(chan Job, 10)
	var results = make(chan Result, 10)
	//startTime := time.Now()
	go allocate(ctx, request, url, trReqVars, loopArray, jobs, async, asyncMsg)
	done := make(chan bool)
	//go result(done,results,responses, trResVars,errs)

	go func(done chan bool, results chan Result) {
		defer func() {
			if r := recover(); r != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in Route Execute: ", r))
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
	if route.LoopInParallel && route.LoopVariable != "" {
		noOfWorkers = loopThread
		if len(loopArray) < noOfWorkers {
			noOfWorkers = len(loopArray)
		}
	}

	createWorkerPool(ctx, route, noOfWorkers, jobs, results)
	<-done
	response, trResVar, resErr = clubResponses(ctx, responses, trResVars, errs)
	//endTime := time.Now()
	//diff := endTime.Sub(startTime)
	//logs.WithContext(ctx).Info(fmt.Sprint("total time taken ", diff.Seconds(), "seconds"))
	logs.WithContext(ctx).Info(fmt.Sprint("Route Execute - End : ", route.RouteName))
	return
}

func (route *Route) RunRoute(ctx context.Context, req *http.Request, url string, trReqVars *TemplateVars, async bool, asyncMsg string) (response *http.Response, trResVars *TemplateVars, err error) {
	logs.WithContext(ctx).Debug("RunRoute - Start : ")
	//TODO commented below line - chk this and take it in route config
	//request.Header.Set("accept-encoding", "identity")

	request := req
	if route.LoopVariable != "" {
		if route.LoopInParallel {
			// cloning request for parallel execution of loop
			request, err = CloneRequest(ctx, req)
			if err != nil {
				return
			}
		}
		if trReqVars == nil {
			trReqVars = &TemplateVars{}
		}
		err = route.transformRequest(ctx, request, url, trReqVars)
		if err != nil {
			return
		}
	}

	if request.Host != "" {
		if route.Async || async {
			//creating a new context with 1ms to give sufficient time to execute the http request and
			// timeout without waiting for the response

			ctxAsync, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
			defer cancel()
			request = request.WithContext(ctxAsync)
			_, err = utils.ExecuteHttp(ctx, request)

			respHeader := http.Header{}
			respHeader.Set("Content-Type", applicationjson)

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
				output, outputErr := processTemplate(ctx, route.RouteName, route.AsyncMessage, avars, "json", route.TokenSecretKey)
				if outputErr != nil {
					err = outputErr
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
				Body:       io.NopCloser(bytes.NewBufferString(body)),

				ContentLength: int64(len(body)),
				Request:       request,
				Header:        respHeader,
			}

		} else {
			utils.PrintRequestBody(ctx, request, "printing request just before utils.ExecuteHttp")
			response, err = utils.ExecuteHttp(ctx, request)
			if err != nil {
				return
			}
			utils.PrintResponseBody(ctx, response, "printing response immediately after utils.ExecuteHttp")
		}
	} else {
		header := http.Header{}
		header.Set("Content-type", applicationjson)
		response = &http.Response{Header: header, StatusCode: http.StatusOK}
		rb, err := json.Marshal(make(map[string]interface{}))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		} else {
			logs.WithContext(ctx).Info("making dummy response body")
			response.Body = io.NopCloser(bytes.NewReader(rb))
		}
	}

	trResVars = &TemplateVars{}
	trResVars, err = route.transformResponse(ctx, response, trReqVars)
	if err != nil {
		return
	}
	return
}

func (route *Route) transformRequest(ctx context.Context, request *http.Request, url string, vars *TemplateVars) (err error) {
	logs.WithContext(ctx).Debug("transformRequest - Start : ")

	if vars.FormData == nil {
		vars.FormData = make(map[string]interface{})
	}

	if vars.Body == nil {
		vars.Body = make(map[string]interface{})
		vars.OrgBody = make(map[string]interface{})

		err = loadRequestVars(ctx, vars, request)
		if err != nil {
			return
		}
	}
	reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]

	scheme, host, port, path, method, err := route.GetTargetSchemeHostPortPath(ctx, url)
	if err != nil {
		return
	}

	if port != "" {
		port = fmt.Sprint(":", port)
	}

	var loopArray []interface{}
	if route.LoopVariable != "" {
		fvars := &FuncTemplateVars{}
		fvars.Vars = vars
		logs.WithContext(ctx).Info(fmt.Sprint("route.LoopVariable == ", route.LoopVariable))
		output, outputErr := processTemplate(ctx, route.RouteName, route.LoopVariable, fvars, "json", route.TokenSecretKey)
		if outputErr != nil {
			err = outputErr
			return
		}
		logs.WithContext(ctx).Info(fmt.Sprint("output of loopvariable after processtemplate : ", string(output)))
		var loopJson interface{}
		loopJsonErr := json.Unmarshal(output, &loopJson)
		if loopJsonErr != nil {
			err = errors.New("route loop variable is not a json")
			logs.WithContext(ctx).Error(fmt.Sprint(err.Error(), " : loopJsonErr"))
			return
		}
		ok := false
		if loopArray, ok = loopJson.([]interface{}); !ok {
			err = errors.New("route loop variable is not an array")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		logs.WithContext(ctx).Info(fmt.Sprint("loopArray = ", loopArray))

	} else {
		//dummy row added to create a job
		loopArray = append(loopArray, make(map[string]interface{}))
	}
	vars.LoopVars = loopArray

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

	if reqContentType == multiPartForm {
		multiPart = true
		mpvars := &FuncTemplateVars{}

		/*
			// TODO - need to reopen this comment block if converting a json request to formdata
			body, err3 := io.ReadAll(request.Body)
			if err3 != nil {
				err = err3
				return
			}
			err = json.Unmarshal(body, &vars.Body)
			if err != nil {
				return &TemplateVars{}, err
			}
		*/
		mpvars.Vars = vars
		routesFormData := route.FormData

		for _, fd := range routesFormData {
			if fd.IsTemplate {
				output, err := processTemplate(ctx, fd.Key, fd.Value, mpvars, "string", route.TokenSecretKey)
				if err != nil {
					return err
				}
				outputStr, err := strconv.Unquote(string(output))
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				logs.WithContext(ctx).Info(outputStr)
				vars.FormData[fd.Key] = outputStr
			} else {
				vars.FormData[fd.Key] = fd.Value
			}
		}

		/*for k, v := range vars.FormData {
			h := Headers{}
			h.Key = k
			h.Value = v.(string)
			routesFormData = append(routesFormData, h)
		}

		*/
		utils.PrintRequestBody(ctx, request, "printing request from route.transformRequest before multipart processing")
		vars.FormData, vars.FormDataKeyArray, vars.FileData, err = processMultipart(ctx, reqContentType, request, route.RemoveParams.FormData, vars.FormData, vars.FileData)
		if err != nil {
			return
		}
		utils.PrintRequestBody(ctx, request, "printing request from route.transformRequest after multipart processing")
	} else if reqContentType == encodedForm {
		rpfErr := request.ParseForm()
		if rpfErr != nil {
			err = rpfErr
			logs.WithContext(ctx).Error(fmt.Sprint("error from request.ParseForm() = ", err.Error()))
			return
		}
		if request.PostForm != nil {
			for k, v := range request.PostForm {
				vars.FormData[k] = strings.Join(v, ",")
			}
		}
	}

	err = processParams(ctx, request, route.RemoveParams.QueryParams, route.QueryParams, vars, nil, nil)
	if err != nil {
		return
	}
	if !multiPart || route.TransformRequest != "" {
		logs.WithContext(ctx).Info(fmt.Sprint("route.TransformRequest = ", route.TransformRequest))
		if route.TransformRequest != "" {

			/*if !reqVarsLoaded {
				err = loadRequestVars(vars, request)
				if err != nil {
					return
				}
				reqVarsLoaded = true
			}

			*/
			fvars := &FuncTemplateVars{}
			fvars.Vars = vars
			output, err := processTemplate(ctx, route.RouteName, route.TransformRequest, fvars, "json", route.TokenSecretKey)
			if err != nil {
				return err
			}
			logs.Logger.Info("printing request body after processing template")
			logs.Logger.Info(string(output))
			err = json.Unmarshal(output, &vars.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			request.Body = io.NopCloser(bytes.NewBuffer(output))
			request.Header.Set("Content-Length", strconv.Itoa(len(output)))
			request.ContentLength = int64(len(output))

		} else {
			body, err3 := io.ReadAll(request.Body)
			if err3 != nil {
				err = err3
				logs.WithContext(ctx).Error(fmt.Sprint("error in io.ReadAll(request.Body) : ", err.Error()))
				return
			}
			if len(body) > 0 {
				err = json.Unmarshal(body, &vars.Body)
				if err != nil {
					logs.WithContext(ctx).Error(fmt.Sprint("error in json.Unmarshal(body, &vars.Body) : ", err.Error()))
					return err
				}
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
			//request.Header.Set("Content-Length", strconv.Itoa(len(body)))
			//request.ContentLength = int64(len(body))
		}
	}

	err = processHeaderTemplates(ctx, request, route.RemoveParams.RequestHeaders, route.RequestHeaders, true, vars, route.TokenSecretKey, nil, nil)
	if err != nil {
		return
	}
	return
}

func (route *Route) transformResponse(ctx context.Context, response *http.Response, trReqVars *TemplateVars) (trResVars *TemplateVars, err error) {
	logs.WithContext(ctx).Debug("transformResponse - Start : ")
	trResVars = &TemplateVars{}

	logs.WithContext(ctx).Info(fmt.Sprint("route.Redirect for route ", route.RouteName, " is ", route.Redirect))
	if route.Redirect {
		finalRedirectUrl := route.RedirectUrl
		fvars := &FuncTemplateVars{}
		fvars.Vars = trReqVars
		redirectUrlBytes, rubErr := processTemplate(ctx, route.RouteName, route.RedirectUrl, fvars, "string", route.TokenSecretKey)
		if rubErr != nil {
			err = rubErr
			return
		}
		finalRedirectUrl, err = strconv.Unquote(string(redirectUrlBytes))
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("error from strconv.Unquote(string(redirectUrlBytes)) = ", err.Error()))
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
				paramValue, rptErr := processTemplate(ctx, v.Key, v.Value, fvars, "string", route.TokenSecretKey)
				if err != nil {
					err = rptErr
					return
				}
				finalParamValue, err = strconv.Unquote(string(paramValue))
				if err != nil {
					logs.WithContext(ctx).Error(fmt.Sprint("error from strconv.Unquote(string(paramValue)) = ", err.Error()))
					finalParamValue = string(paramValue)
				}
			}
			paramStr = fmt.Sprint(paramStr, v.Key, "=", finalParamValue)
		}
		route.FinalRedirectUrl = fmt.Sprint(route.RedirectScheme, "://", finalRedirectUrl, paramStr)
		logs.WithContext(ctx).Info(fmt.Sprint("route.FinalRedirectUrl =", route.FinalRedirectUrl))
		return
	}

	for _, h := range route.ResponseHeaders {
		response.Header.Set(h.Key, h.Value)
	}

	logs.WithContext(ctx).Info(fmt.Sprint("TransformResponse = ", route.TransformResponse))
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
	reqContentType := strings.Split(response.Header.Get("Content-type"), ";")[0]
	if reqContentType == applicationjson {
		var res interface{}
		tmplBodyFromRes := json.NewDecoder(response.Body)
		tmplBodyFromRes.DisallowUnknownFields()
		if err = tmplBodyFromRes.Decode(&res); err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("tmplBodyFromRes.Decode error from routes : ", err.Error()))
			body, readErr := io.ReadAll(response.Body)
			if readErr != nil {
				err = readErr
				logs.WithContext(ctx).Error(fmt.Sprint("io.ReadAll(response.Body) error : ", err.Error()))
				return
			}
			tempBody := make(map[string]string)
			tempBody["data"] = string(body)
			res = tempBody

		}
		rb, err := json.Marshal(res)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return &TemplateVars{}, err
		}
		err = json.Unmarshal(rb, &trResVars.Body)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return &TemplateVars{}, err
		}
		trResVars.OrgBody = trResVars.Body
		if route.TransformResponse != "" {
			logs.WithContext(ctx).Info(fmt.Sprint("inside route.TransformResponse"))
			fvars := &FuncTemplateVars{}
			fvars.Vars = trResVars
			output, err := processTemplate(ctx, route.RouteName, route.TransformResponse, fvars, "json", route.TokenSecretKey)
			logs.WithContext(ctx).Info(fmt.Sprint(string(output)))
			if err != nil {
				return &TemplateVars{}, err
			}
			response.Body = io.NopCloser(bytes.NewBuffer(output))
			response.Header.Set("Content-Length", strconv.Itoa(len(output)))
			response.ContentLength = int64(len(output))

			err = json.Unmarshal(output, &trResVars.Body)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return &TemplateVars{}, err
			}
		} else {
			response.Body = io.NopCloser(bytes.NewBuffer(rb))
			response.Header.Set("Content-Length", strconv.Itoa(len(rb)))
			response.ContentLength = int64(len(rb))
		}
	}
	if route.RemoveParams.ResponseHeaders != nil {
		for _, v := range route.RemoveParams.ResponseHeaders {
			response.Header.Del(v)
		}
	}
	return
}
