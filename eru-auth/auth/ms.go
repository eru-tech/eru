package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-crypto/jwt"
	erupkce "github.com/eru-tech/eru/eru-crypto/pkce"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"reflect"
)

type MsAuth struct {
	Auth
	MsConfig MsConfig `json:"msConfig"`
}

type MsConfig struct {
	ClientId     string `json:"clientId" eru:"required"`
	ClientSecret string `json:"clientSecret" eru:"required"`
	RedirectURI  string `json:"redirectUri" eru:"required"`
	Scope        string `json:"scope" eru:"required"`
	SsoBaseUrl   string `json:"ssoBaseUrl" eru:"required"`
	TokenUrl     string `json:"tokenUrl" eru:"required"`
	JwkUrl       string `json:"jwkUrl" eru:"required"`
}

type MsParams struct {
	ClientId            string `json:"client_id"`
	Scope               string `json:"scope"`
	RedirectURI         string `json:"redirect_uri"`
	ClientRequestId     string `json:"client-request-id"`
	ResponseMode        string `json:"response_mode"`
	ResponseType        string `json:"response_type"`
	CodeChallenge       string `json:"code_challenge"`
	CodeVerifier        string
	CodeChallengeMethod string `json:"code_challenge_method"`
	Nonce               string `json:"nonce"`
	State               string `json:"state"`
	Url                 string
}

func (msAuth *MsAuth) PerformPreSaveTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreSaveTask - Start")
	// Do Nothing
	return
}

func (msAuth *MsAuth) PerformPreDeleteTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreSaveTask - Start")
	// Do Nothing
	return
}

func (msAuth *MsAuth) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &msAuth)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (msAuth *MsAuth) GetUrl(ctx context.Context, state string) (urlStr string, msParams MsParams, err error) {
	codeVerifier, codeChallenge, err := erupkce.NewPKCE(ctx)
	logs.WithContext(ctx).Info(codeVerifier)
	logs.WithContext(ctx).Info(codeChallenge)
	msParams.ClientId = msAuth.MsConfig.ClientId
	msParams.Scope = msAuth.MsConfig.Scope
	msParams.RedirectURI = msAuth.MsConfig.RedirectURI
	msParams.ResponseType = "code"
	msParams.ResponseMode = "fragment"
	msParams.ClientRequestId = uuid.New().String()
	msParams.CodeChallenge = codeChallenge
	msParams.CodeVerifier = codeVerifier
	msParams.CodeChallengeMethod = "S256"
	msParams.Nonce = uuid.New().String()
	msParams.State = state
	params := url.Values{}
	f := reflect.ValueOf(msParams)
	for i := 0; i < f.NumField(); i++ {
		tags := f.Type().Field(i).Tag.Get("json")
		if tags != "" {
			params.Add(tags, f.Field(i).String())
		}
	}
	urlStr = fmt.Sprint(msAuth.MsConfig.SsoBaseUrl, "?", params.Encode())
	msParams.Url = urlStr
	return
}

