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
	"sort"
	"strings"
)

type MsAuth struct {
	Auth
	MsConfig MsConfig    `json:"ms_config" eru:"required"`
	Hydra    HydraConfig `json:"hydra" eru:"required"`
}

type MsConfig struct {
	ClientId     string      `json:"client_id" eru:"required"`
	ClientSecret string      `json:"client_secret" eru:"required"`
	RedirectURI  string      `json:"redirect_uri" eru:"required"`
	Scope        string      `json:"scope" eru:"required"`
	SsoBaseUrl   string      `json:"sso_base_url" eru:"required"`
	TokenUrl     string      `json:"token_url" eru:"required"`
	JwkUrl       string      `json:"jwk_url" eru:"required"`
	Identifiers  Identifiers `json:"identifiers" eru:"required"`
}

type MsParams struct {
	ClientId            string `json:"client_id"`
	Scope               string `json:"scope"`
	RedirectURI         string `json:"redirect_uri"`
	ClientRequestId     string `json:"client_request_id"`
	ResponseMode        string `json:"response_mode"`
	ResponseType        string `json:"response_type"`
	CodeChallenge       string `json:"code_challenge"`
	CodeVerifier        string `json:"code_verifier"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Nonce               string `json:"nonce"`
	State               string `json:"state"`
	Url                 string `json:"url"`
}

func (msAuth *MsAuth) PerformPreSaveTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreSaveTask - Start")
	for _, v := range msAuth.Hydra.HydraClients {
		err = msAuth.Hydra.SaveHydraClient(ctx, v)
		if err != nil {
			return err
		}
	}
	return
}

func (msAuth *MsAuth) PerformPreDeleteTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreDeleteTask - Start")
	for _, v := range msAuth.Hydra.HydraClients {
		err = msAuth.Hydra.RemoveHydraClient(ctx, v.ClientId)
		if err != nil {
			return err
		}
	}
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

	tokens, tokensErr := jwt.DecryptTokenJWK(ctx, idToken, msAuth.MsConfig.JwkUrl)
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

			var insertQueries []*models.Queries
			identity.Id = uuid.New().String()
			identifierFound := false
			var requiredIdentifiers []string
			logs.WithContext(ctx).Info(fmt.Sprint(msAuth.MsConfig.Identifiers))
			if msAuth.MsConfig.Identifiers.Email.Enable {
				if tokenEmail, tokenEmailOk := tokenMap[msAuth.MsConfig.Identifiers.Email.IdpMapper]; tokenEmailOk {
					userTraits.Email = tokenEmail.(string)
					userTraits.EmailVerified = true
					identity.Attributes["email"] = userTraits.Email
					identity.Attributes["email_verified"] = userTraits.EmailVerified // for sso email is considered as verified

					identifierFound = true
					insertQueryIcEmail := models.Queries{}
					insertQueryIcEmail.Query = msAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
					insertQueryIcEmail.Vals = append(insertQueryIcEmail.Vals, uuid.New().String(), identity.Id, userTraits.Email, "email")
					insertQueryIcEmail.Rank = 2

					insertQueries = append(insertQueries, &insertQueryIcEmail)
				} else {
					userTraits.Email = ""
					identity.Attributes["email"] = ""
					requiredIdentifiers = append(requiredIdentifiers, "email")
				}
			}
			if msAuth.MsConfig.Identifiers.Mobile.Enable {
				if tokenMobile, tokenMobileOk := tokenMap[msAuth.MsConfig.Identifiers.Mobile.IdpMapper]; tokenMobileOk {
					userTraits.Mobile = tokenMobile.(string)
					identity.Attributes["mobile"] = userTraits.Mobile

					identifierFound = true
					insertQueryIcMobile := models.Queries{}
					insertQueryIcMobile.Query = msAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
					insertQueryIcMobile.Vals = append(insertQueryIcMobile.Vals, uuid.New().String(), identity.Id, userTraits.Mobile, "mobile")
					insertQueryIcMobile.Rank = 3
					insertQueries = append(insertQueries, &insertQueryIcMobile)
				} else {
					userTraits.Mobile = ""
					identity.Attributes["mobile"] = ""
					requiredIdentifiers = append(requiredIdentifiers, "mobile")
				}
			}
			if msAuth.MsConfig.Identifiers.Username.Enable {
				if tokenUsername, tokenUsernameOk := tokenMap[msAuth.MsConfig.Identifiers.Username.IdpMapper]; tokenUsernameOk {
					userTraits.Username = tokenUsername.(string)
					identity.Attributes["username"] = userTraits.Username
					identifierFound = true
					insertQueryIcUsername := models.Queries{}
					insertQueryIcUsername.Query = msAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
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
			identity.Attributes["idp"] = msAuth.AuthName
			identity.Attributes["idp_sub"] = sub

			userAttrs["sub"] = identity.Id
			userAttrs["idp"] = msAuth.AuthName
			userAttrs["idp_sub"] = sub

			userAttrsBytes, userAttrsBytesErr := json.Marshal(userAttrs)
			if userAttrsBytesErr != nil {
				err = userAttrsBytesErr
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}

			insertQuery := models.Queries{}
			insertQuery.Query = msAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY)
			insertQuery.Vals = append(insertQuery.Vals, identity.Id, msAuth.AuthName, sub, string(userTraitsBytes), string(userAttrsBytes))
			insertQuery.Rank = 1
			insertQueries = append(insertQueries, &insertQuery)

			sort.Sort(models.QueriesSorter(insertQueries))

			insertOutput, err := utils.ExecuteDbSave(ctx, msAuth.AuthDb.GetConn(), insertQueries)
			_ = insertOutput
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}

			if msAuth.Hooks.SWEF != "" {
				msAuth.sendWelcomeEmail(ctx, userTraits.Email, userTraits.FirstName, "", "email")
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
		eruTokens.Id = identity.Id
		return identity, eruTokens, nil
	}
	return identity, LoginSuccess{}, nil
}

func (msAuth *MsAuth) GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error) {
	logs.WithContext(ctx).Debug("GetUserInfo - Start")
	return msAuth.Hydra.GetUserInfo(ctx, access_token)
}

func (msAuth *MsAuth) RemoveUser(ctx context.Context, removeUser RemoveUser) (err error) {
	logs.WithContext(ctx).Debug("RemoveUser - Start")

	var queries []*models.Queries
	idiQuery := models.Queries{}
	idiQuery.Query = msAuth.AuthDb.GetDbQuery(ctx, INSERT_DELETED_IDENTITY)
	idiQuery.Vals = append(idiQuery.Vals, removeUser.UserId)
	idiQuery.Rank = 1
	queries = append(queries, &idiQuery)

	dipQuery := models.Queries{}
	dipQuery.Query = msAuth.AuthDb.GetDbQuery(ctx, DELETE_IDENTITY_PASSWORD)
	dipQuery.Vals = append(dipQuery.Vals, removeUser.UserId)
	dipQuery.Rank = 2
	queries = append(queries, &dipQuery)

	dicQuery := models.Queries{}
	dicQuery.Query = msAuth.AuthDb.GetDbQuery(ctx, DELETE_IDENTITY_CREDENTIALS)
	dicQuery.Vals = append(dicQuery.Vals, removeUser.UserId)
	dicQuery.Rank = 3
	queries = append(queries, &dicQuery)

	diQuery := models.Queries{}
	diQuery.Query = msAuth.AuthDb.GetDbQuery(ctx, DELETE_IDENTITY)
	diQuery.Vals = append(diQuery.Vals, removeUser.UserId)
	diQuery.Rank = 4
	queries = append(queries, &diQuery)

	_, err = utils.ExecuteDbSave(ctx, msAuth.AuthDb.GetConn(), queries)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return errors.New("something went wrong - please try again")
	}
	return
}
