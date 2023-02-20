package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	utils "github.com/eru-tech/eru/eru-utils"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// TODO to read this from kratos schema
var kratosTraits = [...]string{"email", "phone", "name", "role"}

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
	Messages   []KratosUIMessage      `json:"messages"`
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
	Devices                     []KratosDevice                `json:"devices"`
}
type KratosDevice struct {
	Id        string `json:"id"`
	IP        string `json:"ip_address"`
	Location  string `json:"location"`
	UserAgent string `json:"user_agent"`
}
type KratosIdentity struct {
	Id                  string                  `json:"id"`
	SchemaId            string                  `json:"schema_id"`
	SchemaUrl           string                  `json:"schema_url"`
	State               string                  `json:"state"`
	StateChangedAt      time.Time               `json:"state_changed_at"`
	Traits              map[string]interface{}  `json:"traits"`
	VerifiableAddresses []KratosIdentityAddress `json:"verifiable_addresses"`
	RecoveryAddresses   []KratosIdentityAddress `json:"recovery_addresses"`
	MetaDataPublic      map[string]interface{}  `json:"metadata_public"`
	MetaDataAdmin       map[string]interface{}  `json:"metadata_admin"`
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
		//log.Println(newR)
		flowRes, flowErr := httpClient.Do(&newR)
		if flowErr != nil {
			log.Println(" httpClient.Do error ")
			log.Println(flowErr)
			err = flowErr
			return
		}
		loginFLowFromRes := json.NewDecoder(flowRes.Body)
		loginFLowFromRes.DisallowUnknownFields()
		//log.Print(loginFLowFromRes)
		var loginFlow KratosFlow

		if err = loginFLowFromRes.Decode(&loginFlow); err != nil {
			log.Println(err)
			return
		}
		//log.Println(loginFlow)
		//log.Println(flowRes.Header)
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

func (kratosHydraAuth *KratosHydraAuth) getFlowId(flowType string) (flow_id string, err error) {

	port := kratosHydraAuth.Kratos.PublicPort
	if port != "" {
		port = fmt.Sprint(":", port)
	}
	url := url.URL{
		Scheme: kratosHydraAuth.Kratos.PublicScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port),
		Path:   fmt.Sprint("/self-service/", flowType, "/api"),
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Content-Length", strconv.Itoa(0))

	flowRes, _, _, _, flowErr := utils.CallHttp(http.MethodGet, url.String(), headers, nil, nil, nil, nil)
	if flowErr != nil {
		log.Println(" httpClient.Do error ")
		log.Println(flowErr)
		err = flowErr
		return
	}
	loginFLowFromRes, loginFLowFromResErr := json.Marshal(flowRes)
	if loginFLowFromResErr != nil {
		err = loginFLowFromResErr
		log.Print(err)
		return
	}
	//	log.Print(loginFLowFromRes)
	var loginFlow KratosFlow
	if err = json.Unmarshal(loginFLowFromRes, &loginFlow); err != nil {
		log.Println(err)
		return
	}
	//	log.Println(loginFlow)
	return loginFlow.Id, nil
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
func (kratosHydraAuth *KratosHydraAuth) CompleteRecovery(recoveryPassword RecoveryPassword) (msg string, err error) {
	recoveryCodeArray := strings.Split(recoveryPassword.Code, "__")
	if len(recoveryCodeArray) <= 0 {
		err = errors.New("incorrect recovery code")
		log.Println(err)
		return
	}

	recoveryFlow := recoveryCodeArray[0]
	recoveryCode := recoveryCodeArray[1]

	log.Println("recoveryFlow = ", recoveryFlow)
	log.Println("recoveryCode = ", recoveryCode)

	port := kratosHydraAuth.Kratos.PublicPort
	if port != "" {
		port = fmt.Sprint(":", port)
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Content-Length", strconv.Itoa(0))

	iUrl := url.URL{
		Scheme: kratosHydraAuth.Kratos.PublicScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port),
		Path:   fmt.Sprint("/self-service/recovery"),
	}
	iParams := make(map[string]string)
	iParams["flow"] = recoveryFlow

	postBody := make(map[string]string)
	postBody["code"] = recoveryCode
	postBody["method"] = "code"
	flowId := ""
	resBytes := []byte("")
	res, resHeaders, resCookies, resStatusCode, resErr := utils.CallHttp(http.MethodPost, iUrl.String(), headers, nil, nil, iParams, postBody)
	log.Println(resHeaders)
	log.Println("resCookies printed after self-service/recovery call")
	log.Println(resCookies)
	if resErr != nil {
		err = resErr
		log.Print("resErr printed below")
		log.Print(err)
		if resStatusCode != 422 {
			err = errors.New("incorrect response from source. please try again")
			return
		}
		resBytes = []byte(resErr.Error())
		err = nil
	} else {
		resBytesTmp, resBytesErr := json.Marshal(res)
		if resBytesErr != nil {
			err = resBytesErr
			log.Print("resBytesErr printed below")
			log.Print(err)
			return
		}
		resBytes = resBytesTmp
	}
	log.Println("resBytes printded below")
	log.Println(string(resBytes))
	resMap := make(map[string]interface{})
	resBody := json.NewDecoder(bytes.NewReader(resBytes))
	resBody.DisallowUnknownFields()

	if resBodyErr := resBody.Decode(&resMap); resBodyErr != nil {
		err = resBodyErr
		log.Print("resBodyErr printed below")
		log.Print(err)
		return
	}

	if redirectLink, rOk := resMap["redirect_browser_to"]; rOk {
		redirectLinkSplit := strings.Split(redirectLink.(string), "flow=")
		if len(redirectLinkSplit) <= 0 {
			err = errors.New(fmt.Sprint("failed to fetch flow id from redirect link :", redirectLink))
			log.Println(err)
			return
		}
		flowId = redirectLinkSplit[1]
	} else {
		err = errors.New("resMap[\"error\"][\"redirect_browser_to\"] failed")
		log.Println(err)
		return
	}

	fUrl := url.URL{
		Scheme: kratosHydraAuth.Kratos.PublicScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port),
		Path:   fmt.Sprint("/self-service/settings/api"),
	}

	log.Println("flowId = ", flowId)

	fParams := make(map[string]string)
	fParams["flow"] = flowId

	pRes, pResHeaders, pResCookies, pResStatusCode, pResErr := utils.CallHttp(http.MethodGet, fUrl.String(), headers, nil, resCookies, fParams, nil)
	log.Println(pRes)
	log.Println(pResHeaders)
	log.Println(pResCookies)
	log.Println(pResStatusCode)
	log.Println(pResErr)
	pResCookies = append(pResCookies, resCookies...)
	csrf_token := ""
	kratosUi := KratosUI{}
	if pResMap, pResMapOk := pRes.(map[string]interface{}); pResMapOk {
		if pResMapUi, pResMapUiOk := pResMap["ui"]; pResMapUiOk {
			log.Println(pResMapUi)
			pResMapUiBytes, pResMapUiBytesErr := json.Marshal(pResMapUi)
			if pResMapUiBytesErr != nil {
				err = errors.New("json.Marshal(pResMapUi) failed")
				log.Println(err)
				return
			}

			kratosUiErr := json.Unmarshal(pResMapUiBytes, &kratosUi)
			if kratosUiErr != nil {
				err = errors.New("json.Unmarshal(pResMapUiBytes,&kratosUi) failed")
				log.Println(err)
				return
			}

			nodeFound := false
			for _, v := range kratosUi.Nodes {
				if v.Attributes.Name == "csrf_token" {
					nodeFound = true
					csrf_token = v.Attributes.Value
					break
				}
			}
			if !nodeFound {
				err = errors.New("csrf_token not found")
				log.Println(err)
				return
			}
		} else {
			err = errors.New("pResMap[\"ui\"]  failed")
			log.Println(err)
			return
		}
	} else {
		err = errors.New("pRes.(map[string]interface{}) failed")
		log.Println(err)
		return
	}
	log.Println("csrf_token = ", csrf_token)

	sfUrl := url.URL{
		Scheme: kratosHydraAuth.Kratos.PublicScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port),
		Path:   fmt.Sprint("/self-service/settings"),
	}

	sfPostBody := make(map[string]string)
	sfPostBody["method"] = "password"
	sfPostBody["password"] = recoveryPassword.Password
	sfPostBody["csrf_token"] = csrf_token

	sfRes, _, _, sfResStatusCode, sfResErr := utils.CallHttp(http.MethodPost, sfUrl.String(), headers, nil, pResCookies, fParams, sfPostBody)
	log.Println(sfRes)
	log.Println(sfResStatusCode)
	log.Println(sfResErr)
	log.Println("-----------------------------------------")
	kratosMsgUi := KratosUI{}

	sfResBytes := []byte("")
	if sfResErr != nil {
		err = sfResErr
		log.Print("sfResErr printed below")
		log.Print(err)
		sfResBytes = []byte(sfResErr.Error())
		err = nil
	} else {
		sfResBytesTmp, sfResBytesErr := json.Marshal(sfRes)
		if sfResBytesErr != nil {
			err = sfResBytesErr
			log.Print("sfResBytesErr printed below")
			log.Print(err)
			return
		}
		sfResBytes = sfResBytesTmp
	}
	log.Println("sfResBytes printded below")
	log.Println(string(sfResBytes))

	sfResMap := make(map[string]interface{})
	sfResJson := json.NewDecoder(bytes.NewReader(sfResBytes))
	sfResJson.DisallowUnknownFields()

	if sfResJsonErr := sfResJson.Decode(&sfResMap); sfResJsonErr != nil {
		err = sfResJsonErr
		log.Print("sfResJsonErr printed below")
		log.Print(err)
		return
	}

	//if sfResMap, sfResMapOk := sfRes.(map[string]interface{}); sfResMapOk {
	if sfResMapUi, sfResMapUiOk := sfResMap["ui"]; sfResMapUiOk {
		log.Println(sfResMapUi)
		sfResMapUiBytes, sfResMapUiBytesErr := json.Marshal(sfResMapUi)
		if sfResMapUiBytesErr != nil {
			err = errors.New("jjson.Marshal(sfResMapUi) failed")
			log.Println(err)
			return
		}
		kratosMsgErr := json.Unmarshal(sfResMapUiBytes, &kratosMsgUi)
		if kratosMsgErr != nil {
			err = errors.New("json.Unmarshal(sfResMapUiBytes, &kratosMsgUi) failed")
			log.Println(err)
			return
		}
	} else {
		err = errors.New("sfResMap[\"ui\"] failed")
		log.Println(err)
		return
	}
	//}
	//else {
	//	err = errors.New("sfRes.(map[string]interface{}) failed")
	//	log.Println(err)
	//	return
	//}
	if sfResErr == nil {
		for _, m := range kratosMsgUi.Messages {
			if msg != "" {
				msg = fmt.Sprint(msg, " , ")
			}
			msg = fmt.Sprint(msg, m.Text)
		}
	} else {
		for _, n := range kratosMsgUi.Nodes {
			for _, m := range n.Messages {
				if msg != "" {
					msg = fmt.Sprint(msg, " , ")
				}
				msg = fmt.Sprint(msg, m.Text)
			}
		}
		err = errors.New(msg)
		msg = ""
	}

	return
}