func (msAuth *MsAuth) Login(ctx context.Context, loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {
	logs.WithContext(ctx).Debug("Login - Start")

	headers := http.Header{}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")

	msLoginFormBody := make(map[string]string)
	msLoginFormBody["client_id"] = msAuth.MsConfig.ClientId
	msLoginFormBody["client_secret"] = msAuth.MsConfig.ClientSecret
	msLoginFormBody["redirect_uri"] = msAuth.MsConfig.RedirectURI
	msLoginFormBody["grant_type"] = "authorization_code"
	msLoginFormBody["scope"] = msAuth.MsConfig.Scope
	msLoginFormBody["code"] = loginPostBody.IdpCode
	msLoginFormBody["code_verifier"] = loginPostBody.CodeVerifier

	/*
		loginRes, _, _, _, loginErr := utils.CallHttp(ctx, http.MethodPost, msAuth.MsConfig.TokenUrl, headers, msLoginFormBody, nil, nil, nil)
		if loginErr != nil {
			logs.WithContext(ctx).Error(fmt.Sprint(map[string]interface{}{"request_id": loginPostBody.IdpRequestId, "error": fmt.Sprint(loginErr)}))
			return Identity{}, LoginSuccess{}, errors.New("error from identity provider")
		}
	*/
	idToken := "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsImtpZCI6Ii1LSTNROW5OUjdiUm9meG1lWm9YcWJIWkdldyJ9.eyJhdWQiOiI5OTFlMDZjMy0zYmUwLTQ3ZTAtOGVjYS02MTBhMjk4NzkwMmEiLCJpc3MiOiJodHRwczovL2xvZ2luLm1pY3Jvc29mdG9ubGluZS5jb20vZjRjNDgxMmYtMDA1NC00ZTdhLTkyNzEtY2QwNzE4NjVkOWFlL3YyLjAiLCJpYXQiOjE2OTQ4ODAyNDksIm5iZiI6MTY5NDg4MDI0OSwiZXhwIjoxNjk0ODg0MTQ5LCJhaW8iOiJBVFFBeS84VUFBQUFsaWovTGhZTnRVZnRnVXpGdEVOWWMwMEtIK3c4RmcrSWF4bXJJdU1BY0V0Z1A2SUtzL2FZbVdzMXg5V2VERkg5IiwibmFtZSI6IkFsdGFmICBCYXJhZGlhIiwibm9uY2UiOiI0NDljZDc3OC00MTQxLTQ4MjEtOWJkYS0zMWU0N2Y2MmMyNmMiLCJvaWQiOiJkMjFjZjgxMy1mMzA0LTQyODEtOGEwMC1kNzExZDkyYmI4YTkiLCJwcmVmZXJyZWRfdXNlcm5hbWUiOiJhbHRhZi5iYXJhZGlhQGxhbXJvbmFuYWx5c3RzLmNvbSIsInJoIjoiMC5BWEVBTDRIRTlGUUFlazZTY2MwSEdHWFpyc01HSHBuZ08tQkhqc3BoQ2ltSGtDcUhBRXMuIiwic3ViIjoiX1R1akJLdXJNVU1VVzlPWWFsY1oyTk9aRlBYeFJiMUZDZTRPdGh4T25UZyIsInRpZCI6ImY0YzQ4MTJmLTAwNTQtNGU3YS05MjcxLWNkMDcxODY1ZDlhZSIsInV0aSI6IkMwVWdMa2JDbzBXY294UFVUd1FDQUEiLCJ2ZXIiOiIyLjAifQ.cWfmi7uFypdfXIrxX5xgZ7E1DemERR6b7Ux25L2xcp-7H2aLkZNqNldq9a5H66ldiCRRxmb17r9ZUelP4xtBDz85DnYGBVh4jnCxLtvHRj7fgas7JsUnYxAs-ueQ4svlbta-ulBEGFsHqAFcNaUPHylgdAljNwcrRZStsk8pGJ047vfZHxIHYurxUkds0-pZHXGLYnUTcANAHJ7aOBwO_dzqccxvJ3kkGr9voG-tAkNg3d_lQdMqibqqvkkZh46IG4I-Z11urP1T5-gxOOfHVMf1C5yOGXtgELMw8WbqYoWlEsGrzn-cOI8NR31qh-2eEZRFriHStgs1kTFrbv2-QA"
	/*
		if lMap, lMapOk := loginRes.(map[string]interface{}); lMapOk {
			if lToken, lTokensOk := lMap["id_token"]; lTokensOk {
				logs.WithContext(ctx).Info(fmt.Sprint(loginRes))
				idToken = lToken.(string)
			}
		}
		logs.WithContext(ctx).Info(idToken)

	*/

	tokens, tokensErr := jwt.DecryptTokenJWK(ctx, idToken, msAuth.MsConfig.JwkUrl)
	if tokensErr != nil {
		logs.WithContext(ctx).Error(tokensErr.Error())
		return Identity{}, LoginSuccess{}, tokensErr
	}
	logs.WithContext(ctx).Info(reflect.TypeOf(tokens).String())

	name := ""
	user_name := ""
	nonce := ""

	if tokenMap, tokenMapOk := tokens.(map[string]interface{}); tokenMapOk {
		if tokenName, tokenNameOk := tokenMap["name"]; tokenNameOk {
			name = tokenName.(string)
		}
		if tokenUserName, tokenUserNameOk := tokenMap["preferred_username"]; tokenUserNameOk {
			user_name = tokenUserName.(string)
		}
		if tokenNonce, tokenNonceOk := tokenMap["nonce"]; tokenNonceOk {
			nonce = tokenNonce.(string)
		}
	}

	logs.WithContext(ctx).Info(fmt.Sprint(name))
	logs.WithContext(ctx).Info(fmt.Sprint(user_name))
	logs.WithContext(ctx).Info(fmt.Sprint(nonce))
	if nonce != loginPostBody.Nonce {
		logs.WithContext(ctx).Error(fmt.Sprint("incorrect nonce : ", nonce, " expected nonce : ", loginPostBody.Nonce))
		return Identity{}, LoginSuccess{}, errors.New("mismatch in nonce received from identity provided")
	}

	/*
		var kratosSession KratosSession
		var kratosLoginFlow KratosFlow
		kratosLoginSucceed := true
		if loginBodyFromResDecodeErr := loginBodyFromRes.Decode(&kratosSession); loginBodyFromResDecodeErr != nil {
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

				consentChallenge, loginAcceptRequestCookies, loginAcceptErr := kratosHydraAuth.Hydra.acceptLoginRequest(ctx, kratosSession.Session.Identity.Id, loginChallenge, loginChallengeCookies)
				if loginAcceptErr != nil {
					err = loginAcceptErr
					return
				}
				identityHolder := make(map[string]interface{})
				identityHolder["identity"] = identity
				tokens, cosentAcceptErr := kratosHydraAuth.Hydra.acceptConsentRequest(ctx, identityHolder, consentChallenge, loginAcceptRequestCookies)
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

	*/
	return
}
