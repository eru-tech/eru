package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type AuthI interface {
	Login(req *http.Request) (res interface{}, cookies []*http.Cookie, err error)
	Logout(req *http.Request) (res interface{}, resStatusCode int, err error)
	VerifyToken(tokenType string, token string) (res interface{}, err error)
	GetAttribute(attributeName string) (attributeValue interface{}, err error)
	GetUserInfo(access_token string) (identity Identity, err error)
	FetchTokens(refresh_token string) (res interface{}, err error)
	MakeFromJson(rj *json.RawMessage) (err error)
	PerformPreSaveTask() (err error)
	PerformPreDeleteTask() (err error)
}

type LoginPostBody struct {
	Username string `json:"username"`
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
}

func (auth *Auth) MakeFromJson(rj *json.RawMessage) error {
	return errors.New("MakeFromJson Method not implemented")
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

func (auth *Auth) FetchTokens(refresh_token string) (res interface{}, err error) {
	return nil, errors.New("FetchTokens Method not implemented")
}

func (auth *Auth) Login(req *http.Request) (res interface{}, cookies []*http.Cookie, err error) {
	return nil, nil, errors.New("Login Method not implemented")
}

func (auth *Auth) Logout(req *http.Request) (res interface{}, resStatusCode int, err error) {
	return nil, 400, errors.New("Login Method not implemented")
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
