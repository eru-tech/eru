package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-crypto/jwt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
)

type GlAuth struct {
	Auth
	GlConfig GlConfig `json:"gl_config" eru:"required"`
	//Hydra    HydraConfig `json:"hydra" eru:"required"`
}

type GlConfig struct {
	AuthConfig
	Prompt string `json:"prompt" eru:"required"`
}

func (glAuth *GlAuth) PerformPreSaveTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreSaveTask - Start")
	for _, v := range glAuth.Hydra.HydraClients {
		err = glAuth.Hydra.SaveHydraClient(ctx, v)
		if err != nil {
			return err
		}
	}
	return
}

func (glAuth *GlAuth) PerformPreDeleteTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreDeleteTask - Start")
	for _, v := range glAuth.Hydra.HydraClients {
		err = glAuth.Hydra.RemoveHydraClient(ctx, v.ClientId)
		if err != nil {
			return err
		}
	}
	return
}

func (glAuth *GlAuth) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &glAuth)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (glAuth *GlAuth) GetUrl(ctx context.Context, state string) (urlStr string, oAuthParams OAuthParams, err error) {
	oAuthParams.ClientId = glAuth.GlConfig.ClientId
	oAuthParams.Scope = glAuth.GlConfig.Scope
	oAuthParams.RedirectURI = glAuth.GlConfig.RedirectURI
	oAuthParams.ResponseType = "code"
	oAuthParams.State = state
	oAuthParams.Prompt = glAuth.GlConfig.Prompt
	params := url.Values{}
	f := reflect.ValueOf(oAuthParams)
	for i := 0; i < f.NumField(); i++ {
		tags := f.Type().Field(i).Tag.Get("json")
		if tags != "" && f.Field(i).String() != "" {
			params.Add(tags, f.Field(i).String())
		}
	}
	urlStr = fmt.Sprint(glAuth.GlConfig.SsoBaseUrl, "?", params.Encode())
	oAuthParams.Url = urlStr
	return
}

