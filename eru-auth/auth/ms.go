package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-crypto/jwt"
	erupkce "github.com/eru-tech/eru/eru-crypto/pkce"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

type MsAuth struct {
	Auth
	MsConfig MsConfig    `json:"msConfig" eru:"required"`
	Hydra    HydraConfig `json:"hydra" eru:"required"`
}

type MsConfig struct {
	ClientId     string      `json:"clientId" eru:"required"`
	ClientSecret string      `json:"clientSecret" eru:"required"`
	RedirectURI  string      `json:"redirectUri" eru:"required"`
	Scope        string      `json:"scope" eru:"required"`
	SsoBaseUrl   string      `json:"ssoBaseUrl" eru:"required"`
	TokenUrl     string      `json:"tokenUrl" eru:"required"`
	JwkUrl       string      `json:"jwkUrl" eru:"required"`
	Identifiers  Identifiers `json:"identifiers" eru:"required"`
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

	loginRes, _, _, _, loginErr := utils.CallHttp(ctx, http.MethodPost, msAuth.MsConfig.TokenUrl, headers, msLoginFormBody, nil, nil, nil)
	if loginErr != nil {
		logs.WithContext(ctx).Error(fmt.Sprint(map[string]interface{}{"request_id": loginPostBody.IdpRequestId, "error": fmt.Sprint(loginErr)}))
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	idToken := ""

	if lMap, lMapOk := loginRes.(map[string]interface{}); lMapOk {

		if lToken, lTokensOk := lMap["id_token"]; lTokensOk {
			logs.WithContext(ctx).Info(fmt.Sprint(loginRes))
			idToken = lToken.(string)
		}
	}

	logs.WithContext(ctx).Info(idToken)

	tokens, tokensErr := jwt.DecryptTokenJWK(ctx, idToken, msAuth.MsConfig.JwkUrl)
	if tokensErr != nil {
		logs.WithContext(ctx).Error(tokensErr.Error())
		return Identity{}, LoginSuccess{}, tokensErr
	}
	logs.WithContext(ctx).Info(reflect.TypeOf(tokens).String())

	nonce := ""
	sub := ""
	userTraits := UserTraits{}
	userAttrs := make(map[string]string)

	if tokenMap, tokenMapOk := tokens.(map[string]interface{}); tokenMapOk {
		if tokenNonce, tokenNonceOk := tokenMap["nonce"]; tokenNonceOk {
			nonce = tokenNonce.(string)
		}
		if nonce != loginPostBody.Nonce {
			logs.WithContext(ctx).Error(fmt.Sprint("incorrect nonce : ", nonce, " expected nonce : ", loginPostBody.Nonce))
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		}

		if tokenSub, tokenSubOk := tokenMap["sub"]; tokenSubOk {
			sub = tokenSub.(string)
		}
		query := models.Queries{}
		query.Query = msAuth.AuthDb.GetDbQuery(ctx, SELECT_IDENTITY_SUB)
		query.Vals = append(query.Vals, sub)
		output, outputErr := utils.ExecuteDbFetch(ctx, msAuth.AuthDb.GetConn(), query)
		if outputErr != nil {
			err = outputErr
			logs.WithContext(ctx).Error(err.Error())
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		}
		if identity.Attributes == nil {
			identity.Attributes = make(map[string]interface{})
		}
		//creating Just-In-Time user if not found in eru database
		if len(output) == 0 {
			if msAuth.MsConfig.Identifiers.Email.Enable {
				if tokenEmail, tokenEmailOk := tokenMap[msAuth.MsConfig.Identifiers.Email.IdpMapper]; tokenEmailOk {
					userTraits.Email = tokenEmail.(string)
					identity.Attributes["email"] = userTraits.Email
				}
			}
			if msAuth.MsConfig.Identifiers.Mobile.Enable {
				if tokenMobile, tokenMobileOk := tokenMap[msAuth.MsConfig.Identifiers.Mobile.IdpMapper]; tokenMobileOk {
					userTraits.Mobile = tokenMobile.(string)
					identity.Attributes["mobile"] = userTraits.Mobile
				}
			}
			if msAuth.MsConfig.Identifiers.Username.Enable {
				if tokenUsername, tokenUsernameOk := tokenMap[msAuth.MsConfig.Identifiers.Username.IdpMapper]; tokenUsernameOk {
					userTraits.Username = tokenUsername.(string)
					identity.Attributes["userName"] = userTraits.Username
				}
			}

			name := ""
			if tokenName, tokenNameOk := tokenMap["name"]; tokenNameOk {
				name = tokenName.(string)
			}
			nameArray := strings.Split(name, " ")
			userTraits.FirstName = nameArray[0]
			identity.Attributes["firstName"] = userTraits.FirstName
			if len(nameArray) > 1 {
				userTraits.LastName = nameArray[len(nameArray)-1]
				identity.Attributes["lastName"] = userTraits.LastName
			}

			userTraitsBytes, userTraitsBytesErr := json.Marshal(userTraits)
			if userTraitsBytesErr != nil {
				err = userTraitsBytesErr
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}

			identity.Id = uuid.New().String()
			identity.Status = "ACTIVE"
			identity.AuthDetails = IdentityAuth{}
			identity.Attributes["sub"] = identity.Id
			identity.Attributes["idp"] = msAuth.AuthName
			identity.Attributes["idpSub"] = sub

			userAttrs["sub"] = identity.Id
			userAttrs["idp"] = msAuth.AuthName
			userAttrs["idpSub"] = sub

			userAttrsBytes, userAttrsBytesErr := json.Marshal(userAttrs)
			if userAttrsBytesErr != nil {
				err = userAttrsBytesErr
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}

			var insertQueries []*models.Queries
			insertQuery := models.Queries{}
			insertQuery.Query = msAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY)
			insertQuery.Vals = append(insertQuery.Vals, identity.Id, msAuth.AuthName, sub, string(userTraitsBytes), string(userAttrsBytes))
			insertQueries = append(insertQueries, &insertQuery)
			insertOutput, err := utils.ExecuteDbSave(ctx, msAuth.AuthDb.GetConn(), insertQueries)
			logs.WithContext(ctx).Info(fmt.Sprint(insertOutput))
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}
		} else {
			identity.Id = output[0]["identity_id"].(string)
			if output[0]["is_active"].(bool) {
				identity.Status = "ACTIVE"
			} else {
				identity.Status = "INACTIVE"
			}
			identity.AuthDetails = IdentityAuth{}
			if traitsMap, traitsMapOk := output[0]["traits"].(*map[string]interface{}); traitsMapOk {
				for k, v := range *traitsMap {
					identity.Attributes[k] = v
				}
			}
			if attrMap, attrMapOk := output[0]["attributes"].(*map[string]interface{}); attrMapOk {
				for k, v := range *attrMap {
					identity.Attributes[k] = v
				}
			}
		}
	} else {
		logs.WithContext(ctx).Error("token recevied from IDP is not a map")
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	if withTokens {
		loginChallenge, loginChallengeCookies, loginChallengeErr := msAuth.Hydra.GetLoginChallenge(ctx)
		if loginChallengeErr != nil {
			err = loginChallengeErr
			return
		}

		consentChallenge, loginAcceptRequestCookies, loginAcceptErr := msAuth.Hydra.AcceptLoginRequest(ctx, identity.Id, loginChallenge, loginChallengeCookies)
		if loginAcceptErr != nil {
			err = loginAcceptErr
			return
		}
		identityHolder := make(map[string]interface{})
		identityHolder["identity"] = identity
		eruTokens, cosentAcceptErr := msAuth.Hydra.AcceptConsentRequest(ctx, identityHolder, consentChallenge, loginAcceptRequestCookies)
		if cosentAcceptErr != nil {
			err = cosentAcceptErr
			return
		}
		return identity, eruTokens, nil
	}
	return identity, LoginSuccess{}, nil
}

func (msAuth *MsAuth) GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error) {
	logs.WithContext(ctx).Debug("GetUserInfo - Start")
	return msAuth.Hydra.GetUserInfo(ctx, access_token)
}
