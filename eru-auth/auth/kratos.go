package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
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
	Kratos KratosConfig `json:"kratos"`
	Hydra  HydraConfig  `json:"hydra"`
}

type KratosLoginPostBody struct {
	CsrfToken  string `json:"csrf_token"`
	Identifier string `json:"identifier"`
	Method     string `json:"method"`
	Password   string `json:"password"`
}

type KratosConfig struct {
	PublicHost   string `json:"public_host"`
	PublicPort   string `json:"public_port"`
	PublicScheme string `json:"public_scheme"`
	AdminHost    string `json:"admin_host"`
	AdminPort    string `json:"admin_port"`
	AdminScheme  string `json:"admin_scheme"`
	LoginMethod  string `json:"login_method"`
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
	OrgId               string                  `json:"organization_id"`
	Credentials         interface{}             `json:"credentials"`
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

func (kratosHydraAuth *KratosHydraAuth) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &kratosHydraAuth)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
func (kratosHydraAuth *KratosHydraAuth) PerformPreSaveTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreSaveTask - Start")
	for _, v := range kratosHydraAuth.Hydra.HydraClients {
		err = kratosHydraAuth.Hydra.SaveHydraClient(ctx, v)
		if err != nil {
			return err
		}
	}
	return
}
func (kratosHydraAuth *KratosHydraAuth) PerformPreDeleteTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreDeleteTask - Start")
	for _, v := range kratosHydraAuth.Hydra.HydraClients {
		err = kratosHydraAuth.Hydra.RemoveHydraClient(ctx, v.ClientId)
		if err != nil {
			return err
		}
	}
	return
}
func (kratosHydraAuth *KratosHydraAuth) ensureCookieFlowId(ctx context.Context, flowType string, r *http.Request) (cirf_token string, flow_id string, err error) {
	logs.WithContext(ctx).Debug("ensureCookieFlowId - Start")
	//ctx := context.Background()
	// fetch flowID from url query parameters
	flowId := r.URL.Query().Get("flow")
	// fetch cookie from headers
	cookie := r.Header.Get("cookie")
	if flowId == "" || cookie == "" {
		logs.WithContext(ctx).Info("inside flowId == \"\" || cookie == \"\" ")
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

		flowRes, flowErr := utils.ExecuteHttp((&newR).Context(), &newR)
		//flowRes, flowErr := httpClient.Do(&newR)
		if flowErr != nil {
			logs.WithContext(ctx).Error(flowErr.Error())
			err = flowErr
			return
		}
		loginFLowFromRes := json.NewDecoder(flowRes.Body)
		loginFLowFromRes.DisallowUnknownFields()
		var loginFlow KratosFlow

		if err = loginFLowFromRes.Decode(&loginFlow); err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return
		}
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

func (kratosHydraAuth *KratosHydraAuth) getFlowId(ctx context.Context, flowType string) (flow_id string, err error) {
	logs.WithContext(ctx).Debug("getFlowId - Start")
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

	flowRes, _, _, _, flowErr := utils.CallHttp(ctx, http.MethodGet, url.String(), headers, nil, nil, nil, nil)
	if flowErr != nil {
		err = flowErr
		return
	}
	loginFLowFromRes, loginFLowFromResErr := json.Marshal(flowRes)
	if loginFLowFromResErr != nil {
		err = loginFLowFromResErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	var loginFlow KratosFlow
	if err = json.Unmarshal(loginFLowFromRes, &loginFlow); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return loginFlow.Id, nil
}

func (kratosHydraAuth *KratosHydraAuth) Logout(ctx context.Context, req *http.Request) (res interface{}, resStatusCode int, err error) {
	logs.WithContext(ctx).Debug("Logout - Start")
	sessionToken := ""
	tokenObj := make(map[string]interface{})
	//todo - remove hardcoding of claims and change it to projectConfig.TokenSecret.HeaderKey
	tokenStr := req.Header.Get("Claims")
	logs.WithContext(ctx).Info(tokenStr)
	if tokenStr != "" {
		err = json.Unmarshal([]byte(tokenStr), &tokenObj)
		if err != nil {
			return nil, 400, err
		}
	}
	logs.WithContext(ctx).Info(fmt.Sprint(tokenObj))
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
	if sessionToken == "" {
		err = errors.New("session token not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, 400, err
	}

	refreshTokenFromReq := json.NewDecoder(req.Body)
	refreshTokenFromReq.DisallowUnknownFields()
	refreshTokenObj := make(map[string]string)
	if rtErr := refreshTokenFromReq.Decode(&refreshTokenObj); rtErr != nil {
		err = rtErr
		logs.WithContext(ctx).Error(err.Error())
		resStatusCode = 400
		return
	}
	refreshToken := ""
	if rt, ok := refreshTokenObj["refresh_token"]; ok {
		refreshToken = rt
	}
	if refreshToken == "" {
		err = errors.New("refresh token not found")
		logs.WithContext(ctx).Error(err.Error())
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

	res, _, _, resStatusCode, logoutErr := utils.CallHttp(ctx, http.MethodDelete, fmt.Sprint(kratosHydraAuth.Kratos.PublicScheme, "://", kratosHydraAuth.Kratos.PublicHost, port, "/self-service/logout/api"), req.Header, nil, nil, nil, logoutPostBody)
	if logoutErr != nil {
		err = logoutErr
		return
	}
	resStatusCode, err = kratosHydraAuth.Hydra.revokeToken(ctx, refreshToken)
	if res == nil {
		res = make(map[string]interface{})
	}
	return res, resStatusCode, err
}

func (kratosHydraAuth *KratosHydraAuth) VerifyRecovery(ctx context.Context, recoveryPassword RecoveryPassword) (rcMap map[string]string, cookies []*http.Cookie, err error) {
	logs.WithContext(ctx).Debug("VerifyRecovery - Start")
	recoveryFlow := recoveryPassword.Id
	recoveryCode := recoveryPassword.Code

	logs.WithContext(ctx).Info(fmt.Sprint("recoveryFlow = ", recoveryFlow))
	logs.WithContext(ctx).Info(fmt.Sprint("recoveryCode = ", recoveryCode))

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
	res, _, resCookies, resStatusCode, resErr := utils.CallHttp(ctx, http.MethodPost, iUrl.String(), headers, nil, nil, iParams, postBody)
	if resErr != nil {
		err = resErr
		if resStatusCode != 422 {
			err = errors.New("incorrect recovery code. please try again")
			logs.WithContext(ctx).Info(err.Error())
			return
		}
		resBytes = []byte(resErr.Error())
		err = nil
	} else {
		resBytesTmp, resBytesErr := json.Marshal(res)
		if resBytesErr != nil {
			err = resBytesErr
			logs.WithContext(ctx).Error(fmt.Sprint("resBytesErr printed below : ", err.Error()))
			return
		}
		resBytes = resBytesTmp
	}
	resMap := make(map[string]interface{})
	resBody := json.NewDecoder(bytes.NewReader(resBytes))
	resBody.DisallowUnknownFields()

	if resBodyErr := resBody.Decode(&resMap); resBodyErr != nil {
		err = resBodyErr
		logs.WithContext(ctx).Error(fmt.Sprint("resBodyErr printed below : ", err.Error()))
		return
	}
	if redirectLink, rOk := resMap["redirect_browser_to"]; rOk {
		redirectLinkSplit := strings.Split(redirectLink.(string), "flow=")
		if len(redirectLinkSplit) <= 0 {
			err = errors.New(fmt.Sprint("failed to fetch flow id from redirect link :", redirectLink))
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		flowId = redirectLinkSplit[1]
	} else if resUiMap, resUiMapOk := resMap["ui"]; resUiMapOk {
		kratosUi := KratosUI{}
		pResMapUiBytes, pResMapUiBytesErr := json.Marshal(resUiMap)
		if pResMapUiBytesErr != nil {
			err = errors.New(fmt.Sprint("json.Marshal(resUiMap) failed :", pResMapUiBytesErr.Error()))
			logs.WithContext(ctx).Error(err.Error())
			return nil, nil, err
		}
		kratosUiErr := json.Unmarshal(pResMapUiBytes, &kratosUi)
		if kratosUiErr != nil {
			err = errors.New(fmt.Sprint("json.Unmarshal(pResMapUiBytes,&kratosUi) failed :", kratosUiErr.Error()))
			logs.WithContext(ctx).Error(err.Error())
			return nil, nil, err
		}
		if len(kratosUi.Messages) >= 1 {
			err = errors.New(kratosUi.Messages[0].Text)
			logs.WithContext(ctx).Info(err.Error())
			return
		}
	} else {
		err = errors.New("failed to verify recovery code in ui. please try again")
		logs.WithContext(ctx).Info(err.Error())
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprint("flowId = ", flowId))
	if rcMap == nil {
		rcMap = make(map[string]string)
	}
	rcMap["id"] = flowId
	return rcMap, resCookies, nil
}

func (kratosHydraAuth *KratosHydraAuth) CompleteRecovery(ctx context.Context, recoveryPassword RecoveryPassword, cookies []*http.Cookie) (msg string, err error) {
	logs.WithContext(ctx).Debug("CompleteRecovery - Start")
	port := kratosHydraAuth.Kratos.PublicPort
	if port != "" {
		port = fmt.Sprint(":", port)
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Content-Length", strconv.Itoa(0))

	fUrl := url.URL{
		Scheme: kratosHydraAuth.Kratos.PublicScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port),
		Path:   fmt.Sprint("/self-service/settings/api"),
	}

	fParams := make(map[string]string)
	fParams["flow"] = recoveryPassword.Id

	pRes, _, pResCookies, _, _ := utils.CallHttp(ctx, http.MethodGet, fUrl.String(), headers, nil, cookies, fParams, nil)
	pResCookies = append(pResCookies, cookies...)
	csrf_token := ""
	kratosUi := KratosUI{}
	if pResMap, pResMapOk := pRes.(map[string]interface{}); pResMapOk {
		if pResMapUi, pResMapUiOk := pResMap["ui"]; pResMapUiOk {
			pResMapUiBytes, pResMapUiBytesErr := json.Marshal(pResMapUi)
			if pResMapUiBytesErr != nil {
				err = errors.New("json.Marshal(pResMapUi) failed")
				logs.WithContext(ctx).Error(err.Error())
				return "", err
			}

			kratosUiErr := json.Unmarshal(pResMapUiBytes, &kratosUi)
			if kratosUiErr != nil {
				err = errors.New("json.Unmarshal(pResMapUiBytes,&kratosUi) failed")
				logs.WithContext(ctx).Error(err.Error())
				return "", err
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
				logs.WithContext(ctx).Error(err.Error())
				return "", err
			}
		} else {
			err = errors.New("pResMap[\"ui\"]  failed")
			logs.WithContext(ctx).Error(err.Error())
			return "", err
		}
	} else {
		err = errors.New("invalid session")
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}

	sfUrl := url.URL{
		Scheme: kratosHydraAuth.Kratos.PublicScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.PublicHost, port),
		Path:   fmt.Sprint("/self-service/settings"),
	}

	sfPostBody := make(map[string]string)
	sfPostBody["method"] = "password"
	sfPostBody["password"] = recoveryPassword.Password
	sfPostBody["csrf_token"] = csrf_token

	sfRes, _, _, _, sfResErr := utils.CallHttp(ctx, http.MethodPost, sfUrl.String(), headers, nil, pResCookies, fParams, sfPostBody)
	kratosMsgUi := KratosUI{}

	sfResBytes := []byte("")
	if sfResErr != nil {
		err = sfResErr
		sfResBytes = []byte(sfResErr.Error())
		err = nil
	} else {
		sfResBytesTmp, sfResBytesErr := json.Marshal(sfRes)
		if sfResBytesErr != nil {
			err = sfResBytesErr
			logs.WithContext(ctx).Error(err.Error())
			return "", err
		}
		sfResBytes = sfResBytesTmp
	}

	sfResMap := make(map[string]interface{})
	sfResJson := json.NewDecoder(bytes.NewReader(sfResBytes))
	sfResJson.DisallowUnknownFields()

	if sfResJsonErr := sfResJson.Decode(&sfResMap); sfResJsonErr != nil {
		err = sfResJsonErr
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}

	//if sfResMap, sfResMapOk := sfRes.(map[string]interface{}); sfResMapOk {
	if sfResMapUi, sfResMapUiOk := sfResMap["ui"]; sfResMapUiOk {
		sfResMapUiBytes, sfResMapUiBytesErr := json.Marshal(sfResMapUi)
		if sfResMapUiBytesErr != nil {
			err = errors.New("jjson.Marshal(sfResMapUi) failed")
			logs.WithContext(ctx).Error(err.Error())
			return "", err
		}
		kratosMsgErr := json.Unmarshal(sfResMapUiBytes, &kratosMsgUi)
		if kratosMsgErr != nil {
			err = errors.New("json.Unmarshal(sfResMapUiBytes, &kratosMsgUi) failed")
			logs.WithContext(ctx).Error(err.Error())
			return "", err
		}
	} else if sfResMapErr, sfResMapErrOk := sfResMap["error"].(map[string]interface{}); sfResMapErrOk {
		errMsg := sfResMapErr["message"].(string)
		errReason := sfResMapErr["reason"].(string)
		err = errors.New(fmt.Sprint(errMsg, ". ", errReason))
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	} else {
		err = errors.New("failed to read error from source")
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}

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
		logs.WithContext(ctx).Error(err.Error())
		msg = ""
	}
	return msg, err
}

func (kratosHydraAuth *KratosHydraAuth) GenerateRecoveryCode(ctx context.Context, recoveryIdentifier RecoveryPostBody, projectId string, silentFlag bool) (msg string, err error) {
	logs.WithContext(ctx).Debug("GenerateRecoveryCode - Start")
	userid := ""
	firstName := ""
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
	iParams["per_page"] = "2000"

	iRes, _, _, _, iResErr := utils.CallHttp(ctx, http.MethodGet, iUrl.String(), headers, nil, nil, iParams, nil)

	if iResErr != nil {
		err = iResErr
		return
	}
	iResBytes, iResBytesErr := json.Marshal(iRes)
	if iResBytesErr != nil {
		err = iResBytesErr
		logs.WithContext(ctx).Error(err.Error())
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
				if nameObj, nameObjOk := v.Traits["name"].(map[string]interface{}); nameObjOk {
					firstName = nameObj["first"].(string)
				}
				break
			}
		}
		if !userFound {
			err = errors.New(fmt.Sprint("user not found with email ", recoveryIdentifier.Username))
			logs.WithContext(ctx).Info(err.Error())
			return
		}
	} else {
		err = iResDecodeErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	recoveryPostBody := make(map[string]string)
	recoveryPostBody["identity_id"] = userid

	url := url.URL{
		Scheme: kratosHydraAuth.Kratos.AdminScheme,
		Host:   fmt.Sprint(kratosHydraAuth.Kratos.AdminHost, port),
		Path:   fmt.Sprint("/admin/recovery/code"),
	}

	rcRes, _, _, _, rcResErr := utils.CallHttp(ctx, http.MethodPost, url.String(), headers, nil, nil, nil, recoveryPostBody)

	if rcResErr != nil {
		err = rcResErr
		return
	}
	rcResBytes, rcResBytesErr := json.Marshal(rcRes)
	if rcResBytesErr != nil {
		err = errors.New(fmt.Sprint("rcResBytesErr : ", rcResBytesErr.Error()))
		logs.WithContext(ctx).Info(err.Error())
		return
	}
	rcResMap := make(map[string]interface{})
	rcResUmErr := json.Unmarshal(rcResBytes, &rcResMap)
	if rcResUmErr != nil {
		err = errors.New(fmt.Sprint("rcResUmErr : ", rcResUmErr.Error()))
		logs.WithContext(ctx).Info(err.Error())
		return
	}

	recovery_link := strings.Split(rcResMap["recovery_link"].(string), "flow=")
	if len(recovery_link) <= 0 {
		err = errors.New("incorrect recovery link received")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	rCode := rcResMap["recovery_code"].(string)
	rExpiryStr := rcResMap["expires_at"].(string)
	rExpiryStr = rExpiryStr[0:strings.LastIndex(rExpiryStr, ".")]

	err = kratosHydraAuth.sendCode(ctx, recoveryIdentifier.Username, rCode, fmt.Sprint(rExpiryStr, " GMT"), firstName, projectId, OTP_PURPOSE_RECOVERY, "email")
	if err != nil {
		return "", err
	}
	//_ = firstName
	return recovery_link[1], nil
}
func (kratosHydraAuth *KratosHydraAuth) Login(ctx context.Context, loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {
	logs.WithContext(ctx).Debug("Login - Start")
	flowId, flowErr := kratosHydraAuth.getFlowId(ctx, "login")
	if flowErr != nil {
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

	loginRes, _, _, _, loginErr := utils.CallHttp(ctx, http.MethodPost, url.String(), headers, nil, nil, params, kratosLoginPostBody)
	loginResBytes := []byte("")
	if loginErr != nil {
		err = loginErr
		loginResBytes = []byte(loginErr.Error())
		//return
	} else {
		loginResJson, loginResJsonErr := json.Marshal(loginRes)
		if loginResJsonErr != nil {
			logs.WithContext(ctx).Error(loginResJsonErr.Error())
			return
		}
		loginResBytes = loginResJson
	}
	logs.WithContext(ctx).Info(string(loginResBytes))
	loginBodyFromRes := json.NewDecoder(bytes.NewReader(loginResBytes))
	loginBodyFromRes.DisallowUnknownFields()
	var kratosSession KratosSession
	var kratosLoginFlow KratosFlow
	kratosLoginSucceed := true
	if loginBodyFromResDecodeErr := loginBodyFromRes.Decode(&kratosSession); loginBodyFromResDecodeErr != nil {
		logs.WithContext(ctx).Error(loginBodyFromResDecodeErr.Error())
		kratosLoginSucceed = false
		loginBodyFromRes = json.NewDecoder(bytes.NewReader(loginResBytes))
		loginBodyFromRes.DisallowUnknownFields()
		if err = loginBodyFromRes.Decode(&kratosLoginFlow); err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	}

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
			loginChallenge, loginChallengeCookies, loginChallengeErr := kratosHydraAuth.getLoginChallenge(ctx)
			if loginChallengeErr != nil {
				err = loginChallengeErr
				return
			}

			consentChallenge, loginAcceptRequestCookies, loginAcceptErr := kratosHydraAuth.Hydra.AcceptLoginRequest(ctx, kratosSession.Session.Identity.Id, loginChallenge, loginChallengeCookies)
			if loginAcceptErr != nil {
				err = loginAcceptErr
				return
			}
			identityHolder := make(map[string]interface{})
			identityHolder["identity"] = identity
			tokens, cosentAcceptErr := kratosHydraAuth.Hydra.AcceptConsentRequest(ctx, identityHolder, consentChallenge, loginAcceptRequestCookies)
			if cosentAcceptErr != nil {
				err = cosentAcceptErr
				return
			}
			return identity, tokens, nil
		}
		return identity, LoginSuccess{}, nil
	} else {
		err = errors.New(fmt.Sprint("Login Failed : ", kratosLoginFlow.UI.Messages))
		logs.WithContext(ctx).Info(err.Error())
		return
	}
	return
}

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
func (kratosHydraAuth *KratosHydraAuth) GetUser(ctx context.Context, userId string) (identity Identity, err error) {
	logs.WithContext(ctx).Debug("GetUser - Start")
	kIdentity, err := kratosHydraAuth.getKratosUser(ctx, userId)
	if err != nil {
		return Identity{}, err
	}
	return convertToIdentity(ctx, kIdentity)
}

func (kratosHydraAuth *KratosHydraAuth) FetchTokens(ctx context.Context, refreshToken string, userId string) (res interface{}, err error) {
	logs.WithContext(ctx).Debug("FetchTokens - Start")

	_, err = kratosHydraAuth.Hydra.fetchTokens(ctx, refreshToken)
	if err != nil {
		return
	}
	kratosIdentity, err := kratosHydraAuth.getKratosUser(ctx, userId)
	if err != nil {
		return
	}
	identity, err := convertToIdentity(ctx, kratosIdentity)
	if err != nil {
		return
	}
	loginChallenge, loginChallengeCookies, loginChallengeErr := kratosHydraAuth.getLoginChallenge(ctx)
	if loginChallengeErr != nil {
		err = loginChallengeErr
		return
	}

	consentChallenge, loginAcceptRequestCookies, loginAcceptErr := kratosHydraAuth.Hydra.AcceptLoginRequest(ctx, identity.Id, loginChallenge, loginChallengeCookies)
	if loginAcceptErr != nil {
		err = loginAcceptErr
		return
	}
	identityHolder := make(map[string]interface{})
	identityHolder["identity"] = identity
	tokens, cosentAcceptErr := kratosHydraAuth.Hydra.AcceptConsentRequest(ctx, identityHolder, consentChallenge, loginAcceptRequestCookies)
	if cosentAcceptErr != nil {
		err = cosentAcceptErr
		return
	}
	return tokens, nil
}

func (kratosHydraAuth *KratosHydraAuth) getKratosUser(ctx context.Context, userId string) (identity KratosIdentity, err error) {
	logs.WithContext(ctx).Debug("getKratosUser - Start")
	dummyMap := make(map[string]string)
	headers := http.Header{}
	res, _, _, _, err := utils.CallHttp(ctx, "GET", fmt.Sprint(kratosHydraAuth.Kratos.getAdminUrl(), "/admin/identities/", userId), headers, dummyMap, nil, dummyMap, dummyMap)
	if err != nil {
		return KratosIdentity{}, err
	}

	if userInfo, ok := res.(map[string]interface{}); !ok {
		err = errors.New("User response not in expected format")
		logs.WithContext(ctx).Error(err.Error())
		return KratosIdentity{}, err
	} else {
		kratosIdentityObjJson, iErr := json.Marshal(userInfo)
		if iErr != nil {
			err = errors.New(fmt.Sprint("Json marshal error for identityObj : ", iErr.Error()))
			logs.WithContext(ctx).Error(err.Error())
			return KratosIdentity{}, err
		}
		iiErr := json.Unmarshal(kratosIdentityObjJson, &identity)
		if iiErr != nil {
			err = errors.New(fmt.Sprint("Json unmarshal error for identityObj : ", iiErr.Error()))
			logs.WithContext(ctx).Error(err.Error())
			return KratosIdentity{}, err
		}
		return
	}
}

func (kratosHydraAuth *KratosHydraAuth) UpdateUser(ctx context.Context, identityToUpdate Identity, userId string, token map[string]interface{}) (tokens interface{}, err error) {
	logs.WithContext(ctx).Debug("UpdateUser - Start")
	//userId := identityToUpdate.Attributes["sub"].(string)
	kratosIdentity, err := kratosHydraAuth.getKratosUser(ctx, userId)
	err = convertFromIdentity(ctx, identityToUpdate, &kratosIdentity)
	if err != nil {
		return nil, err
	}
	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	_, _, _, _, err = utils.CallHttp(ctx, "PUT", fmt.Sprint(kratosHydraAuth.Kratos.getAdminUrl(), "/admin/identities/", userId), headers, dummyMap, nil, dummyMap, kratosIdentity)
	if err != nil {
		return nil, err
	}
	return
}

func (kratosHydraAuth *KratosHydraAuth) ChangePassword(ctx context.Context, tokenObj map[string]interface{}, userId string, changePassword ChangePassword) (err error) {
	logs.WithContext(ctx).Debug("ChangePassword - Start")
	headers := http.Header{}
	headers.Set("Origin", "")
	sessionToken := ""
	identifier := ""

	if identity, ok := tokenObj["identity"]; ok {
		i, e := json.Marshal(identity)
		if e != nil {
			logs.WithContext(ctx).Error(e.Error())
		}
		identityMap := Identity{}
		ee := json.Unmarshal(i, &identityMap)
		if ee != nil {
			logs.WithContext(ctx).Error(ee.Error())
		}
		logs.WithContext(ctx).Info(fmt.Sprint(identityMap))
		sessionToken = identityMap.AuthDetails.SessionToken
		identifier = identityMap.Attributes["email"].(string)

	}
	if sessionToken == "" {
		err = errors.New("session token not found")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
	var loginPostBody LoginPostBody
	loginPostBody.Username = identifier
	loginPostBody.Password = changePassword.OldPassword
	_, _, loginErr := kratosHydraAuth.Login(ctx, loginPostBody, false)
	if loginErr != nil {
		err = errors.New("Incorrect Old Password")
		logs.WithContext(ctx).Info(err.Error())
		return
	}
	headers.Set("X-Session-Token", sessionToken)
	headers.Set("content-type", "application/json")

	port := kratosHydraAuth.Kratos.PublicPort
	if port != "" {
		port = fmt.Sprint(":", port)
	}

	res, _, _, _, resErr := utils.CallHttp(ctx, http.MethodGet, fmt.Sprint(kratosHydraAuth.Kratos.PublicScheme, "://", kratosHydraAuth.Kratos.PublicHost, port, "/self-service/settings/api"), headers, nil, nil, nil, nil)
	if resErr != nil {
		err = resErr
		return
	}
	logs.WithContext(ctx).Info(fmt.Sprint(res))
	flowParams := make(map[string]string)
	if resMap, ok := res.(map[string]interface{}); ok {
		if flowId, fOk := resMap["id"]; fOk {
			flowParams["flow"] = flowId.(string)
		} else {
			err = errors.New("flow id not found")
			logs.WithContext(ctx).Info(err.Error())
			return
		}
	} else {
		err = errors.New("incorrect getflow response")
		logs.WithContext(ctx).Info(err.Error())
		return
	}

	changePasswordBody := make(map[string]string)
	changePasswordBody["method"] = "password"
	changePasswordBody["password"] = changePassword.NewPassword

	_, _, _, _, cpResErr := utils.CallHttp(ctx, http.MethodPost, fmt.Sprint(kratosHydraAuth.Kratos.PublicScheme, "://", kratosHydraAuth.Kratos.PublicHost, port, "/self-service/settings"), headers, nil, nil, flowParams, changePasswordBody)
	if cpResErr != nil {
		err = cpResErr
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
					logs.WithContext(ctx).Info(err.Error())
				} else {
					logs.WithContext(ctx).Error("actualError.(map[string]interface{}) failed")
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
												if uiName == "password" {
													if uiMsg, msgOk := uiNode["messages"]; msgOk {
														if uiMsgArray, msgArrayOk := uiMsg.([]interface{}); msgArrayOk {
															if uiMsgMap, msgMapOk := uiMsgArray[0].(map[string]interface{}); msgMapOk {
																if uiReason, reasonOk := uiMsgMap["text"]; reasonOk {
																	err = errors.New(uiReason.(string))
																	logs.WithContext(ctx).Info(err.Error())
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
							logs.WithContext(ctx).Error(fmt.Sprint("uiErrorNodes.([]map[string]interface{}) failed : ", reflect.TypeOf(uiErrorNodes).String()))
						}
					} else {
						logs.WithContext(ctx).Error("uiErrorMap[\"nodes\"] not found")
					}
				} else {
					logs.WithContext(ctx).Error("uiError[\"nodes\"] not found")
				}
			} else {
				logs.WithContext(ctx).Error("ui or error attribute not found in errMap")
			}
		} else {
			logs.WithContext(ctx).Error(fmt.Sprint("json.Unmarshal(errBytes,&errMap) failed : ", errBytesUmErr.Error()))
		}
		err = errors.New(strings.Replace(err.Error(), "\"", "", -1))
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return err
}

func convertToIdentity(ctx context.Context, kratosIdentity KratosIdentity) (identity Identity, err error) {
	logs.WithContext(ctx).Debug("convertToIdentity - Start")
	identity = Identity{}
	identity.Id = kratosIdentity.Id
	identity.CreatedAt = kratosIdentity.CreatedAt
	identity.UpdatedAt = kratosIdentity.UpdatedAt
	identity.OtherInfo = make(map[string]interface{})
	identity.OtherInfo["schema_id"] = kratosIdentity.SchemaId
	identity.OtherInfo["state_changed_at"] = kratosIdentity.StateChangedAt
	identity.Status = kratosIdentity.State
	identity.AuthDetails = IdentityAuth{}
	if identity.Attributes == nil {
		identity.Attributes = make(map[string]interface{})
	}
	identity.Attributes["sub"] = kratosIdentity.Id
	for k, v := range kratosIdentity.MetaDataPublic {
		identity.Attributes[k] = v
	}
	for k, v := range kratosIdentity.Traits {
		identity.Attributes[k] = v
	}
	return
}

func convertFromIdentity(ctx context.Context, identity Identity, kratosIdentity *KratosIdentity) (err error) {
	logs.WithContext(ctx).Debug("convertFromIdentity - Start")
	logs.WithContext(ctx).Info(fmt.Sprint("original kratosIdentity printed below : ", kratosIdentity))

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
	return
}
