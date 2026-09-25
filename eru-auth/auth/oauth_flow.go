package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
)

// LoginRequestInfo is what the authorization server knows about a login step before anyone has
// been authenticated. Skip is set when the server already has a session it is willing to reuse,
// in which case no credentials are collected and Subject is already filled in.
type LoginRequestInfo struct {
	Challenge      string   `json:"challenge"`
	Skip           bool     `json:"skip"`
	Subject        string   `json:"subject"`
	ClientId       string   `json:"client_id"`
	ClientName     string   `json:"client_name"`
	RequestedScope []string `json:"requested_scope"`
	RequestUrl     string   `json:"request_url"`
}

// ConsentRequestInfo is what the consent screen renders: who is asking, and for what.
type ConsentRequestInfo struct {
	Challenge         string   `json:"challenge"`
	Skip              bool     `json:"skip"`
	Subject           string   `json:"subject"`
	ClientId          string   `json:"client_id"`
	ClientName        string   `json:"client_name"`
	ClientUri         string   `json:"client_uri"`
	LogoUri           string   `json:"logo_uri"`
	PolicyUri         string   `json:"policy_uri"`
	TosUri            string   `json:"tos_uri"`
	RequestedScope    []string `json:"requested_scope"`
	RequestedAudience []string `json:"requested_audience"`
}

// AuthorizationFlowI is the browser facing half of the authorization code grant: the steps that
// have to pause for a human. It is the second seam alongside ClientRegistryI - hydra drives it
// today, and an eru-auth implementation replaces it without the handlers changing.
//
// Every Accept and Reject returns the url the browser must be sent to. That is the whole point of
// this interface: the existing AcceptLoginRequest and AcceptConsentRequest follow those redirects
// server side to mint tokens headlessly, which is right for the password api and wrong here.
type AuthorizationFlowI interface {
	LoginRequest(ctx context.Context, challenge string) (LoginRequestInfo, error)
	AcceptLogin(ctx context.Context, challenge string, subject string, remember bool) (redirectTo string, err error)
	RejectLogin(ctx context.Context, challenge string, errorCode string, errorDescription string) (redirectTo string, err error)
	ConsentRequest(ctx context.Context, challenge string) (ConsentRequestInfo, error)
	AcceptConsent(ctx context.Context, challenge string, grantScope []string, grantAudience []string, idTokenClaims map[string]interface{}, accessTokenClaims map[string]interface{}, remember bool) (redirectTo string, err error)
	RejectConsent(ctx context.Context, challenge string, errorCode string, errorDescription string) (redirectTo string, err error)
}

