package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/routes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AuthI interface {
	//Login(req *http.Request) (res interface{}, cookies []*http.Cookie, err error)
	Login(ctx context.Context, loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error)
	Logout(ctx context.Context, req *http.Request) (res interface{}, resStatusCode int, err error)
	VerifyToken(ctx context.Context, tokenType string, token string) (res interface{}, err error)
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error)
	FetchTokens(ctx context.Context, refresh_token string) (res interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) (err error)
	PerformPreSaveTask(ctx context.Context) (err error)
	PerformPreDeleteTask(ctx context.Context) (err error)
	GetUser(ctx context.Context, userId string) (identity Identity, err error)
	UpdateUser(ctx context.Context, identityToUpdate Identity) (err error)
	ChangePassword(ctx context.Context, req *http.Request, changePasswordObj ChangePassword) (err error)
	GenerateRecoveryCode(ctx context.Context, recoveryIdentifier RecoveryPostBody) (msg string, err error)
	CompleteRecovery(ctx context.Context, recoveryPassword RecoveryPassword, cookies []*http.Cookie) (msg string, err error)
	VerifyRecovery(ctx context.Context, recoveryPassword RecoveryPassword) (res map[string]string, cookies []*http.Cookie, err error)
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

func (auth *Auth) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	err := errors.New("MakeFromJson Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}

func (auth *Auth) GenerateRecoveryCode(ctx context.Context, recoveryIdentifier RecoveryPostBody) (msg string, err error) {
	err = errors.New("GenerateRecoveryCode Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return "", err
}

func (auth *Auth) sendRecoveryCode(ctx context.Context, email_id string, recovery_code string, recovery_time string, name string) (err error) {
	logs.WithContext(ctx).Debug("sendRecoveryCode - Start")
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
	logs.WithContext(ctx).Info(fmt.Sprint("len(auth.Hooks.SRC.TargetHosts) = ", len(auth.Hooks.SRC.TargetHosts)))
	if len(auth.Hooks.SRC.TargetHosts) > 0 {
		_, _, respErr := auth.Hooks.SRC.Execute(r.Context(), r, "/", false, "", trReqVars, 1)
		return respErr
	} else {
		logs.WithContext(ctx).Warn("SRC hook not defined for auth. Thus no email was triggered.")
	}
	return
}

func (auth *Auth) CompleteRecovery(ctx context.Context, recoveryPassword RecoveryPassword, cookies []*http.Cookie) (msg string, err error) {
	err = errors.New("CompleteRecovery Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return "", err
}

func (auth *Auth) VerifyRecovery(ctx context.Context, recoveryPassword RecoveryPassword) (res map[string]string, cookies []*http.Cookie, err error) {
	err = errors.New("VerifyRecovery Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, nil, err
}

func (auth *Auth) VerifyToken(ctx context.Context, tokenType string, token string) (res interface{}, err error) {
	err = errors.New("VerifyToken Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func (auth *Auth) PerformPreSaveTask(ctx context.Context) (err error) {
	err = errors.New("PerformPreSaveTask Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}
func (auth *Auth) PerformPreDeleteTask(ctx context.Context) (err error) {
	err = errors.New("PerformPreDeleteTask Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}

func (auth *Auth) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "AuthType":
		return auth.AuthType, nil
	case "AuthName":
		return auth.AuthName, nil
	case "TokenHeaderKey":
		return auth.TokenHeaderKey, nil
	default:
		err := errors.New("Attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (auth *Auth) GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error) {
	err = errors.New("GetUserInfo Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return Identity{}, err
}

func (auth *Auth) GetUser(ctx context.Context, userId string) (identity Identity, err error) {
	err = errors.New("GetUser Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return Identity{}, err
}

func (auth *Auth) UpdateUser(ctx context.Context, identityToUpdate Identity) (err error) {
	err = errors.New("UpdateUser Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}

func (auth *Auth) FetchTokens(ctx context.Context, refresh_token string) (res interface{}, err error) {
	err = errors.New("FetchTokens Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func (auth *Auth) Login(ctx context.Context, loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {
	err = errors.New("Login Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return Identity{}, LoginSuccess{}, err
}

func (auth *Auth) Logout(ctx context.Context, req *http.Request) (res interface{}, resStatusCode int, err error) {
	err = errors.New("Logout Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, 400, err
}

func (auth *Auth) ChangePassword(ctx context.Context, req *http.Request, changePasswordObj ChangePassword) (err error) {
	err = errors.New("ChangePassword Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
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
