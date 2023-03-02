package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-crypto/jwt"
	utils "github.com/eru-tech/eru/eru-utils"
	"golang.org/x/oauth2"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type LoginSuccess struct {
	AccessToken  string
	RefreshToken string
	IdToken      string
	Expiry       time.Time
	ExpiresIn    float64
}

type HydraConfig struct {
	PublicHost   string
	PublicPort   string
	PublicScheme string
	AdminHost    string
	AdminPort    string
	AdminScheme  string
	AuthURL      string
	TokenURL     string
	HydraClients map[string]HydraClient
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
	ClientSecret            string   `json:"client_secret"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
}

func (kratosHydraAuth *KratosHydraAuth) getLoginChallenge() (loginChallenge string, cookies []*http.Cookie, err error) {
	log.Println("inside getLoginChallenge")
	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		log.Println("generate state failed: %v", err)
		return
	}
	state := base64.StdEncoding.EncodeToString(b)

	hydraClientId := ""
	for _, v := range kratosHydraAuth.Hydra.HydraClients {
		hydraClientId = v.ClientId
		break
	}
	outhConfig, ocErr := kratosHydraAuth.Hydra.getOauthConfig(hydraClientId)
	if ocErr != nil {
		log.Println("generate state failed: %v", err)
		err = ocErr
		return
	}
	log.Println(outhConfig)
	redirectTo := outhConfig.AuthCodeURL(state)

	log.Println("redirect to hydra, url: %s", redirectTo)
	res, headers, respCookies, statusCode, err := utils.CallHttp(http.MethodGet, redirectTo, nil, nil, nil, nil, nil)
	log.Println(res)
	log.Println(headers)
	cookies = respCookies
	log.Println(cookies)
	log.Println(statusCode)
	log.Println(err)
	if statusCode >= 300 && statusCode < 400 {
		redirectLocation := headers["Location"][0]
		log.Println(redirectLocation)
		params := strings.Split(redirectLocation, "?")[1]
		log.Println(params)
		loginChallenge = strings.Split(params, "=")[1]
	}
	return
}

func (kratosHydraAuth *KratosHydraAuth) GetUserInfo(access_token string) (identity Identity, err error) {
	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Add("Authorization", fmt.Sprint("Bearer ", access_token))
	res, _, _, _, err := utils.CallHttp("POST", fmt.Sprint(kratosHydraAuth.Hydra.getPublicUrl(), "/userinfo"), headers, dummyMap, nil, dummyMap, dummyMap)
	if err != nil {
		log.Print("error in http.Get GetUserInfo")
		log.Print(err)
		return Identity{}, err
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
		err = errors.New("User Info response not in expected format")
		return Identity{}, err
	} else {
		if identityObj, isIdOk := userInfo["identity"]; !isIdOk {
			err = errors.New("Identity attribute missing in User Info")
			return Identity{}, err
		} else {
			log.Println(identityObj)
			identityObjJson, iErr := json.Marshal(identityObj)
			if iErr != nil {
				log.Println(iErr)
				err = errors.New("Json marshal error for identityObj")
				return Identity{}, err
			}
			iiErr := json.Unmarshal(identityObjJson, &identity)
			if iiErr != nil {
				log.Println(iiErr)
				err = errors.New("Json unmarshal error for identityObj")
				return Identity{}, err
			}
			return
		}
	}
}

func (kratosHydraAuth *KratosHydraAuth) FetchTokens(refresh_token string) (res interface{}, err error) {
	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Add("content-type", "application/x-www-form-urlencoded")
	formData := make(map[string]string)
	formData["refresh_token"] = refresh_token
	formData["grant_type"] = "refresh_token"
	for _, v := range kratosHydraAuth.Hydra.HydraClients {
		formData["client_id"] = v.ClientId
		break
	}
	res, _, _, _, err = utils.CallHttp("POST", fmt.Sprint(kratosHydraAuth.Hydra.getPublicUrl(), "/oauth2/token"), headers, formData, nil, dummyMap, dummyMap)
	if err != nil {
		log.Print("error in http.Get FetchTokens")
		log.Print(err)
		return nil, err
	}
	log.Println(res)
	loginSuccess := LoginSuccess{}
	ok := false
	if resMap, resOk := res.(map[string]interface{}); !resOk {
		err = errors.New("Response is not map[string]string")
		log.Println(err)
		return nil, err
	} else {
		if loginSuccess.AccessToken, ok = resMap["access_token"].(string); !ok {
			err = errors.New("access_token attribute missing from response")
			log.Print(err)
			return nil, err
		}
		if loginSuccess.IdToken, ok = resMap["id_token"].(string); !ok {
			err = errors.New("id_token attribute missing from response")
			log.Print(err)
			return nil, err
		}
		if loginSuccess.RefreshToken, ok = resMap["refresh_token"].(string); !ok {
			err = errors.New("refresh_token attribute missing from response")
			log.Print(err)
			return nil, err
		}
		if loginSuccess.ExpiresIn, ok = resMap["expires_in"].(float64); !ok {
			err = errors.New("expires_in attribute missing from response")
			log.Print(err)
			return nil, err
		}
	}
	log.Println("before returning loginSuccess")
	log.Println(loginSuccess)
	return loginSuccess, nil
}

func (kratosHydraAuth *KratosHydraAuth) VerifyToken(tokenType string, token string) (res interface{}, err error) {
	switch tokenType {
	case "id":
		log.Println("calling VerifyIdToken")
		return kratosHydraAuth.Hydra.verifyIdToken(token)
	case "access":
		log.Println("calling VerifyAccessToken")
		return kratosHydraAuth.Hydra.verifyAccessToken(token)
	case "refresh":
		log.Println("calling VerifyRefreshToken")
		return kratosHydraAuth.Hydra.verifyRefreshToken(token)
	default:
		//do nothing
	}
	return nil, errors.New(fmt.Sprint("tokenType Mismatch : ", tokenType))
}

func (hydraConfig HydraConfig) verifyIdToken(token string) (res interface{}, err error) {
	jwkUrl := fmt.Sprint(hydraConfig.getPublicUrl(), "/.well-known/jwks.json")
	claims, err := jwt.DecryptTokenJWK(token, jwkUrl)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (hydraConfig HydraConfig) verifyAccessToken(token string) (res interface{}, err error) {
	dummyMap := make(map[string]string)
	formData := make(map[string]string)
	formData["token"] = token
	headers := http.Header{}
	headers.Add("content-type", "application/x-www-form-urlencoded")
	res, _, _, _, err = utils.CallHttp("POST", fmt.Sprint(hydraConfig.getAminUrl(), "/oauth2/introspect"), headers, formData, nil, dummyMap, dummyMap)
	if err != nil {
		log.Print("error in http.Get verifyAccessToken")
		log.Print(err)
		return nil, err
	}
	log.Println(res)

	if verifiedToken, ok := res.(map[string]interface{}); !ok {
		err = errors.New("Verified token response not in expected format")
		return nil, err
	} else {
		if isActive, isActiveOk := verifiedToken["active"]; !isActiveOk {
			err = errors.New("Verified token response not in expected format")
			return nil, err
		} else {
			if isActive.(bool) {
				return verifiedToken, nil
			} else {
				err = errors.New("Invalid Token")
				return nil, err
			}
		}
	}
}

func (hydraConfig HydraConfig) verifyRefreshToken(token string) (res interface{}, err error) {
	return hydraConfig.verifyAccessToken(token)
}

func (hydraConfig HydraConfig) getOauthConfig(clientId string) (oauth2Config *oauth2.Config, err error) {
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
		return nil, errors.New(fmt.Sprint(clientId, "not found"))
	}
}
func (hydraConfig HydraConfig) RemoveHydraClient(clientId string) (err error) {
	log.Println("inside RemoveHydraClient")
	if _, ok := hydraConfig.HydraClients[clientId]; ok {
		_, _, _, err = hydraConfig.deleteHydraClient(clientId)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		err = errors.New(fmt.Sprint("hydra client not found : ", clientId))
		return
	}
	return nil
}
func (hydraConfig HydraConfig) SaveHydraClient(hydraClient HydraClient) (err error) {
	_, _, _, err = hydraConfig.getHydraClient(hydraClient.ClientId)
	if err != nil {
		log.Println("calling create api for hydraclient : ", hydraClient.ClientId)
		_, _, _, err = hydraConfig.createHydraClient(hydraClient)
	} else {
		log.Println("calling update api for hydraclient : ", hydraClient.ClientId)
		_, _, _, err = hydraConfig.updateHydraClient(hydraClient.ClientId, hydraClient)
	}
	return
}

func (hydraConfig HydraConfig) getHydraClient(clientId string) (resp interface{}, headers map[string][]string, statusCode int, err error) {
	dummyMap := make(map[string]string)
	resp, headers, _, statusCode, err = utils.CallHttp("GET", fmt.Sprint(hydraConfig.getAminUrl(), "/clients/", clientId), nil, nil, nil, dummyMap, dummyMap)
	if err != nil {
		log.Print("error in http.Get getHydraClient")
		log.Print(err)
		return nil, nil, 0, err
	}
	return resp, headers, statusCode, checkResponseError(resp)
}
func (hydraConfig HydraConfig) createHydraClient(hydraClient HydraClient) (resp interface{}, headers http.Header, statusCode int, err error) {
	dummyMap := make(map[string]string)
	headers = http.Header{}
	headers.Add("Content-Type", "application/json")
	resp, headers, _, statusCode, err = utils.CallHttp("POST", fmt.Sprint(hydraConfig.getAminUrl(), "/clients"), headers, dummyMap, nil, dummyMap, hydraClient)
	if err != nil {
		log.Print("error in http.Post createHydraClient")
		log.Print(err)
		return nil, nil, 0, err
	}
	return resp, headers, statusCode, checkResponseError(resp)
}
func (hydraConfig HydraConfig) updateHydraClient(clientId string, hydraClient HydraClient) (resp interface{}, headers http.Header, statusCode int, err error) {
	dummyMap := make(map[string]string)
	headers = http.Header{}
	headers.Add("Content-Type", "application/json")
	resp, headers, _, statusCode, err = utils.CallHttp("PUT", fmt.Sprint(hydraConfig.getAminUrl(), "/clients/", clientId), headers, dummyMap, nil, dummyMap, hydraClient)
	if err != nil {
		log.Print("error in http.Post updateHydraClient")
		log.Print(err)
		return nil, nil, 0, err
	}
	return resp, headers, statusCode, checkResponseError(resp)
}
func (hydraConfig HydraConfig) deleteHydraClient(clientId string) (resp interface{}, headers http.Header, statusCode int, err error) {
	log.Println("inside deleteHydraClient")
	dummyMap := make(map[string]string)
	headers = http.Header{}
	headers.Add("Content-Type", "application/json")
	resp, headers, _, statusCode, err = utils.CallHttp("DELETE", fmt.Sprint(hydraConfig.getAminUrl(), "/clients/", clientId), headers, dummyMap, nil, dummyMap, dummyMap)
	if err != nil {
		log.Print("error in http.Post deleteHydraClient")
		log.Print(err)
		return nil, nil, 0, err
	}
	if resp != nil {
		return resp, headers, statusCode, checkResponseError(resp)
	}
	return resp, headers, statusCode, nil
}
func checkResponseError(resp interface{}) (err error) {
	if respMap, ok := resp.(map[string]interface{}); !ok {
		err = errors.New("resp.(map[string]interface{}) conversion failed")
		log.Println(err)
		return
	} else {
		if errResp, ok := respMap["error"]; ok {
			log.Println(errResp)
			err = errors.New(errResp.(string))
			return
		}
	}
	return
}

func (hydraConfig HydraConfig) getPublicUrl() (url string) {
	port := ""
	if hydraConfig.PublicPort != "" {
		port = fmt.Sprint(":", hydraConfig.PublicPort)
	}
	return fmt.Sprint(hydraConfig.PublicScheme, "://", hydraConfig.PublicHost, port)
}
func (hydraConfig HydraConfig) getAminUrl() (url string) {
	port := ""
	if hydraConfig.AdminPort != "" {
		port = fmt.Sprint(":", hydraConfig.AdminPort)
	}
	return fmt.Sprint(hydraConfig.AdminScheme, "://", hydraConfig.AdminHost, port)
}
func (hydraConfig HydraConfig) acceptLoginRequest(subject string, loginChallenge string, cookies []*http.Cookie) (consentChallenge string, respCookies []*http.Cookie, err error) {
	hydraALR := hydraAcceptLoginRequest{}
	hydraALR.Remember = true
	hydraALR.RememberFor = 0
	hydraALR.Subject = subject
	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Add("content-type", "application/json")
	paramsMap := make(map[string]string)
	paramsMap["login_challenge"] = loginChallenge
	postUrl := fmt.Sprint(hydraConfig.getAminUrl(), "/oauth2/auth/requests/login/accept")
	resp, respHeaders, respCookies, statusCode, err := utils.CallHttp(http.MethodPut, postUrl, headers, dummyMap, cookies, paramsMap, hydraALR)
	log.Println(respHeaders)
	log.Println(statusCode)
	log.Println(respCookies)
	respCookies = append(respCookies, cookies...)
	if err != nil {
		log.Println(err)
		return "", nil, err
	}
	if respMap, ok := resp.(map[string]interface{}); ok {
		if redirectUrl, ok1 := respMap["redirect_to"]; ok1 {
			log.Print(redirectUrl)
			resp2, respHeaders2, respCookies2, statusCode2, err2 := utils.CallHttp(http.MethodGet, redirectUrl.(string), headers, dummyMap, respCookies, dummyMap, dummyMap)
			log.Println(resp2)
			log.Println(respHeaders2)
			log.Println(statusCode2)
			log.Println(respCookies2)
			respCookies = append(respCookies, respCookies2...)
			log.Println(err2)
			if err != nil {
				return "", nil, err
			}
			if statusCode2 >= 300 && statusCode2 < 400 {
				redirectLocation := respHeaders2["Location"][0]
				log.Println(redirectLocation)
				params := strings.Split(redirectLocation, "?")[1]
				log.Println(params)
				consentChallenge = strings.Split(params, "=")[1]
			}
			log.Println("consentChallenge = ", consentChallenge)
			log.Println("cookies = ", cookies)
		} else {
			err = errors.New("redirect_to attribute not found in response")
			log.Println(err)
			return
		}
	} else {
		err = errors.New("response is not a map")
		log.Println(err)
		return
	}
	return
}

func (hydraConfig HydraConfig) acceptConsentRequest(identityHolder map[string]interface{}, consentChallenge string, loginCookies []*http.Cookie) (tokens LoginSuccess, err error) {

	hydraCLR := hydraAcceptConsentRequest{}
	hydraCLR.Remember = true
	hydraCLR.RememberFor = 0
	hydraCLR.Session.IdToken = identityHolder
	for _, v := range hydraConfig.HydraClients {
		log.Println("v.GrantTypes")
		log.Println(v.Scope)
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
	postUrl := fmt.Sprint(hydraConfig.getAminUrl(), "/oauth2/auth/requests/consent/accept")
	resp, respHeaders, respCookies, statusCode, err := utils.CallHttp(http.MethodPut, postUrl, headers, dummyMap, loginCookies, paramsMap, hydraCLR)
	log.Println("(respHeaders from oauth2/auth/requests/consent/accept")
	log.Println(respHeaders)
	log.Println(statusCode)
	respCookies = append(respCookies, loginCookies...)
	log.Println(respHeaders)
	log.Println("respCookies ===============", respCookies)
	//headerMap["cookie"] = strings.Join(respCookies, " ; ")
	if err != nil {
		log.Println(err)
		return LoginSuccess{}, err
	}
	code := ""
	if respMap, ok := resp.(map[string]interface{}); ok {
		if redirectUrl, ok1 := respMap["redirect_to"]; ok1 {
			log.Print(redirectUrl)
			resp2, respHeaders2, respCookies2, statusCode2, err2 := utils.CallHttp(http.MethodGet, redirectUrl.(string), headers, dummyMap, respCookies, dummyMap, dummyMap)
			log.Println(resp2)
			log.Println(respHeaders2)
			log.Println(statusCode2)
			log.Println(err2)
			respCookies = append(respCookies, respCookies2...)
			if err != nil {
				return LoginSuccess{}, err
			}
			if statusCode2 >= 300 && statusCode2 < 400 {
				redirectLocation := respHeaders2["Location"][0]
				log.Println(redirectLocation)
				params := strings.Split(redirectLocation, "?")[1]
				log.Println(params)
				codeParam := strings.Split(params, "&")[0]
				code = strings.Split(codeParam, "=")[1]
				log.Println("respCookies :::::::::::::::::::::::: ", respCookies)
				//todo context from handler
				ctx := context.Background()

				hydraClientId := ""
				for _, v := range hydraConfig.HydraClients {
					hydraClientId = v.ClientId
					break
				}

				outhConfig, ocErr := hydraConfig.getOauthConfig(hydraClientId)
				if ocErr != nil {
					log.Println("generate state failed: %v", err)
					err = ocErr
					return
				}

				log.Println("code = ", code)
				log.Println("respCookies = ", respCookies)

				token, tokenErr := outhConfig.Exchange(ctx, code)
				if tokenErr != nil {
					log.Printf("unable to exchange code for token: %s\n", err)
					err = tokenErr
					return
				}
				idt := token.Extra("id_token")
				loginSuccess := LoginSuccess{}
				loginSuccess.AccessToken = token.AccessToken
				loginSuccess.RefreshToken = token.RefreshToken
				loginSuccess.IdToken = idt.(string)
				loginSuccess.Expiry = token.Expiry
				loginSuccess.ExpiresIn = token.Extra("expires_in").(float64)
				//log.Println(loginSuccess)
				log.Println("respCookies before returning")
				log.Println(respCookies)
				return loginSuccess, nil
			}
		} else {
			err = errors.New("redirect_to attribute not found in response")
			log.Println(err)
			return
		}
	} else {
		err = errors.New("response is not a map")
		log.Println(err)
		return
	}
	return
}

func (hydraConfig HydraConfig) revokeToken(token string) (resStatusCode int, err error) {
	hydraRevokeToken := make(map[string]string)
	hydraRevokeToken["token"] = token
	for i, _ := range hydraConfig.HydraClients {
		//picking up first client as usually there will be only 1 client defined
		hydraRevokeToken["client_id"] = hydraConfig.HydraClients[i].ClientId
		break
	}

	dummyMap := make(map[string]string)
	headers := http.Header{}
	headers.Add("content-type", "application/x-www-form-urlencoded")

	postUrl := fmt.Sprint(hydraConfig.getPublicUrl(), "/oauth2/revoke")
	resp, _, _, statusCode, err := utils.CallHttp(http.MethodPost, postUrl, headers, hydraRevokeToken, nil, dummyMap, dummyMap)
	log.Println("(resp from oauth2/revoke")
	log.Print(resp)
	log.Println(statusCode)
	if err != nil {
		log.Println(err)
		return statusCode, err
	}
	return statusCode, nil
}
