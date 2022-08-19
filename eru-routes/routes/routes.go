package routes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-crypto/jwt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	encodedForm   = "application/x-www-form-urlencoded"
	multiPartForm = "multipart/form-data"
)
const MatchTypePrefix = "PREFIX"
const MatchTypeExact = "EXACT"

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
	RouteName           string `eru:"required"`
	RouteCategoryName   string
	Url                 string `eru:"required"`
	MatchType           string `eru:"required"`
	RewriteUrl          string
	TargetHosts         []TargetHost `eru:"required"`
	AllowedHosts        []string
	AllowedMethods      []string
	RequiredHeaders     []Headers
	EnableCache         bool
	RequestHeaders      []Headers
	QueryParams         []Headers
	FormData            []Headers
	FileData            []FilePart
	ResponseHeaders     []Headers
	TransformRequest    string
	TransformResponse   string
	IsPublic            bool
	Authorizer          string
	AuthorizerException []string
	TokenSecret         TokenSecret `json:"-"`
	RemoveParams        RemoveParams
	OnError             string
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
	Token            interface{}
	FormDataKeyArray []string
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
		path = fmt.Sprint(route.RewriteUrl, strings.TrimPrefix(strings.Split(url, route.RouteName)[1], route.Url))
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
	log.Println(route)
	log.Println(url)
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
	log.Println(route)
	//TODO Random selection of target based on allocation
	if len(route.TargetHosts) > 0 {
		return route.TargetHosts[0], err
	}
	err = errors.New(fmt.Sprint("No Target Host defined for this route :", route.RouteName))
	return
}

func (route *Route) Execute(request *http.Request, url string) (response *http.Response, trResVars *TemplateVars, err error) {
	log.Println("inside route.Execute")
	log.Println("url = ", url)

	trReqVars, err := route.transformRequest(request, url)
	if err != nil {
		log.Println("error from transformRequest")
		log.Println(err)
		return
	}
	log.Println("Before httpClient.Do of route Execute")
	log.Print(trReqVars)
	log.Println(request.URL)
	log.Println(request.Method)
	log.Print("printing request header beofre route.Execute")
	log.Print(request.Header)

	log.Println(route.TargetHosts)
	//log.Println(request)
	request.Header.Set("accept-encoding", "identity")
	printRequestBody(request, "printing request Before httpClient.Do of route Execute")
	log.Print("request.Host = ", request.Host)
	if request.Host != "" {
		response, err = httpClient.Do(request)
		if err != nil {
			log.Println(" httpClient.Do error from route execute function")
			log.Println(err)
			return
		}
		log.Println(response.Header)
		log.Println(response.StatusCode)
		log.Print(response.ContentLength)
		printResponseBody(response, "printing response After httpClient.Do of route Execute before transformResponse")
	} else {
		response = &http.Response{Header: http.Header{}, StatusCode: http.StatusOK}
		rb, err := json.Marshal(make(map[string]interface{}))
		if err != nil {
			log.Println(err)
		} else {
			log.Print("making dummy response body")
			response.Body = ioutil.NopCloser(bytes.NewReader(rb))
		}
		log.Print("response.Body")
		log.Print(response.Body)
	}
	trResVars = &TemplateVars{}

	if route.TransformResponse != "" {
		trResVars, err = route.transformResponse(response, trReqVars)

		if err != nil {
			log.Println(err)
			return
		}
		printResponseBody(response, "printing response After httpClient.Do of route Execute after transformResponse")
	}
	return

}

