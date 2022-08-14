package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	utils "github.com/eru-tech/eru/eru-utils"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type KratosHydraAuth struct {
	Auth
	Kratos KratosConfig
	Hydra  HydraConfig
}

type KratosLoginPostBody struct {
	CsrfToken  string `json:"csrf_token"`
	Identifier string `json:"identifier"`
	Method     string `json:"method"`
	Password   string `json:"password"`
}

type KratosConfig struct {
	PublicHost   string
	PublicPort   string
	PublicScheme string
	AdminHost    string
	AdminPort    string
	AdminScheme  string
	LoginMethod  string
}

type KratosFlow struct {
	Id           string    `json:"id"`
	Flowtype     string    `json:"type"`
	ExpiresAt    time.Time `json:"expires_at"`
	IssuedAt     time.Time `json:"issued_at"`
	RequestUrl   string    `json:"request_url"`
	UI           KratosUI  `json:"ui"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Refresh      bool      `json:"refresh"`
	RequestedAal string    `json:"requested_aal"`
}

type KratosUI struct {
	Action   string            `json:"action"`
	Method   string            `json:"method"`
	Nodes    []KratosUINode    `json:"nodes"`
	Messages []KratosUIMessage `json:"messages"`
}
type KratosUIMessage struct {
	Id          int64       `json:"id"`
	MessageType string      `json:"type"`
	Text        string      `json:"text"`
	Context     interface{} `json:"context"`
}
type KratosUINode struct {
	NodeType   string                 `json:"type"`
	Group      string                 `json:"group"`
	Attributes KratosUINodeAttributes `json:"attributes"`
	Messages   []interface{}          `json:"messages"`
	Meta       interface{}            `json:"meta"`
}

type KratosUINodeAttributes struct {
	Name          string `json:"name"`
	AttributeType string `json:"type"`
	Value         string `json:"value"`
	Required      bool   `json:"required"`
	Disabled      bool   `json:"disabled"`
	NodeType      string `json:"node_type"`
}

type KratosAuthenticationMethods struct {
	Method      string    `json:"method"`
	Aal         string    `json:"aal"`
	CompletedAt time.Time `json:"completed_at"`
}

type KratosSession struct {
	SessionToken string            `json:"session_token"`
	Session      KratosSessionBody `json:"session"`
}
type KratosSessionBody struct {
	Id                          string                        `json:"id"`
	Active                      bool                          `json:"active"`
	ExpiresAt                   time.Time                     `json:"expires_at"`
	AuthenticatedAt             time.Time                     `json:"authenticated_at"`
	AuthenticatorAssuranceLevel string                        `json:"authenticator_assurance_level"`
	AuthenticationMethods       []KratosAuthenticationMethods `json:"authentication_methods"`
	IssuedAt                    time.Time                     `json:"issued_at"`
	Identity                    KratosIdentity                `json:"identity"`
}

type KratosIdentity struct {
	Id                  string                  `json:"id"`
	SchemaId            string                  `json:"schema_id"`
	SchemaUrl           string                  `json:"schema_url"`
	State               string                  `json:"state"`
	StateChangedAt      time.Time               `json:"state_changed_at"`
	Traits              interface{}             `json:"traits"`
	VerifiableAddresses []KratosIdentityAddress `json:"verifiable_addresses"`
	RecoveryAddresses   []KratosIdentityAddress `json:"recovery_addresses"`
	MetaDataPublic      interface{}             `json:"metadata_public"`
	CreatedAt           time.Time               `json:"created_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
}
type KratosIdentityAddress struct {
	Id        string    `json:"id"`
	Value     string    `json:"value"`
	Verified  bool      `json:"verified"`
	Via       string    `json:"via"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (kratosHydraAuth *KratosHydraAuth) MakeFromJson(rj *json.RawMessage) error {
	log.Println("inside AwsStorage MakeFromJson")
	err := json.Unmarshal(*rj, &kratosHydraAuth)
	if err != nil {
		log.Print("error json.Unmarshal(*rj, &awsStorage)")
		log.Print(err)
		return err
	}
	log.Println(kratosHydraAuth)
	return nil
}
func (kratosHydraAuth *KratosHydraAuth) PerformPreSaveTask() (err error) {
	log.Println("inside PerformPreSaveTask")
	for _, v := range kratosHydraAuth.Hydra.HydraClients {
		err = kratosHydraAuth.Hydra.SaveHydraClient(v)
		if err != nil {
			log.Print(err)
			return err
		}
	}
	return
}
func (kratosHydraAuth *KratosHydraAuth) PerformPreDeleteTask() (err error) {
	log.Println("inside PerformPreDeleteTask")
	for _, v := range kratosHydraAuth.Hydra.HydraClients {
		err = kratosHydraAuth.Hydra.RemoveHydraClient(v.ClientId)
		if err != nil {
			log.Print(err)
			return err
		}
	}
	return
}
func (kratosHydraAuth *KratosHydraAuth) ensureCookieFlowId(flowType string, r *http.Request) (cirf_token string, flow_id string, err error) {
	//ctx := context.Background()
	// fetch flowID from url query parameters
	flowId := r.URL.Query().Get("flow")
	// fetch cookie from headers
	cookie := r.Header.Get("cookie")
	if flowId == "" || cookie == "" {
		log.Println("inside flowId == \"\" || cookie == \"\" ")
		//newR := r.Clone(ctx)
		newR := http.Request{
			Method: "GET",
			Host:   kratosHydraAuth.Kratos.PublicHost,
			Header: http.Header{},
		}
		port := kratosHydraAuth.Kratos.PublicPort
		if port != "" {
			port = fmt.Sprint(":", port)
		}
		url := url.URL{
			Scheme: kratosHydraAuth.Kratos.PublicScheme,
			Host:   fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port),
			Path:   fmt.Sprint("/self-service/", flowType, "/api"),
		}
		newR.URL = &url
		//newR.URL.Host = fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port)
		//newR.URL.Path = fmt.Sprint("/self-service/", flowType, "/api")
		//newR.URL.Scheme = kratosHydraAuth.Kratos.PublicScheme
		//newR.Method = "GET"
		newR.ContentLength = int64(0)

		params := newR.URL.Query()
		params.Del("flow")
		newR.URL.RawQuery = params.Encode()

		//newR.Header.Del("cookie")
		newR.Header.Set("Accept", "application/json")
		newR.Header.Set("Content-Length", strconv.Itoa(0))
		//newR.RequestURI = ""
		//newR.Host = kratosHydraAuth.Kratos.PublicHost
		log.Println(newR)
		flowRes, flowErr := httpClient.Do(&newR)
		if flowErr != nil {
			log.Println(" httpClient.Do error ")
			log.Println(flowErr)
			err = flowErr
			return
		}
		loginFLowFromRes := json.NewDecoder(flowRes.Body)
		loginFLowFromRes.DisallowUnknownFields()
		log.Print(loginFLowFromRes)
		var loginFlow KratosFlow

		if err = loginFLowFromRes.Decode(&loginFlow); err != nil {
			log.Println(err)
			return
		}
		log.Println(loginFlow)
		log.Println(flowRes.Header)
		cirf_token = ""
		for _, node := range loginFlow.UI.Nodes {
			if node.Attributes.Name == "csrf_token" {
				cirf_token = node.Attributes.Value
				break
			}
		}
		return cirf_token, loginFlow.Id, nil
	}
	return
}
func (kratosHydraAuth *KratosHydraAuth) Logout(req *http.Request) (res interface{}, resStatusCode int, err error) {
	sessionToken := ""
	tokenObj := make(map[string]interface{})
	//todo - remove hardcoding of claims and change it to projectConfig.TokenSecret.HeaderKey
	tokenStr := req.Header.Get("Claims")
	log.Print("tokenStr = ", tokenStr)
	if tokenStr != "" {
		err = json.Unmarshal([]byte(tokenStr), &tokenObj)
		if err != nil {
			return nil, 400, err
		}
	}
	if identity, ok := tokenObj["identity"]; ok {
		if identityMap, iOk := identity.(map[string]interface{}); iOk {
			if authDetails, adOk := identityMap["auth_details"]; adOk {
				if authDetailsMap, admOk := authDetails.(map[string]interface{}); admOk {
					if st, stOk := authDetailsMap["session_token"]; stOk {
						sessionToken = st.(string)
					}
				}
			}
		}
	}
	log.Print(sessionToken)
	if sessionToken == "" {
		err = errors.New("session token not found")
		log.Print(err)
		return nil, 400, err
	}

	refreshTokenFromReq := json.NewDecoder(req.Body)
	refreshTokenFromReq.DisallowUnknownFields()
	refreshTokenObj := make(map[string]string)
	if rtErr := refreshTokenFromReq.Decode(&refreshTokenObj); rtErr != nil {
		log.Print(rtErr)
		err = rtErr
		resStatusCode = 400
		return
	}
	refreshToken := ""
	if rt, ok := refreshTokenObj["refresh_token"]; ok {
		refreshToken = rt
	}
	if refreshToken == "" {
		err = errors.New("refresh token not found")
		log.Print(err)
		resStatusCode = 400
		return
	}

	logoutPostBody := make(map[string]string)
	logoutPostBody["session_token"] = sessionToken

	port := kratosHydraAuth.Kratos.PublicPort
	if port != "" {
		port = fmt.Sprint(":", port)
	}
	//req.RequestURI = ""
	//req.Host = kratosHydraAuth.Kratos.PublicHost
	//req.URL.Host = fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port)
	//req.URL.Path = "/self-service//logout/api"
	//req.URL.Scheme = kratosHydraAuth.Kratos.PublicScheme
	//req.Method = "DELETE"

	log.Println(req.URL)

	res, _, _, resStatusCode, logoutErr := utils.CallHttp(http.MethodDelete, fmt.Sprint(kratosHydraAuth.Kratos.PublicScheme, "://", kratosHydraAuth.Kratos.PublicHost, port, "/self-service/logout/api"), req.Header, nil, nil, nil, logoutPostBody)
	if logoutErr != nil {
		log.Println(" httpClient.Do error ")
		log.Println(logoutErr)
		err = logoutErr
		return
	}
	resStatusCode, err = kratosHydraAuth.Hydra.revokeToken(refreshToken)
	if res == nil {
		res = make(map[string]interface{})
	}
	return res, resStatusCode, err
}
func (kratosHydraAuth *KratosHydraAuth) Login(req *http.Request) (res interface{}, cookies []*http.Cookie, err error) {
	//loginPostBodyMap := loginPostBody.(map[string]interface{})
	//username := loginPostBodyMap["identifier"]
	//password := loginPostBodyMap["password"]
	//log.Println(username,password)

	cirf_token, flow_id, err := kratosHydraAuth.ensureCookieFlowId("login", req)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("flow == ", flow_id)
	log.Println("cirf_token == ", cirf_token)
	log.Println(err)

	loginChallenge, loginChallengeCookies, err := kratosHydraAuth.ensureLoginChallenge(req)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("loginChallenge = ", loginChallenge)

	newR := http.Request{
		Method: "POST",
		Host:   kratosHydraAuth.Kratos.PublicHost,
		Header: http.Header{},
	}
	port := kratosHydraAuth.Kratos.PublicPort
	if port != "" {
		port = fmt.Sprint(":", port)
	}
	url := url.URL{
		Scheme: kratosHydraAuth.Kratos.PublicScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port),
		Path:   fmt.Sprint("/self-service/login"),
	}
	newR.URL = &url
	//newR.URL.Host = fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port)
	//newR.URL.Path = fmt.Sprint("/self-service/", flowType, "/api")
	//newR.URL.Scheme = kratosHydraAuth.Kratos.PublicScheme
	//newR.Method = "GET"
	newR.ContentLength = int64(0)

	params := newR.URL.Query()
	params.Del("flow")
	newR.URL.RawQuery = params.Encode()

	//newR.Header.Del("cookie")
	newR.Header.Set("Accept", "application/json")
	//newR.Header.Set("Content-Length", strconv.Itoa(0))

	loginPostBodyFromReq := json.NewDecoder(req.Body)
	loginPostBodyFromReq.DisallowUnknownFields()

	var loginPostBody LoginPostBody

	if err = loginPostBodyFromReq.Decode(&loginPostBody); err != nil {
		log.Println(err)
		return
	}
	log.Println(loginPostBody)

	kratosLoginPostBody := KratosLoginPostBody{}

	kratosLoginPostBody.Identifier = loginPostBody.Username
	kratosLoginPostBody.Password = loginPostBody.Password
	kratosLoginPostBody.Method = kratosHydraAuth.Kratos.LoginMethod
	kratosLoginPostBody.CsrfToken = cirf_token

	newParams := newR.URL.Query()
	newParams.Set("flow", flow_id)
	newR.URL.RawQuery = newParams.Encode()
	//r.Header.Set("cookie", flowRes.Header.Get("set-cookie"))
	log.Println("kratosLoginPostBody")
	log.Println(kratosLoginPostBody)
	rb, jmErr := json.Marshal(kratosLoginPostBody)
	if jmErr != nil {
		log.Println(jmErr)
		err = jmErr
		return
	}
	newR.Body = ioutil.NopCloser(bytes.NewBuffer(rb))
	newR.Header.Set("Content-Length", strconv.Itoa(len(rb)))
	newR.Header.Set("Content-Type", "application/json")
	newR.ContentLength = int64(len(rb))

	//newR.RequestURI = ""
	//newR.Host = kratosHydraAuth.Kratos.PublicHost
	log.Println(newR)
	utils.PrintRequestBody(&newR, "printing second request to kratos")

	//port := kratosHydraAuth.Kratos.PublicPort
	//if port != "" {
	//	port = fmt.Sprint(":", port)
	//}
	//req.RequestURI = ""
	//req.Host = kratosHydraAuth.Kratos.PublicHost
	//req.URL.Host = fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port)
	//req.URL.Path = "/self-service/login"
	//req.URL.Scheme = kratosHydraAuth.Kratos.PublicScheme
	//req.Method = "POST"

	//log.Println(req.URL)
	loginRes, loginErr := httpClient.Do(&newR)
	if loginErr != nil {
		log.Println(" httpClient.Do error ")
		log.Println(loginErr)
		err = loginErr
		return
	}

	utils.PrintResponseBody(loginRes, "printing response After httpClient.Do of self-service/login/flows")

	body, err := ioutil.ReadAll(loginRes.Body)
	if err != nil {
		log.Println(err)
		return
	}

	loginBodyFromRes := json.NewDecoder(bytes.NewReader(body))
	loginBodyFromRes.DisallowUnknownFields()
	var kratosSession KratosSession
	var kratosLoginFlow KratosFlow
	kratosLoginSucceed := true
	if err = loginBodyFromRes.Decode(&kratosSession); err != nil {
		kratosLoginSucceed = false
		log.Println(err)
		loginBodyFromRes = json.NewDecoder(bytes.NewReader(body))
		loginBodyFromRes.DisallowUnknownFields()

		if err = loginBodyFromRes.Decode(&kratosLoginFlow); err != nil {
			log.Println(err)
			return
		}
	}
	log.Println(kratosSession)
	log.Println(kratosLoginFlow)
	if kratosLoginSucceed {
		// accept Hydra login request
		consentChallenge, loginAcceptRequestCookies, loginAcceptErr := kratosHydraAuth.Hydra.acceptLoginRequest(kratosSession.Session.Identity.Id, loginChallenge, loginChallengeCookies)
		log.Println(consentChallenge)
		log.Println(loginAcceptRequestCookies)
		if loginAcceptErr != nil {
			log.Println(loginAcceptErr)
			err = loginAcceptErr
			return
		}
		identityHolder := make(map[string]interface{})
		identity := Identity{}
		identity.Id = kratosSession.Session.Identity.Id
		identity.CreatedAt = kratosSession.Session.Identity.CreatedAt
		identity.UpdatedAt = kratosSession.Session.Identity.UpdatedAt
		identity.OtherInfo = make(map[string]interface{})
		identity.OtherInfo["schema_id"] = kratosSession.Session.Identity.SchemaId
		identity.OtherInfo["state_changed_at"] = kratosSession.Session.Identity.StateChangedAt
		identity.Status = kratosSession.Session.Identity.State
		identity.AuthDetails = IdentityAuth{}
		identity.AuthDetails.SessionId = kratosSession.Session.Id
		identity.AuthDetails.SessionToken = kratosSession.SessionToken
		identity.AuthDetails.AuthenticatedAt = kratosSession.Session.AuthenticatedAt
		for _, v := range kratosSession.Session.AuthenticationMethods {
			identity.AuthDetails.AuthenticationMethods = append(identity.AuthDetails.AuthenticationMethods, v)
		}
		log.Println("asasasasas")
		log.Println(identity.AuthDetails.AuthenticationMethods)
		identity.AuthDetails.AuthenticatorAssuranceLevel = kratosSession.Session.AuthenticatorAssuranceLevel
		identity.AuthDetails.ExpiresAt = kratosSession.Session.ExpiresAt
		identity.AuthDetails.IssuedAt = kratosSession.Session.IssuedAt
		identity.AuthDetails.SessionStatus = kratosSession.Session.Active
		ok := false
		if identity.Attributes, ok = kratosSession.Session.Identity.Traits.(map[string]interface{}); ok {
			identity.Attributes["sub"] = kratosSession.Session.Identity.Id
			if pubMetadata, ok := kratosSession.Session.Identity.MetaDataPublic.(map[string]interface{}); ok {
				for k, v := range pubMetadata {
					if _, chkKey := identity.Attributes[k]; !chkKey { // check if key already exists then silemtly ignore the value from public metadata
						identity.Attributes[k] = v
					}
				}
			} else {
				log.Println("kratosSession.Session.Identity.MetaDataPublic is not a map[string]interface{}")
				err = errors.New("Error reading Identity MetaDataPublic")
				return
			}
		} else {
			log.Println("kratosSession.Session.Identity.Traits is not a map[string]interface{}")
			err = errors.New("Error reading Identity")
			return
		}
		identityHolder["identity"] = identity
		tokens, loginAcceptConsentCookies, cosentAcceptErr := kratosHydraAuth.Hydra.acceptConsentRequest(identityHolder, consentChallenge, loginAcceptRequestCookies)
		if cosentAcceptErr != nil {
			log.Println(cosentAcceptErr)
			err = cosentAcceptErr
			return
		}
		return tokens, loginAcceptConsentCookies, nil
	} else {
		err = errors.New(fmt.Sprint("Login Failed : ", kratosLoginFlow.UI.Messages))
		return
		//reject Hudra login request
	}
}

func (kratosConfig KratosConfig) getPublicUrl() (url string) {
	port := ""
	if kratosConfig.PublicPort != "" {
		port = fmt.Sprint(":", kratosConfig.PublicPort)
	}
	return fmt.Sprint(kratosConfig.PublicScheme, "://", kratosConfig.PublicHost, port)
}
func (kratosConfig KratosConfig) getAminUrl() (url string) {
	port := ""
	if kratosConfig.AdminPort != "" {
		port = fmt.Sprint(":", kratosConfig.AdminPort)
	}
	return fmt.Sprint(kratosConfig.AdminScheme, "://", kratosConfig.AdminHost, port)
}
