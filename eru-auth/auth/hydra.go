package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-crypto/jwt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
	"time"
)

type LoginSuccess struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IdToken      string    `json:"id_token"`
	Expiry       time.Time `json:"expiry"`
	ExpiresIn    float64   `json:"expires_in"`
	Id           string    `json:"id"`
}

type HydraConfig struct {
	PublicHost   string                 `json:"public_host"`
	PublicPort   string                 `json:"public_port"`
	PublicScheme string                 `json:"public_scheme"`
	AdminHost    string                 `json:"admin_host"`
	AdminPort    string                 `json:"admin_port"`
	AdminScheme  string                 `json:"admin_scheme"`
	AuthURL      string                 `json:"auth_url"`
	TokenURL     string                 `json:"token_url"`
	HydraClients map[string]HydraClient `json:"hydra_clients"`
}

type hydraAcceptLoginRequest struct {
	Remember    bool   `json:"remember"`
	RememberFor int    `json:"remember_for"`
	Subject     string `json:"subject"`
}

type hydraAcceptConsentRequest struct {
	Remember    bool                             `json:"remember"`
	RememberFor int                              `json:"remember_for"`
	GrantScope  []string                         `json:"grant_scope"`
	HandledAt   time.Time                        `json:"handled_at"`
	Session     hydraAcceptConsentRequestSession `json:"session"`
}

type hydraAcceptConsentRequestSession struct {
	IdToken interface{} `json:"id_token"`
}