func (kratosHydraAuth *KratosHydraAuth) GenerateRecoveryCode(recoveryIdentifier RecoveryPostBody) (recoveryCode map[string]string, err error) {
	userid := ""
	port := kratosHydraAuth.Kratos.AdminPort
	if port != "" {
		port = fmt.Sprint(":", port)
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Content-Length", strconv.Itoa(0))

	iUrl := url.URL{
		Scheme: kratosHydraAuth.Kratos.AdminScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.AdminHost, port),
		Path:   fmt.Sprint("/admin/identities"),
	}

	iParams := make(map[string]string)
	iParams["credentials_identifier"] = recoveryIdentifier.Username

	iRes, _, _, _, iResErr := utils.CallHttp(http.MethodGet, iUrl.String(), headers, nil, nil, iParams, nil)

	if iResErr != nil {
		err = iResErr
		log.Print("iResErr printed below")
		log.Print(err)
		return
	}
	iResBytes, iResBytesErr := json.Marshal(iRes)
	if iResBytesErr != nil {
		err = iResBytesErr
		log.Print("iResBytesErr printed below")
		log.Print(err)
		return
	}
	iResJson := json.NewDecoder(bytes.NewReader(iResBytes))
	iResJson.DisallowUnknownFields()
	var kratosIdentities []KratosIdentity
	if iResDecodeErr := iResJson.Decode(&kratosIdentities); iResDecodeErr == nil {
		userFound := false

		for _, v := range kratosIdentities {
			if v.Traits["email"] == recoveryIdentifier.Username {
				userFound = true
				userid = v.Id
				break
			}
		}
		if !userFound {
			err = errors.New(fmt.Sprint("user not found with email ", recoveryIdentifier.Username))
			log.Println(err)
			return
		}
	} else {
		err = iResDecodeErr
		log.Print("iResJson.Decode(&kratosIdentities) failed")
		log.Print(err)
		return
	}
	log.Println("userid = ", userid)
	recoveryPostBody := make(map[string]string)
	recoveryPostBody["identity_id"] = userid

	url := url.URL{
		Scheme: kratosHydraAuth.Kratos.AdminScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.AdminHost, port),
		Path:   fmt.Sprint("/admin/recovery/code"),
	}

	rcRes, _, _, _, rcResErr := utils.CallHttp(http.MethodPost, url.String(), headers, nil, nil, nil, recoveryPostBody)
	log.Println(rcRes)
	log.Println(rcResErr)

	if rcResErr != nil {
		err = rcResErr
		log.Print("rcResErr printed below")
		log.Print(err)
		return
	}
	rcResBytes, rcResBytesErr := json.Marshal(rcRes)
	if rcResBytesErr != nil {
		err = rcResBytesErr
		log.Print("rcResBytesErr printed below")
		log.Print(err)
		return
	}
	rcResMap := make(map[string]interface{})
	rcResUmErr := json.Unmarshal(rcResBytes, &rcResMap)
	if rcResUmErr != nil {
		err = rcResUmErr
		log.Print("rcResUmErr printed below")
		log.Print(err)
		return
	}
	log.Println(rcResMap["recovery_code"])
	log.Println(rcResMap["recovery_link"])
	log.Println(strings.Split(rcResMap["recovery_link"].(string), "flow=")[1])

	recovery_link := strings.Split(rcResMap["recovery_link"].(string), "flow=")
	if len(recovery_link) <= 0 {
		err = errors.New("incorrect recovery link received")
		log.Print(err)
		return
	}

	if recoveryCode == nil {
		recoveryCode = make(map[string]string)
	}
	recoveryCode["code"] = fmt.Sprint(recovery_link[1], "__", rcResMap["recovery_code"].(string))
	return
}
func (kratosHydraAuth *KratosHydraAuth) Login(loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {

	flowId, flowErr := kratosHydraAuth.getFlowId("login")
	if flowErr != nil {
		log.Print(flowErr)
		return Identity{}, LoginSuccess{}, flowErr
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
	params := make(map[string]string)
	params["flow"] = flowId

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Content-Length", strconv.Itoa(0))

	kratosLoginPostBody := KratosLoginPostBody{}

	kratosLoginPostBody.Identifier = loginPostBody.Username
	kratosLoginPostBody.Password = loginPostBody.Password
	kratosLoginPostBody.Method = kratosHydraAuth.Kratos.LoginMethod

	loginRes, _, _, _, loginErr := utils.CallHttp(http.MethodPost, url.String(), headers, nil, nil, params, kratosLoginPostBody)
	loginResBytes := []byte("")
	if loginErr != nil {
		err = loginErr
		log.Print("loginErr printed below")
		log.Print(err)
		loginResBytes = []byte(loginErr.Error())
		//return
	} else {
		loginResJson, loginResJsonErr := json.Marshal(loginRes)
		if loginResJsonErr != nil {
			log.Print(loginResJsonErr)
			return
		}
		loginResBytes = loginResJson
	}
	log.Print("loginResBytes printed below")
	log.Print(string(loginResBytes))

	loginBodyFromRes := json.NewDecoder(bytes.NewReader(loginResBytes))
	loginBodyFromRes.DisallowUnknownFields()
	//	log.Print("loginBodyFromRes printed below")
	//log.Print(loginBodyFromRes)
	var kratosSession KratosSession
	var kratosLoginFlow KratosFlow
	kratosLoginSucceed := true
	if loginBodyFromResDecodeErr := loginBodyFromRes.Decode(&kratosSession); loginBodyFromResDecodeErr != nil {
		kratosLoginSucceed = false
		log.Println(loginBodyFromResDecodeErr)
		loginBodyFromRes = json.NewDecoder(bytes.NewReader(loginResBytes))
		loginBodyFromRes.DisallowUnknownFields()
		if err = loginBodyFromRes.Decode(&kratosLoginFlow); err != nil {
			log.Println(err)
			return
		}
	}
	//log.Print("kratosLoginSucceed = ", kratosLoginSucceed)
	log.Println(kratosSession)
	log.Println(kratosLoginFlow)

	if kratosLoginSucceed {
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
		identity.AuthDetails.AuthenticatorAssuranceLevel = kratosSession.Session.AuthenticatorAssuranceLevel
		identity.AuthDetails.ExpiresAt = kratosSession.Session.ExpiresAt
		identity.AuthDetails.IssuedAt = kratosSession.Session.IssuedAt
		identity.AuthDetails.SessionStatus = kratosSession.Session.Active
		//ok := false
		//if identity.Attributes, ok = kratosSession.Session.Identity.Traits.(map[string]interface{}); ok {
		if identity.Attributes == nil {
			identity.Attributes = make(map[string]interface{})
		}
		identity.Attributes["sub"] = kratosSession.Session.Identity.Id
		for k, v := range kratosSession.Session.Identity.Traits {
			if _, chkKey := identity.Attributes[k]; !chkKey { // check if key already exists then silemtly ignore the value from public metadata
				identity.Attributes[k] = v
			}
		}
		//if pubMetadata, ok := kratosSession.Session.Identity.MetaDataPublic.(map[string]interface{}); ok {
		for k, v := range kratosSession.Session.Identity.MetaDataPublic {
			if _, chkKey := identity.Attributes[k]; !chkKey { // check if key already exists then silemtly ignore the value from public metadata
				identity.Attributes[k] = v
			}
		}

		if withTokens {
			loginChallenge, loginChallengeCookies, loginChallengeErr := kratosHydraAuth.getLoginChallenge()
			if loginChallengeErr != nil {
				err = loginChallengeErr
				log.Println(err)
				return
			}
			//log.Println("loginChallenge = ", loginChallenge)

			consentChallenge, loginAcceptRequestCookies, loginAcceptErr := kratosHydraAuth.Hydra.acceptLoginRequest(kratosSession.Session.Identity.Id, loginChallenge, loginChallengeCookies)
			//log.Println("consentChallenge = " , consentChallenge)
			//	log.Println("loginAcceptRequestCookies = " , loginAcceptRequestCookies)
			if loginAcceptErr != nil {
				log.Println(loginAcceptErr)
				err = loginAcceptErr
				return
			}
			identityHolder := make(map[string]interface{})
			identityHolder["identity"] = identity
			tokens, cosentAcceptErr := kratosHydraAuth.Hydra.acceptConsentRequest(identityHolder, consentChallenge, loginAcceptRequestCookies)
			if cosentAcceptErr != nil {
				log.Println(cosentAcceptErr)
				err = cosentAcceptErr
				return
			}
			return identity, tokens, nil
		}
		return identity, LoginSuccess{}, nil
	} else {
		err = errors.New(fmt.Sprint("Login Failed : ", kratosLoginFlow.UI.Messages))
		return
	}
	return
}

/*
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

	loginChallenge, loginChallengeCookies, err := kratosHydraAuth.getLoginChallenge()
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
		identity.AuthDetails.AuthenticatorAssuranceLevel = kratosSession.Session.AuthenticatorAssuranceLevel
		identity.AuthDetails.ExpiresAt = kratosSession.Session.ExpiresAt
		identity.AuthDetails.IssuedAt = kratosSession.Session.IssuedAt
		identity.AuthDetails.SessionStatus = kratosSession.Session.Active
		//ok := false
		//if identity.Attributes, ok = kratosSession.Session.Identity.Traits.(map[string]interface{}); ok {
		if identity.Attributes == nil {
			identity.Attributes = make(map[string]interface{})
		}
		identity.Attributes["sub"] = kratosSession.Session.Identity.Id
		for k, v := range kratosSession.Session.Identity.Traits {
			if _, chkKey := identity.Attributes[k]; !chkKey { // check if key already exists then silemtly ignore the value from public metadata
				identity.Attributes[k] = v
			}
		}
		//if pubMetadata, ok := kratosSession.Session.Identity.MetaDataPublic.(map[string]interface{}); ok {
		for k, v := range kratosSession.Session.Identity.MetaDataPublic {
			if _, chkKey := identity.Attributes[k]; !chkKey { // check if key already exists then silemtly ignore the value from public metadata
				identity.Attributes[k] = v
			}
		}
		//} else {
		//	log.Println("kratosSession.Session.Identity.MetaDataPublic is not a map[string]interface{}")
		//	err = errors.New("Error reading Identity MetaDataPublic")
		//	return
		//}
		//} else {
		//	log.Println("kratosSession.Session.Identity.Traits is not a map[string]interface{}")
		////	err = errors.New("Error reading Identity")
		//	return
		//}
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
} */

func (kratosConfig KratosConfig) getPublicUrl() (url string) {
	port := ""
	if kratosConfig.PublicPort != "" {
		port = fmt.Sprint(":", kratosConfig.PublicPort)
	}
	return fmt.Sprint(kratosConfig.PublicScheme, "://", kratosConfig.PublicHost, port)
}
func (kratosConfig KratosConfig) getAdminUrl() (url string) {
	port := ""
	if kratosConfig.AdminPort != "" {
		port = fmt.Sprint(":", kratosConfig.AdminPort)
	}
	return fmt.Sprint(kratosConfig.AdminScheme, "://", kratosConfig.AdminHost, port)
}
func (kratosHydraAuth *KratosHydraAuth) GetUser(userId string) (identity Identity, err error) {
	kIdentity, err := kratosHydraAuth.getKratosUser(userId)
	if err != nil {
		return Identity{}, err
	}
	return convertToIdentity(kIdentity)
}

func (kratosHydraAuth *KratosHydraAuth) getKratosUser(userId string) (identity KratosIdentity, err error) {
	dummyMap := make(map[string]string)
	headers := http.Header{}
	res, _, _, _, err := utils.CallHttp("GET", fmt.Sprint(kratosHydraAuth.Kratos.getAdminUrl(), "/admin/identities/", userId), headers, dummyMap, nil, dummyMap, dummyMap)
	if err != nil {
		log.Print("error in http.Get GetUserInfo")
		log.Print(err)
		return KratosIdentity{}, err
	}
	log.Println(res)
	/*
		if sc >= 400{
			log.Print("error in http.Get GetUserInfo")
			resBytes, bytesErr := json.Marshal(res)
			if bytesErr != nil {
				log.Print("error in http.Get GetUserInfo")
				log.Print(bytesErr)
				return Identity{}, bytesErr
			}
			err = errors.New(strings.Replace(string(resBytes),"\"","",-1))
			return Identity{}, err
		}
	*/
	if userInfo, ok := res.(map[string]interface{}); !ok {
		err = errors.New("User response not in expected format")
		return KratosIdentity{}, err
	} else {
		log.Println(userInfo)
		kratosIdentityObjJson, iErr := json.Marshal(userInfo)
		if iErr != nil {
			log.Println(iErr)
			err = errors.New("Json marshal error for identityObj")
			return KratosIdentity{}, err
		}
		iiErr := json.Unmarshal(kratosIdentityObjJson, &identity)
		if iiErr != nil {
			log.Println(iiErr)
			err = errors.New("Json unmarshal error for identityObj")
			return KratosIdentity{}, err
		}
		return
	}
}

func (kratosHydraAuth *KratosHydraAuth) UpdateUser(identityToUpdate Identity) (err error) {
	userId := identityToUpdate.Attributes["sub"].(string)
	kratosIdentity, err := kratosHydraAuth.getKratosUser(userId)
	err = convertFromIdentity(identityToUpdate, &kratosIdentity)
	if err != nil {
		log.Print(err)
		return err
	}
	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp("PUT", fmt.Sprint(kratosHydraAuth.Kratos.getAdminUrl(), "/admin/identities/", userId), headers, dummyMap, nil, dummyMap, kratosIdentity)
	if err != nil {
		log.Print("error in http.Get GetUserInfo")
		log.Print(err)
		return err
	}
	log.Println(res)
	return
}

func (kratosHydraAuth *KratosHydraAuth) ChangePassword(req *http.Request, changePassword ChangePassword) (err error) {
	sessionToken := ""
	identifier := ""
	tokenObj := make(map[string]interface{})
	//todo - remove hardcoding of claims and change it to projectConfig.TokenSecret.HeaderKey
	tokenStr := req.Header.Get("Claims")
	log.Print("tokenStr = ", tokenStr)
	if tokenStr != "" {
		err = json.Unmarshal([]byte(tokenStr), &tokenObj)
		if err != nil {
			return err
		}
	}
	if identity, ok := tokenObj["identity"]; ok {
		log.Print("inside tokenObj[\"identity\"]")
		log.Print(identity)
		i, e := json.Marshal(identity)
		identityMap := Identity{}
		ee := json.Unmarshal(i, &identityMap)
		log.Print(e)
		log.Print(ee)
		log.Print(identityMap)
		sessionToken = identityMap.AuthDetails.SessionToken
		identifier = identityMap.Attributes["email"].(string)
		//if identityMap, iOk := json.NewDecoder(json.Marshal(identity))  identity.(Identity); iOk {
		//	log.Print("inside identity.(Identity)")
		//	sessionToken = identityMap.AuthDetails.SessionToken
	}
	log.Print(sessionToken)
	if sessionToken == "" {
		err = errors.New("session token not found")
		log.Print(err)
		return err
	}
	var loginPostBody LoginPostBody
	loginPostBody.Username = identifier
	loginPostBody.Password = changePassword.OldPassword
	_, _, loginErr := kratosHydraAuth.Login(loginPostBody, false)
	if loginErr != nil {
		err = errors.New("Incorrect Old Password")
		return
	}
	req.Header.Set("X-Session-Token", sessionToken)
	port := kratosHydraAuth.Kratos.PublicPort
	if port != "" {
		port = fmt.Sprint(":", port)
	}

	res, _, _, resStatusCode, resErr := utils.CallHttp(http.MethodGet, fmt.Sprint(kratosHydraAuth.Kratos.PublicScheme, "://", kratosHydraAuth.Kratos.PublicHost, port, "/self-service/settings/api"), req.Header, nil, nil, nil, nil)
	if resErr != nil {
		log.Println(" httpClient.Do error ")
		log.Print(err)
		err = resErr
		return
	}
	log.Print(res)
	log.Print(resStatusCode)

	flowParams := make(map[string]string)
	if resMap, ok := res.(map[string]interface{}); ok {
		if flowId, fOk := resMap["id"]; fOk {
			flowParams["flow"] = flowId.(string)
		} else {
			err = errors.New("flow id not found")
			log.Println(err)
			return
		}
	} else {
		err = errors.New("incorrect getflow response")
		log.Println(err)
		return
	}

	changePasswordBody := make(map[string]string)
	changePasswordBody["method"] = "password"
	changePasswordBody["password"] = changePassword.NewPassword

	cpRes, _, _, cpResStatusCode, cpResErr := utils.CallHttp(http.MethodPost, fmt.Sprint(kratosHydraAuth.Kratos.PublicScheme, "://", kratosHydraAuth.Kratos.PublicHost, port, "/self-service/settings"), req.Header, nil, nil, flowParams, changePasswordBody)
	if cpResErr != nil {
		log.Println(" httpClient.Do error ")
		err = cpResErr
		log.Println(err)
		errMap := make(map[string]interface{})

		errBytesUmErr := json.Unmarshal([]byte(err.Error()), &errMap)
		if errBytesUmErr == nil {
			if actualError, aeOk := errMap["error"]; aeOk {
				if actualErrorMap, aemOk := actualError.(map[string]interface{}); aemOk {
					errFinalMsg := ""
					if errMsg, emOk := actualErrorMap["message"]; emOk {
						errFinalMsg = fmt.Sprint(errMsg.(string), ".")
					}
					if errMsg, emOk := actualErrorMap["reason"]; emOk {
						errFinalMsg = fmt.Sprint(errFinalMsg, " ", errMsg.(string), ".")
					}
					err = errors.New(errFinalMsg)
				} else {
					log.Print("actualError.(map[string]interface{}) failed")
				}
			} else if uiError, uiOk := errMap["ui"]; uiOk {
				if uiErrorMap, uiMapOk := uiError.(map[string]interface{}); uiMapOk {
					if uiErrorNodes, uiNodesOk := uiErrorMap["nodes"]; uiNodesOk {
						if uiErrorNodesArray, uiNodeArrayOk := uiErrorNodes.([]interface{}); uiNodeArrayOk {
							for _, node := range uiErrorNodesArray {
								if uiNode, nodeOk := node.(map[string]interface{}); nodeOk {
									if uiAttrs, attrOk := uiNode["attributes"]; attrOk {
										if uiAttrsMap, attrMapOk := uiAttrs.(map[string]interface{}); attrMapOk {
											if uiName, nameOk := uiAttrsMap["name"]; nameOk {
												log.Print(uiName)
												if uiName == "password" {
													if uiMsg, msgOk := uiNode["messages"]; msgOk {
														if uiMsgArray, msgArrayOk := uiMsg.([]interface{}); msgArrayOk {
															if uiMsgMap, msgMapOk := uiMsgArray[0].(map[string]interface{}); msgMapOk {
																if uiReason, reasonOk := uiMsgMap["text"]; reasonOk {
																	err = errors.New(uiReason.(string))
																	return
																}
															}
														}
													}
													break
												}
											}
										}
									}
								}
							}
						} else {
							log.Print(reflect.TypeOf(uiErrorNodes))
							log.Print("uiErrorNodes.([]map[string]interface{}) failed")
						}
					} else {
						log.Print("uiErrorMap[\"nodes\"] not found")
					}
				} else {
					log.Print("uiError.(map[string]interface{}) failed")
				}

			} else {
				log.Print("ui or error attribute not found in errMap")
			}
		} else {
			log.Print(errBytesUmErr)
			log.Print("json.Unmarshal(errBytes,&errMap) failed")
		}
		err = errors.New(strings.Replace(err.Error(), "\"", "", -1))
		return
	}
	log.Print(cpRes)
	log.Print(cpResStatusCode)

	return err
}

func convertToIdentity(kratosIdentity KratosIdentity) (identity Identity, err error) {
	identity = Identity{}
	identity.Id = kratosIdentity.Id
	identity.CreatedAt = kratosIdentity.CreatedAt
	identity.UpdatedAt = kratosIdentity.UpdatedAt
	identity.OtherInfo = make(map[string]interface{})
	identity.OtherInfo["schema_id"] = kratosIdentity.SchemaId
	identity.OtherInfo["state_changed_at"] = kratosIdentity.StateChangedAt
	identity.Status = kratosIdentity.State
	identity.AuthDetails = IdentityAuth{}

	//ok := false
	//if identity.Attributes, ok = kratosIdentity.Traits.(map[string]interface{}); ok {
	if identity.Attributes == nil {
		identity.Attributes = make(map[string]interface{})
	}
	identity.Attributes["sub"] = kratosIdentity.Id
	//if pubMetadata, ok := kratosIdentity.MetaDataPublic.(map[string]interface{}); ok {
	for k, v := range kratosIdentity.MetaDataPublic {
		//if _, chkKey := identity.Attributes[k]; !chkKey { // check if key already exists then silemtly ignore the value from public metadata
		identity.Attributes[k] = v
		//}
	}
	//} else {
	//	log.Println("kratosIdentity.MetaDataPublic is not a map[string]interface{}")
	//	err = errors.New("Error reading Identity MetaDataPublic")
	//	return
	//}
	//} else {
	//	log.Println("kratosIdentity.Traits is not a map[string]interface{}")
	//	err = errors.New("Error reading Identity")
	//	return
	//}
	for k, v := range kratosIdentity.Traits {
		//if _, chkKey := identity.Attributes[k]; !chkKey { // check if key already exists then silemtly ignore the value from public metadata
		identity.Attributes[k] = v
		//}
	}
	return
}

func convertFromIdentity(identity Identity, kratosIdentity *KratosIdentity) (err error) {
	log.Print("original kratosIdentity printed below ")
	log.Print(kratosIdentity)

	log.Print("identity printed below")
	log.Print(identity)
	for k, v := range identity.Attributes {
		if k == "sub" {
			kratosIdentity.Id = v.(string)
		} else {
			traitFound := false
			for _, attr := range kratosTraits {
				if attr == k {
					traitFound = true
					kratosIdentity.Traits[k] = v
					break
				}
			}
			if !traitFound {
				kratosIdentity.MetaDataPublic[k] = v
			}
		}
	}
	log.Print("kratosIdentity printed below")
	log.Print(kratosIdentity)
	return
}
