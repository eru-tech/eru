package auth

import (
	"encoding/json"
	"errors"
	"github.com/eru-tech/eru/eru-routes/routes"
	utils "github.com/eru-tech/eru/eru-utils"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AuthI interface {
	//Login(req *http.Request) (res interface{}, cookies []*http.Cookie, err error)
	Login(loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error)
	Logout(req *http.Request) (res interface{}, resStatusCode int, err error)
	VerifyToken(tokenType string, token string) (res interface{}, err error)
	GetAttribute(attributeName string) (attributeValue interface{}, err error)
	GetUserInfo(access_token string) (identity Identity, err error)
	FetchTokens(refresh_token string) (res interface{}, err error)
	MakeFromJson(rj *json.RawMessage) (err error)
	PerformPreSaveTask() (err error)
	PerformPreDeleteTask() (err error)
	GetUser(userId string) (identity Identity, err error)
	UpdateUser(identityToUpdate Identity) (err error)
	ChangePassword(req *http.Request, changePasswordObj ChangePassword) (err error)
	GenerateRecoveryCode(recoveryIdentifier RecoveryPostBody) (msg string, err error)
	CompleteRecovery(recoveryPassword RecoveryPassword, cookies []*http.Cookie) (msg string, err error)
	VerifyRecovery(recoveryPassword RecoveryPassword) (res map[string]string, cookies []*http.Cookie, err error)
}
type ChangePassword struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type LoginPostBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RecoveryPostBody struct {
	Username string `json:"username"`
}

type RecoveryPassword struct {
	Code     string `json:"code"`
	Id       string `json:"id"`
	Password string `json:"password"`
}

type Identity struct {
	Id          string                 `json:"id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Attributes  map[string]interface{} `json:"attributes"`
	AuthDetails IdentityAuth           `json:"auth_details"`
	OtherInfo   map[string]interface{} `json:"other_info"`
	Status      string                 `json:"status"`
}

type IdentityAuth struct {
	SessionToken                string        `json:"session_token"`
	SessionId                   string        `json:"session_id"`
	SessionStatus               bool          `json:"session_status"`
	ExpiresAt                   time.Time     `json:"expires_at"`
	AuthenticatedAt             time.Time     `json:"authenticated_at"`
	AuthenticatorAssuranceLevel string        `json:"authenticator_assurance_level"`
	AuthenticationMethods       []interface{} `json:"authentication_methods"`
	IssuedAt                    time.Time     `json:"issued_at"`
}

type Auth struct {
	AuthType       string
	AuthName       string
	TokenHeaderKey string
	Hooks          AuthHooks
}

type AuthHooks struct {
	SRC routes.Route
}

func (auth *Auth) MakeFromJson(rj *json.RawMessage) error {
	return errors.New("MakeFromJson Method not implemented")
}

func (auth *Auth) GenerateRecoveryCode(recoveryIdentifier RecoveryPostBody) (msg string, err error) {
	return "", errors.New("GenerateRecoveryCode Method not implemented")
}

func (auth *Auth) sendRecoveryCode(email_id string, recovery_code string, recovery_time string, name string) (err error) {
	trReqVars := &routes.TemplateVars{}
	if trReqVars.Vars == nil {
		trReqVars.Vars = make(map[string]interface{})
	}
	trReqVars.Vars["email_id"] = email_id
	trReqVars.Vars["recovery_code"] = recovery_code
	trReqVars.Vars["recovery_time"] = recovery_time
	trReqVars.Vars["name"] = name
	r := &http.Request{}
	url := url.URL{
		Scheme: "",
		Host:   "",
		Path:   "/",
	}
	r.URL = &url
	r.Body = io.NopCloser(strings.NewReader("{}"))
	h := http.Header{}
	h.Set("content-type", "application/json")
	r.Header = h
	response, _, respErr := auth.Hooks.SRC.Execute(r, "/", false, "", trReqVars, 1)
	utils.PrintResponseBody(response, "send recovery code response")
	return respErr
}

func (auth *Auth) CompleteRecovery(recoveryPassword RecoveryPassword, cookies []*http.Cookie) (msg string, err error) {
	return "", errors.New("CompleteRecovery Method not implemented")
}

func (auth *Auth) VerifyRecovery(recoveryPassword RecoveryPassword) (res map[string]string, cookies []*http.Cookie, err error) {
	return nil, nil, errors.New("CompleteRecovery Method not implemented")
}

func (auth *Auth) VerifyToken(tokenType string, token string) (res interface{}, err error) {
	return nil, errors.New("VerifyToken Method not implemented")
}

func (auth *Auth) PerformPreSaveTask() (err error) {
	return errors.New("PerformPreSaveTask Method not implemented")
}
func (auth *Auth) PerformPreDeleteTask() (err error) {
	return errors.New("PerformPreDeleteTask Method not implemented")
}

func (auth *Auth) GetAttribute(attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "AuthType":
		return auth.AuthType, nil
	case "AuthName":
		return auth.AuthName, nil
	case "TokenHeaderKey":
		return auth.TokenHeaderKey, nil
	default:
		return nil, errors.New("Attribute not found")
	}
}

func (auth *Auth) GetUserInfo(access_token string) (identity Identity, err error) {
	return Identity{}, errors.New("GetUserInfo Method not implemented")
}

func (auth *Auth) GetUser(userId string) (identity Identity, err error) {
	return Identity{}, errors.New("GetUser Method not implemented")
}

func (auth *Auth) UpdateUser(identityToUpdate Identity) (err error) {
	return errors.New("UpdateUser Method not implemented")
}

func (auth *Auth) FetchTokens(refresh_token string) (res interface{}, err error) {
	return nil, errors.New("FetchTokens Method not implemented")
}

func (auth *Auth) Login(loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {
	return Identity{}, LoginSuccess{}, errors.New("Login Method not implemented")
}

func (auth *Auth) Logout(req *http.Request) (res interface{}, resStatusCode int, err error) {
	return nil, 400, errors.New("Login Method not implemented")
}

func (auth *Auth) ChangePassword(req *http.Request, changePasswordObj ChangePassword) (err error) {
	return errors.New("ChangePassword Method not implemented")
}

func GetAuth(authType string) AuthI {
	switch authType {
	case "KRATOS-HYDRA":
		return new(KratosHydraAuth)
	default:
		return new(Auth)
	}
	return nil
}