type HydraClient struct {
	ClientId                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	ClientSecret            string   `json:"client_secret"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
}

func (kratosHydraAuth *KratosHydraAuth) getLoginChallenge(ctx context.Context) (loginChallenge string, cookies []*http.Cookie, err error) {
	logs.WithContext(ctx).Debug("getLoginChallenge - Start")
	return kratosHydraAuth.Hydra.GetLoginChallenge(ctx)
}

func (hydra HydraConfig) GetLoginChallenge(ctx context.Context) (loginChallenge string, cookies []*http.Cookie, err error) {
	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("generate state failed: %v", err.Error()))
		return
	}
	state := base64.StdEncoding.EncodeToString(b)

	hydraClientId := ""
	for _, v := range hydra.HydraClients {
		hydraClientId = v.ClientId
		break
	}
	outhConfig, ocErr := hydra.GetOauthConfig(ctx, hydraClientId)
	if ocErr != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("generate state failed: %v", err.Error()))
		err = ocErr
		return
	}
	redirectTo := outhConfig.AuthCodeURL(state)

	_, headers, respCookies, statusCode, err := utils.CallHttp(ctx, http.MethodGet, redirectTo, nil, nil, nil, nil, nil)
	cookies = respCookies
	if statusCode >= 300 && statusCode < 400 {
		redirectLocation := headers["Location"][0]
		logs.WithContext(ctx).Info(redirectLocation)
		params := strings.Split(redirectLocation, "?")[1]
		logs.WithContext(ctx).Info(fmt.Sprint(params))
		loginChallenge = strings.Split(params, "=")[1]
	}
	return
}

func (kratosHydraAuth *KratosHydraAuth) GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error) {
	logs.WithContext(ctx).Debug("GetUserInfo - Start")
	return kratosHydraAuth.Hydra.GetUserInfo(ctx, access_token)
}

func (hydraConfig HydraConfig) GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error) {
	logs.WithContext(ctx).Debug("GetUserInfo - Start")
	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Add("Authorization", fmt.Sprint("Bearer ", access_token))
	res, _, _, _, err := utils.CallHttp(ctx, "POST", fmt.Sprint(hydraConfig.GetPublicUrl(), "/userinfo"), headers, dummyMap, nil, dummyMap, dummyMap)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("error in http.Get GetUserInfo : ", err.Error()))
		return Identity{}, err
	}
	/*
		if sc >= 400{
			resBytes, bytesErr := json.Marshal(res)
			if bytesErr != nil {
				return Identity{}, bytesErr
			}
			err = errors.New(strings.Replace(string(resBytes),"\"","",-1))
			return Identity{}, err
		}
	*/
	if userInfo, ok := res.(map[string]interface{}); !ok {
		err = errors.New("User Info response not in expected format")
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, err
	} else {
		if identityObj, isIdOk := userInfo["identity"]; !isIdOk {
			err = errors.New("Identity attribute missing in User Info")
			logs.WithContext(ctx).Error(err.Error())
			return Identity{}, err
		} else {
			identityObjJson, iErr := json.Marshal(identityObj)
			if iErr != nil {
				err = errors.New(fmt.Sprint("Json marshal error for identityObj : ", iErr.Error()))
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, err
			}
			iiErr := json.Unmarshal(identityObjJson, &identity)
			if iiErr != nil {
				err = errors.New(fmt.Sprint("Json unmarshal error for identityObj : ", iiErr.Error()))
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, err
			}
			return
		}
	}
}

func (kratosHydraAuth *KratosHydraAuth) FetchTokens(ctx context.Context, refresh_token string, userId string) (res interface{}, err error) {
	logs.WithContext(ctx).Debug("FetchTokens - Start")
	return kratosHydraAuth.Hydra.fetchTokens(ctx, refresh_token)
}

func (hydraConfig HydraConfig) fetchTokens(ctx context.Context, refresh_token string) (res interface{}, err error) {
	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Add("content-type", "application/x-www-form-urlencoded")
	formData := make(map[string]string)
	formData["refresh_token"] = refresh_token
	formData["grant_type"] = "refresh_token"
	for _, v := range hydraConfig.HydraClients {
		formData["client_id"] = v.ClientId
		break
	}
	res, _, _, _, err = utils.CallHttp(ctx, "POST", fmt.Sprint(hydraConfig.GetPublicUrl(), "/oauth2/token"), headers, formData, nil, dummyMap, dummyMap)
	if err != nil {
		return nil, err
	}
	loginSuccess := LoginSuccess{}
	ok := false
	if resMap, resOk := res.(map[string]interface{}); !resOk {
		err = errors.New("Response is not map[string]string")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	} else {
		if loginSuccess.AccessToken, ok = resMap["access_token"].(string); !ok {
			err = errors.New("access_token attribute missing from response")
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		if loginSuccess.IdToken, ok = resMap["id_token"].(string); !ok {
			err = errors.New("id_token attribute missing from response")
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		if loginSuccess.RefreshToken, ok = resMap["refresh_token"].(string); !ok {
			err = errors.New("refresh_token attribute missing from response")
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
		if loginSuccess.ExpiresIn, ok = resMap["expires_in"].(float64); !ok {
			err = errors.New("expires_in attribute missing from response")
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
	}
	return loginSuccess, nil
}

func (kratosHydraAuth *KratosHydraAuth) VerifyToken(ctx context.Context, tokenType string, token string) (res interface{}, err error) {
	logs.WithContext(ctx).Debug("VerifyToken - Start")
	switch tokenType {
	case "id":
		return kratosHydraAuth.Hydra.verifyIdToken(ctx, token)
	case "access":
		return kratosHydraAuth.Hydra.verifyAccessToken(ctx, token)
	case "refresh":
		return kratosHydraAuth.Hydra.verifyRefreshToken(ctx, token)
	default:
		//do nothing
	}
	err = errors.New(fmt.Sprint("tokenType Mismatch : ", tokenType))
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func (hydraConfig HydraConfig) verifyIdToken(ctx context.Context, token string) (res interface{}, err error) {
	logs.WithContext(ctx).Debug("verifyIdToken - Start")
	jwkUrl := fmt.Sprint(hydraConfig.GetPublicUrl(), "/.well-known/jwks.json")
	claims, err := jwt.DecryptTokenJWK(ctx, token, jwkUrl)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return claims, nil
}

func (hydraConfig HydraConfig) verifyAccessToken(ctx context.Context, token string) (res interface{}, err error) {
	logs.WithContext(ctx).Debug("verifyAccessToken - Start")
	dummyMap := make(map[string]string)
	formData := make(map[string]string)
	formData["token"] = token
	headers := http.Header{}
	headers.Add("content-type", "application/x-www-form-urlencoded")
	res, _, _, _, err = utils.CallHttp(ctx, "POST", fmt.Sprint(hydraConfig.GetAminUrl(), "/admin/oauth2/introspect"), headers, formData, nil, dummyMap, dummyMap)
	if err != nil {
		return nil, err
	}

	if verifiedToken, ok := res.(map[string]interface{}); !ok {
		err = errors.New("Verified token response not in expected format")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	} else {
		if isActive, isActiveOk := verifiedToken["active"]; !isActiveOk {
			err = errors.New("Verified token response not in expected format")
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		} else {
			if isActive.(bool) {
				return verifiedToken, nil
			} else {
				err = errors.New("Invalid Token")
				logs.WithContext(ctx).Error(err.Error())
				return nil, err
			}
		}
	}
}

func (hydraConfig HydraConfig) verifyRefreshToken(ctx context.Context, token string) (res interface{}, err error) {
	logs.WithContext(ctx).Debug("verifyRefreshToken - Start")
	return hydraConfig.verifyAccessToken(ctx, token)
}

func (hydraConfig HydraConfig) GetOauthConfig(ctx context.Context, clientId string) (oauth2Config *oauth2.Config, err error) {
	logs.WithContext(ctx).Debug("getOauthConfig - Start")
	if hc, ok := hydraConfig.HydraClients[clientId]; ok {
		return &oauth2.Config{
			RedirectURL:  hc.RedirectURIs[0],
			ClientID:     clientId,
			ClientSecret: hc.ClientSecret,
			Scopes:       strings.Split(hc.Scope, " "),
			Endpoint: oauth2.Endpoint{
				AuthURL:  hydraConfig.AuthURL,
				TokenURL: hydraConfig.TokenURL,
			},
		}, nil
	} else {
		err = errors.New(fmt.Sprint(clientId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (hydraConfig HydraConfig) RemoveHydraClient(ctx context.Context, clientId string) (err error) {
	logs.WithContext(ctx).Debug("RemoveHydraClient - Start")
	if _, ok := hydraConfig.HydraClients[clientId]; ok {
		_, _, _, err = hydraConfig.DeleteHydraClient(ctx, clientId)
		if err != nil {
			return
		}
	} else {
		err = errors.New(fmt.Sprint("hydra client not found : ", clientId))
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return nil
}
func (hydraConfig HydraConfig) SaveHydraClient(ctx context.Context, hydraClient HydraClient) (err error) {
	logs.WithContext(ctx).Debug("SaveHydraClient - Start")
	_, _, _, err = hydraConfig.GetHydraClient(ctx, hydraClient.ClientId)
	if err != nil {
		logs.WithContext(ctx).Info(fmt.Sprint("calling create api for hydraclient : ", hydraClient.ClientId))
		_, _, _, err = hydraConfig.CreateHydraClient(ctx, hydraClient)
	} else {
		logs.WithContext(ctx).Info(fmt.Sprint("calling update api for hydraclient : ", hydraClient.ClientId))
		_, _, _, err = hydraConfig.UpdateHydraClient(ctx, hydraClient.ClientId, hydraClient)
	}
	return
}

func (hydraConfig HydraConfig) GetHydraClient(ctx context.Context, clientId string) (resp interface{}, headers map[string][]string, statusCode int, err error) {
	logs.WithContext(ctx).Debug("getHydraClient - Start")
	dummyMap := make(map[string]string)
	logs.WithContext(ctx).Info(fmt.Sprint(hydraConfig.GetAminUrl(), "/admin/clients/", clientId))
	resp, headers, _, statusCode, err = utils.CallHttp(ctx, "GET", fmt.Sprint(hydraConfig.GetAminUrl(), "/admin/clients/", clientId), nil, nil, nil, dummyMap, dummyMap)
	if err != nil {
		return nil, nil, 0, err
	}
	return resp, headers, statusCode, checkResponseError(ctx, resp)
}
func (hydraConfig HydraConfig) CreateHydraClient(ctx context.Context, hydraClient HydraClient) (resp interface{}, headers http.Header, statusCode int, err error) {
	logs.WithContext(ctx).Debug("createHydraClient - Start")
	dummyMap := make(map[string]string)
	headers = http.Header{}
	headers.Add("Content-Type", "application/json")
	resp, headers, _, statusCode, err = utils.CallHttp(ctx, "POST", fmt.Sprint(hydraConfig.GetAminUrl(), "/admin/clients"), headers, dummyMap, nil, dummyMap, hydraClient)
	if err != nil {
		return nil, nil, 0, err
	}
	logs.WithContext(ctx).Info(fmt.Sprint(resp))
	return resp, headers, statusCode, checkResponseError(ctx, resp)
}
func (hydraConfig HydraConfig) UpdateHydraClient(ctx context.Context, clientId string, hydraClient HydraClient) (resp interface{}, headers http.Header, statusCode int, err error) {
	logs.WithContext(ctx).Debug("updateHydraClient - Start")
	dummyMap := make(map[string]string)
	headers = http.Header{}
	headers.Add("Content-Type", "application/json")
	resp, headers, _, statusCode, err = utils.CallHttp(ctx, "PUT", fmt.Sprint(hydraConfig.GetAminUrl(), "/admin/clients/", clientId), headers, dummyMap, nil, dummyMap, hydraClient)
	if err != nil {
		return nil, nil, 0, err
	}
	return resp, headers, statusCode, checkResponseError(ctx, resp)
}
func (hydraConfig HydraConfig) DeleteHydraClient(ctx context.Context, clientId string) (resp interface{}, headers http.Header, statusCode int, err error) {
	logs.WithContext(ctx).Debug("deleteHydraClient - Start")
	dummyMap := make(map[string]string)
	headers = http.Header{}
	headers.Add("Content-Type", "application/json")
	resp, headers, _, statusCode, err = utils.CallHttp(ctx, "DELETE", fmt.Sprint(hydraConfig.GetAminUrl(), "/admin/clients/", clientId), headers, dummyMap, nil, dummyMap, dummyMap)
	if err != nil {
		return nil, nil, 0, err
	}
	if resp != nil {
		return resp, headers, statusCode, checkResponseError(ctx, resp)
	}
	return resp, headers, statusCode, nil
}
func checkResponseError(ctx context.Context, resp interface{}) (err error) {
	logs.WithContext(ctx).Debug("checkResponseError - Start")
	if respMap, ok := resp.(map[string]interface{}); !ok {
		err = errors.New("resp.(map[string]interface{}) conversion failed")
		logs.WithContext(ctx).Error(err.Error())
		return
	} else {
		if errResp, ok := respMap["error"]; ok {
			err = errors.New(errResp.(string))
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	}
	return
}

func (hydraConfig HydraConfig) GetPublicUrl() (url string) {
	port := ""
	if hydraConfig.PublicPort != "" {
		port = fmt.Sprint(":", hydraConfig.PublicPort)
	}
	return fmt.Sprint(hydraConfig.PublicScheme, "://", hydraConfig.PublicHost, port)
}
func (hydraConfig HydraConfig) GetAminUrl() (url string) {
	port := ""
	if hydraConfig.AdminPort != "" {
		port = fmt.Sprint(":", hydraConfig.AdminPort)
	}
	return fmt.Sprint(hydraConfig.AdminScheme, "://", hydraConfig.AdminHost, port)
}
func (hydraConfig HydraConfig) AcceptLoginRequest(ctx context.Context, subject string, loginChallenge string, cookies []*http.Cookie) (consentChallenge string, respCookies []*http.Cookie, err error) {
	logs.WithContext(ctx).Debug("acceptLoginRequest - Start")
	hydraALR := hydraAcceptLoginRequest{}
	hydraALR.Remember = true
	hydraALR.RememberFor = 0
	hydraALR.Subject = subject
	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Add("content-type", "application/json")
	paramsMap := make(map[string]string)
	paramsMap["login_challenge"] = loginChallenge
	postUrl := fmt.Sprint(hydraConfig.GetAminUrl(), "/admin/oauth2/auth/requests/login/accept")
	resp, _, respCookies, _, err := utils.CallHttp(ctx, http.MethodPut, postUrl, headers, dummyMap, cookies, paramsMap, hydraALR)
	respCookies = append(respCookies, cookies...)
	if err != nil {
		return "", nil, err
	}
	if respMap, ok := resp.(map[string]interface{}); ok {
		if redirectUrl, ok1 := respMap["redirect_to"]; ok1 {
			logs.WithContext(ctx).Info(redirectUrl.(string))
			_, respHeaders2, respCookies2, statusCode2, err2 := utils.CallHttp(ctx, http.MethodGet, redirectUrl.(string), headers, dummyMap, respCookies, dummyMap, dummyMap)
			respCookies = append(respCookies, respCookies2...)
			if err2 != nil {
				return "", nil, err
			}
			if statusCode2 >= 300 && statusCode2 < 400 {
				redirectLocation := respHeaders2["Location"][0]
				logs.WithContext(ctx).Info(redirectLocation)
				params := strings.Split(redirectLocation, "?")[1]
				logs.WithContext(ctx).Info(fmt.Sprint(params))
				consentChallenge = strings.Split(params, "=")[1]
			}
		} else {
			err = errors.New("redirect_to attribute not found in response")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	} else {
		err = errors.New("response is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return
}

func (hydraConfig HydraConfig) AcceptConsentRequest(ctx context.Context, identityHolder map[string]interface{}, consentChallenge string, loginCookies []*http.Cookie) (tokens LoginSuccess, err error) {
	logs.WithContext(ctx).Debug("acceptConsentRequest - Start")
	hydraCLR := hydraAcceptConsentRequest{}
	hydraCLR.Remember = true
	hydraCLR.RememberFor = 0
	hydraCLR.Session.IdToken = identityHolder
	for _, v := range hydraConfig.HydraClients {
		hydraCLR.GrantScope = strings.Split(v.Scope, " ")
		break
	}
	handledAt := time.Now()
	hydraCLR.HandledAt = handledAt
	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Add("content-type", "application/json")

	paramsMap := make(map[string]string)
	paramsMap["consent_challenge"] = consentChallenge

	postUrl := fmt.Sprint(hydraConfig.GetAminUrl(), "/admin/oauth2/auth/requests/consent/accept")
	resp, _, respCookies, _, err := utils.CallHttp(ctx, http.MethodPut, postUrl, headers, dummyMap, loginCookies, paramsMap, hydraCLR)
	respCookies = append(respCookies, loginCookies...)
	if err != nil {
		return LoginSuccess{}, err
	}
	code := ""
	if respMap, ok := resp.(map[string]interface{}); ok {
		if redirectUrl, ok1 := respMap["redirect_to"]; ok1 {
			_, respHeaders2, respCookies2, statusCode2, err2 := utils.CallHttp(ctx, http.MethodGet, redirectUrl.(string), headers, dummyMap, respCookies, dummyMap, dummyMap)
			respCookies = append(respCookies, respCookies2...)
			if err2 != nil {
				return LoginSuccess{}, err
			}
			if statusCode2 >= 300 && statusCode2 < 400 {
				redirectLocation := respHeaders2["Location"][0]
				params := strings.Split(redirectLocation, "?")[1]
				codeParam := strings.Split(params, "&")[0]
				code = strings.Split(codeParam, "=")[1]

				hydraClientId := ""
				for _, v := range hydraConfig.HydraClients {
					hydraClientId = v.ClientId
					break
				}
				outhConfig, ocErr := hydraConfig.GetOauthConfig(ctx, hydraClientId)
				if ocErr != nil {
					err = ocErr
					return
				}

				token, tokenErr := outhConfig.Exchange(ctx, code)
				if tokenErr != nil {
					err = tokenErr
					logs.WithContext(ctx).Error(fmt.Sprint("unable to exchange code for token: %s\n", err.Error()))
					return
				}
				idt := token.Extra("id_token")
				logs.WithContext(ctx).Info(idt.(string))
				loginSuccess := LoginSuccess{}
				loginSuccess.AccessToken = token.AccessToken
				loginSuccess.RefreshToken = token.RefreshToken
				loginSuccess.IdToken = idt.(string)
				loginSuccess.Expiry = token.Expiry
				loginSuccess.ExpiresIn = token.Extra("expires_in").(float64)
				return loginSuccess, nil
			}
		} else {
			err = errors.New("redirect_to attribute not found in response")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	} else {
		err = errors.New("response is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return
}

func (hydraConfig HydraConfig) revokeToken(ctx context.Context, token string) (resStatusCode int, err error) {
	logs.WithContext(ctx).Debug("revokeToken - Start")
	hydraRevokeToken := make(map[string]string)
	hydraRevokeToken["token"] = token
	for i, _ := range hydraConfig.HydraClients {
		hydraRevokeToken["client_id"] = hydraConfig.HydraClients[i].ClientId
		break
	}

	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Add("content-type", "application/x-www-form-urlencoded")

	postUrl := fmt.Sprint(hydraConfig.GetPublicUrl(), "/oauth2/revoke")
	_, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodPost, postUrl, headers, hydraRevokeToken, nil, dummyMap, dummyMap)
	if err != nil {
		return statusCode, err
	}
	return statusCode, nil
}
