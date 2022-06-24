package auth

import (
	"encoding/json"
	"errors"
	"net/http"
)

type AuthI interface {
	Login(req *http.Request) (res interface{}, cookies []*http.Cookie, err error)
	VerifyToken(tokenType string, token string) (res interface{}, err error)
	GetAttribute(attributeName string) (attributeValue interface{}, err error)
	MakeFromJson(rj *json.RawMessage) (err error)
	PerformPreSaveTask() (err error)
	PerformPreDeleteTask() (err error)
}

type LoginPostBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
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

func (auth *Auth) Login(req *http.Request) (res interface{}, cookies []*http.Cookie, err error) {
	return nil, nil, errors.New("Login Method not implemented")
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