func (glAuth *GlAuth) Login(ctx context.Context, loginPostBody LoginPostBody, projectId string, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {
	logs.WithContext(ctx).Debug("Login - Start")

	headers := http.Header{}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")

	glLoginFormBody := make(map[string]string)
	glLoginFormBody["client_id"] = glAuth.GlConfig.ClientId
	glLoginFormBody["client_secret"] = glAuth.GlConfig.ClientSecret
	logs.WithContext(ctx).Info(glAuth.GlConfig.RedirectURI)
	glLoginFormBody["redirect_uri"] = glAuth.GlConfig.RedirectURI
	glLoginFormBody["code"] = loginPostBody.IdpCode
	glLoginFormBody["grant_type"] = "authorization_code"

	loginRes, _, _, _, loginErr := utils.CallHttp(ctx, http.MethodPost, glAuth.GlConfig.TokenUrl, headers, glLoginFormBody, nil, nil, nil)
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

	tokens, tokensErr := jwt.DecryptTokenJWK(ctx, idToken, glAuth.GlConfig.JwkUrl)
	if tokensErr != nil {
		logs.WithContext(ctx).Error(tokensErr.Error())
		return Identity{}, LoginSuccess{}, tokensErr
	}

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
		query.Query = glAuth.AuthDb.GetDbQuery(ctx, SELECT_IDENTITY_SUB)
		query.Vals = append(query.Vals, sub)
		if tokenEmail, tokenEmailOk := tokenMap[glAuth.GlConfig.Identifiers.Email.IdpMapper]; tokenEmailOk {
			query.Vals = append(query.Vals, tokenEmail)
		} else {
			query.Vals = append(query.Vals, "")
		}

		logs.WithContext(ctx).Info(fmt.Sprint(query.Vals))
		output, outputErr := utils.ExecuteDbFetch(ctx, glAuth.AuthDb.GetConn(), query)
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

			var insertQueries []*models.Queries
			identity.Id = uuid.New().String()
			identifierFound := false
			var requiredIdentifiers []string
			logs.WithContext(ctx).Info(fmt.Sprint(glAuth.GlConfig.Identifiers))
			if glAuth.GlConfig.Identifiers.Email.Enable {
				if tokenEmail, tokenEmailOk := tokenMap[glAuth.GlConfig.Identifiers.Email.IdpMapper]; tokenEmailOk {
					userTraits.Email = tokenEmail.(string)
					userTraits.EmailVerified = true
					identity.Attributes["email"] = userTraits.Email
					identity.Attributes["email_verified"] = userTraits.EmailVerified // for sso email is considered as verified

					identifierFound = true
					insertQueryIcEmail := models.Queries{}
					insertQueryIcEmail.Query = glAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
					insertQueryIcEmail.Vals = append(insertQueryIcEmail.Vals, uuid.New().String(), identity.Id, userTraits.Email, "email")
					insertQueryIcEmail.Rank = 2

					insertQueries = append(insertQueries, &insertQueryIcEmail)
				} else {
					userTraits.Email = ""
					identity.Attributes["email"] = ""
					requiredIdentifiers = append(requiredIdentifiers, "email")
				}
			}
			if glAuth.GlConfig.Identifiers.Mobile.Enable {
				if tokenMobile, tokenMobileOk := tokenMap[glAuth.GlConfig.Identifiers.Mobile.IdpMapper]; tokenMobileOk {
					userTraits.Mobile = tokenMobile.(string)
					identity.Attributes["mobile"] = userTraits.Mobile

					identifierFound = true
					insertQueryIcMobile := models.Queries{}
					insertQueryIcMobile.Query = glAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
					insertQueryIcMobile.Vals = append(insertQueryIcMobile.Vals, uuid.New().String(), identity.Id, userTraits.Mobile, "mobile")
					insertQueryIcMobile.Rank = 3
					insertQueries = append(insertQueries, &insertQueryIcMobile)
				} else {
					userTraits.Mobile = ""
					identity.Attributes["mobile"] = ""
					requiredIdentifiers = append(requiredIdentifiers, "mobile")
				}
			}
			if glAuth.GlConfig.Identifiers.Username.Enable {
				if tokenUsername, tokenUsernameOk := tokenMap[glAuth.GlConfig.Identifiers.Username.IdpMapper]; tokenUsernameOk {
					userTraits.Username = tokenUsername.(string)
					identity.Attributes["username"] = userTraits.Username
					identifierFound = true
					insertQueryIcUsername := models.Queries{}
					insertQueryIcUsername.Query = glAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
					insertQueryIcUsername.Vals = append(insertQueryIcUsername.Vals, uuid.New().String(), identity.Id, userTraits.Username, "userName")
					insertQueryIcUsername.Rank = 4
					insertQueries = append(insertQueries, &insertQueryIcUsername)
				} else {
					userTraits.Username = ""
					identity.Attributes["username"] = ""
					requiredIdentifiers = append(requiredIdentifiers, "username")
				}
			}

			if !identifierFound {
				err = errors.New(fmt.Sprint("missing mandatory indentifiers : ", strings.Join(requiredIdentifiers, " , ")))
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, LoginSuccess{}, err
			}

			name := ""
			if tokenName, tokenNameOk := tokenMap["name"]; tokenNameOk {
				name = tokenName.(string)
			}
			nameArray := strings.Split(name, " ")
			userTraits.FirstName = nameArray[0]
			identity.Attributes["first_name"] = userTraits.FirstName
			if len(nameArray) > 1 {
				userTraits.LastName = nameArray[len(nameArray)-1]
				identity.Attributes["last_name"] = userTraits.LastName
			}
			logs.WithContext(ctx).Info(fmt.Sprint(userTraits))
			userTraitsBytes, userTraitsBytesErr := json.Marshal(userTraits)
			if userTraitsBytesErr != nil {
				err = userTraitsBytesErr
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}
			logs.WithContext(ctx).Info(fmt.Sprint(string(userTraitsBytes)))
			identity.Status = "ACTIVE"
			identity.AuthDetails = IdentityAuth{}
			identity.Attributes["sub"] = identity.Id
			identity.Attributes["idp"] = glAuth.AuthName
			identity.Attributes["idp_sub"] = sub

			userAttrs["sub"] = identity.Id
			userAttrs["idp"] = glAuth.AuthName
			userAttrs["idp_sub"] = sub

			userAttrsBytes, userAttrsBytesErr := json.Marshal(userAttrs)
			if userAttrsBytesErr != nil {
				err = userAttrsBytesErr
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}

			insertQuery := models.Queries{}
			insertQuery.Query = glAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY)
			insertQuery.Vals = append(insertQuery.Vals, identity.Id, glAuth.AuthName, sub, string(userTraitsBytes), string(userAttrsBytes))
			insertQuery.Rank = 1
			insertQueries = append(insertQueries, &insertQuery)

			sort.Sort(models.QueriesSorter(insertQueries))

			insertOutput, err := utils.ExecuteDbSave(ctx, glAuth.AuthDb.GetConn(), insertQueries)
			_ = insertOutput
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}

			if glAuth.Hooks.SWEF != "" {
				glAuth.sendWelcomeEmail(ctx, userTraits.Email, userTraits.FirstName, "", "email")
			} else {
				logs.WithContext(ctx).Info("SWEF hook not defined")
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
		loginChallenge, loginChallengeCookies, loginChallengeErr := glAuth.Hydra.GetLoginChallenge(ctx)
		if loginChallengeErr != nil {
			err = loginChallengeErr
			return
		}

		consentChallenge, loginAcceptRequestCookies, loginAcceptErr := glAuth.Hydra.AcceptLoginRequest(ctx, identity.Id, loginChallenge, loginChallengeCookies)
		if loginAcceptErr != nil {
			err = loginAcceptErr
			return
		}
		identityHolder := make(map[string]interface{})
		identityHolder["identity"] = identity
		eruTokens, cosentAcceptErr := glAuth.Hydra.AcceptConsentRequest(ctx, identityHolder, consentChallenge, loginAcceptRequestCookies)
		if cosentAcceptErr != nil {
			err = cosentAcceptErr
			return
		}
		eruTokens.Id = identity.Id
		logs.WithContext(ctx).Info(fmt.Sprint(identity))
		logs.WithContext(ctx).Info(fmt.Sprint(eruTokens))
		return identity, eruTokens, nil
	}
	return identity, LoginSuccess{}, nil
}

func (glAuth *GlAuth) GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error) {
	logs.WithContext(ctx).Debug("GetUserInfo - Start")
	return glAuth.Hydra.GetUserInfo(ctx, access_token)
}