func (route *Route) transformRequest(request *http.Request, url string) (vars *TemplateVars, err error) {
	log.Println("inside route.transformRequest")
	//printRequestBody(request,"body from route transformRequest")

	reqVarsLoaded := false

	vars = &TemplateVars{}
	vars.FormData = make(map[string]interface{})
	vars.Body = make(map[string]interface{})
	reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	if reqContentType == encodedForm || reqContentType == multiPartForm {
		vars.FormData["dummy"] = nil
		// addding the same so loadvars will get length > 0 and avoid processing body
		// this dummy record will get overwritten as part of return value from process multipart
	}

	if !reqVarsLoaded {
		err = loadRequestVars(vars, request)
		if err != nil {
			log.Println(err)
			return
		}
		reqVarsLoaded = true
	}
	scheme, host, port, path, method, err := route.GetTargetSchemeHostPortPath(url)
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
	request.Method = method

	for _, h := range route.RequestHeaders {
		if !h.IsTemplate {
			request.Header.Set(h.Key, h.Value)
		}
	}
	multiPart := false
	//reqContentType := strings.Split(request.Header.Get("Content-type"), ";")[0]
	log.Print("reqContentType from makeMultipart = ", reqContentType)
	if reqContentType == encodedForm || reqContentType == multiPartForm {
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

		for i, fd := range route.FormData {
			if fd.IsTemplate {
				log.Print("inside route.FormData")
				log.Print(fd.Key)
				output, err := processTemplate(fd.Key, fd.Value, mpvars, "string", route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl)
				if err != nil {
					log.Println(err)
					return &TemplateVars{}, err
				}
				log.Print("form data template processed")
				outputStr, err := strconv.Unquote(string(output))
				if err != nil {
					log.Println(err)
					return &TemplateVars{}, err
				}
				log.Print(outputStr)
				route.FormData[i].Value = outputStr
			}
		}
		vars.FormData, vars.FormDataKeyArray, err = processMultipart(request, route.RemoveParams.FormData, route.FormData)
		if err != nil {
			return
		}
	}
	err = processParams(request, route.RemoveParams.QueryParams, route.QueryParams)
	if err != nil {
		return
	}

	if !multiPart {
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
				return &TemplateVars{}, err
			}
			err = json.Unmarshal(output, &vars.Body)
			if err != nil {
				log.Println(err)
				return &TemplateVars{}, err
			}
			request.Body = ioutil.NopCloser(bytes.NewBuffer(output))
			request.Header.Set("Content-Length", strconv.Itoa(len(output)))
			request.ContentLength = int64(len(output))

		} else {
			log.Print("inside else")
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
			//log.Println("body from route transformRequest - else part")
			//log.Println(string(body))
			//log.Println(request.Header.Get("Content-Length"))
			request.Body = ioutil.NopCloser(bytes.NewReader(body))
			//request.Header.Set("Content-Length", strconv.Itoa(len(body)))
			//request.ContentLength = int64(len(body))
		}
	}

	err = processHeaderTemplates(request, route.RemoveParams.RequestHeaders, route.RequestHeaders, reqVarsLoaded, vars, route.TokenSecret.HeaderKey, route.TokenSecret.JwkUrl, nil, nil)
	if err != nil {
		log.Print("error from processHeaderTemplates")
		return
	}
	return
}

func (route *Route) transformResponse(response *http.Response, trReqVars *TemplateVars) (trResVars *TemplateVars, err error) {

	log.Println("inside transformResponse")
	//a, e := json.Marshal(trReqVars)
	//log.Print(string(a))
	//log.Print(e)
	trResVars = &TemplateVars{}
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

	trResVars.Vars = trReqVars.Vars
	var res interface{}
	log.Print(response.Body)
	tmplBodyFromRes := json.NewDecoder(response.Body)
	tmplBodyFromRes.DisallowUnknownFields()
	log.Print("tmplBodyFromRes = ", tmplBodyFromRes)
	if err = tmplBodyFromRes.Decode(&res); err != nil {
		log.Println("tmplBodyFromRes.Decode error")
		log.Println(err)
		return
	}
	log.Print(res)
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
	log.Print(trResVars)
	if route.TransformResponse != "" {

		fvars := &FuncTemplateVars{}
		fvars.Vars = trResVars
		log.Print(fvars.Vars.Body)
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