// AuthorizationFlow resolves the backend driving the interactive grant.
func (auth *Auth) AuthorizationFlow(ctx context.Context, projectId string) (AuthorizationFlowI, error) {
	backend := auth.OAuthServerConfig.Backend
	if backend == "" {
		backend = OAuthBackendHydra
	}
	switch strings.ToUpper(backend) {
	case OAuthBackendHydra:
		return HydraAuthorizationFlow{Hydra: auth.Hydra}, nil
	case OAuthBackendEru:
		registry, registryErr := auth.ClientRegistry(ctx, projectId)
		if registryErr != nil {
			return nil, registryErr
		}
		return EruAuthorizationFlow{
			AuthDb:    auth.AuthDb,
			ProjectId: projectId,
			AuthName:  auth.AuthName,
			Issuer:    auth.OAuthIssuer(ctx),
			Registry:  registry,
			Policy:    auth.OAuthServerConfig.ClientPolicy,
		}, nil
	default:
		err := errors.New(fmt.Sprint("unknown oauth server backend : ", backend))
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

// GrantableScope narrows what a consent step may grant to what the project allows, so a client
// cannot widen its own grant by asking for more than it registered for.
func (oAuthServerConfig OAuthServerConfig) GrantableScope(requestedScope []string) []string {
	allowed := oAuthServerConfig.ClientPolicy.Scopes()
	var grantScope []string
	for _, scope := range requestedScope {
		if contains(allowed, scope) {
			grantScope = append(grantScope, scope)
		}
	}
	return grantScope
}

// HydraAuthorizationFlow drives hydra's login and consent requests over its admin api.
type HydraAuthorizationFlow struct {
	Hydra HydraConfig
}

const (
	hydraLoginRequestPath   = "/admin/oauth2/auth/requests/login"
	hydraConsentRequestPath = "/admin/oauth2/auth/requests/consent"
)

func (flow HydraAuthorizationFlow) LoginRequest(ctx context.Context, challenge string) (LoginRequestInfo, error) {
	logs.WithContext(ctx).Debug("LoginRequest - Start")
	respMap, err := flow.fetchRequest(ctx, hydraLoginRequestPath, "login_challenge", challenge)
	if err != nil {
		return LoginRequestInfo{}, err
	}
	loginRequest := LoginRequestInfo{
		Challenge:      challenge,
		Skip:           boolValue(respMap["skip"]),
		Subject:        stringValue(respMap["subject"]),
		RequestUrl:     stringValue(respMap["request_url"]),
		RequestedScope: stringValues(respMap["requested_scope"]),
	}
	loginRequest.ClientId, loginRequest.ClientName, _, _, _, _ = hydraClientInfo(respMap["client"])
	return loginRequest, nil
}

func (flow HydraAuthorizationFlow) AcceptLogin(ctx context.Context, challenge string, subject string, remember bool) (string, error) {
	logs.WithContext(ctx).Debug("AcceptLogin - Start")
	if subject == "" {
		err := errors.New("cannot accept a login request without a subject")
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	body := map[string]interface{}{
		"subject":      subject,
		"remember":     remember,
		"remember_for": 0,
	}
	return flow.putRequest(ctx, fmt.Sprint(hydraLoginRequestPath, "/accept"), "login_challenge", challenge, body)
}

func (flow HydraAuthorizationFlow) RejectLogin(ctx context.Context, challenge string, errorCode string, errorDescription string) (string, error) {
	logs.WithContext(ctx).Debug("RejectLogin - Start")
	body := map[string]interface{}{"error": errorCode, "error_description": errorDescription}
	return flow.putRequest(ctx, fmt.Sprint(hydraLoginRequestPath, "/reject"), "login_challenge", challenge, body)
}

func (flow HydraAuthorizationFlow) ConsentRequest(ctx context.Context, challenge string) (ConsentRequestInfo, error) {
	logs.WithContext(ctx).Debug("ConsentRequest - Start")
	respMap, err := flow.fetchRequest(ctx, hydraConsentRequestPath, "consent_challenge", challenge)
	if err != nil {
		return ConsentRequestInfo{}, err
	}
	consentRequest := ConsentRequestInfo{
		Challenge:         challenge,
		Skip:              boolValue(respMap["skip"]),
		Subject:           stringValue(respMap["subject"]),
		RequestedScope:    stringValues(respMap["requested_scope"]),
		RequestedAudience: stringValues(respMap["requested_access_token_audience"]),
	}
	consentRequest.ClientId, consentRequest.ClientName, consentRequest.ClientUri, consentRequest.LogoUri, consentRequest.PolicyUri, consentRequest.TosUri = hydraClientInfo(respMap["client"])
	return consentRequest, nil
}

func (flow HydraAuthorizationFlow) AcceptConsent(ctx context.Context, challenge string, grantScope []string, grantAudience []string, idTokenClaims map[string]interface{}, accessTokenClaims map[string]interface{}, remember bool) (string, error) {
	logs.WithContext(ctx).Debug("AcceptConsent - Start")
	session := map[string]interface{}{}
	if len(idTokenClaims) > 0 {
		session["id_token"] = idTokenClaims
	}
	if len(accessTokenClaims) > 0 {
		session["access_token"] = accessTokenClaims
	}
	body := map[string]interface{}{
		"grant_scope":                 grantScope,
		"grant_access_token_audience": grantAudience,
		"remember":                    remember,
		"remember_for":                0,
		"session":                     session,
	}
	return flow.putRequest(ctx, fmt.Sprint(hydraConsentRequestPath, "/accept"), "consent_challenge", challenge, body)
}

func (flow HydraAuthorizationFlow) RejectConsent(ctx context.Context, challenge string, errorCode string, errorDescription string) (string, error) {
	logs.WithContext(ctx).Debug("RejectConsent - Start")
	body := map[string]interface{}{"error": errorCode, "error_description": errorDescription}
	return flow.putRequest(ctx, fmt.Sprint(hydraConsentRequestPath, "/reject"), "consent_challenge", challenge, body)
}

func (flow HydraAuthorizationFlow) fetchRequest(ctx context.Context, path string, challengeKey string, challenge string) (map[string]interface{}, error) {
	if challenge == "" {
		err := errors.New(fmt.Sprint(challengeKey, " is missing"))
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	paramsMap := map[string]string{challengeKey: challenge}
	resp, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, fmt.Sprint(flow.Hydra.GetAminUrl(), path), nil, nil, nil, paramsMap, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	respMap, respMapOk := resp.(map[string]interface{})
	if !respMapOk {
		err = errors.New("authorization request response is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	if err = checkResponseError(ctx, resp); err != nil {
		return nil, err
	}
	return respMap, nil
}

// putRequest returns hydra's redirect_to rather than following it, leaving the browser to make the
// next hop.
func (flow HydraAuthorizationFlow) putRequest(ctx context.Context, path string, challengeKey string, challenge string, body map[string]interface{}) (string, error) {
	if challenge == "" {
		err := errors.New(fmt.Sprint(challengeKey, " is missing"))
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	paramsMap := map[string]string{challengeKey: challenge}
	resp, _, _, _, err := utils.CallHttp(ctx, http.MethodPut, fmt.Sprint(flow.Hydra.GetAminUrl(), path), headers, map[string]string{}, nil, paramsMap, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	if err = checkResponseError(ctx, resp); err != nil {
		return "", err
	}
	respMap, respMapOk := resp.(map[string]interface{})
	if !respMapOk {
		err = errors.New("authorization request response is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	redirectTo := stringValue(respMap["redirect_to"])
	if redirectTo == "" {
		err = errors.New("authorization server did not return a redirect")
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	return redirectTo, nil
}

func hydraClientInfo(value interface{}) (clientId string, clientName string, clientUri string, logoUri string, policyUri string, tosUri string) {
	clientMap, clientMapOk := value.(map[string]interface{})
	if !clientMapOk {
		return
	}
	return stringValue(clientMap["client_id"]), stringValue(clientMap["client_name"]),
		stringValue(clientMap["client_uri"]), stringValue(clientMap["logo_uri"]),
		stringValue(clientMap["policy_uri"]), stringValue(clientMap["tos_uri"])
}

func stringValue(value interface{}) string {
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}

func boolValue(value interface{}) bool {
	if v, ok := value.(bool); ok {
		return v
	}
	return false
}

func stringValues(value interface{}) []string {
	values, _ := stringSlice(value)
	return values
}
