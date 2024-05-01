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

type OAuth struct {
	Auth
	OAuthConfig OAuthConfig `json:"oauth_config" eru:"required"`
	Hydra       HydraConfig `json:"hydra" eru:"required"`
}

type OAuthConfig struct {
	ClientId            string      `json:"client_id"`
	ClientSecret        string      `json:"client_secret"`
	RedirectURI         string      `json:"redirect_uri" eru:"required"`
	CodeKey             string      `json:"code_key"`
	TokenUrlContentType string      `json:"token_url_content_type"`
	Scope               string      `json:"scope"`
	SsoBaseUrl          string      `json:"sso_base_url" eru:"required"`
	TokenUrl            string      `json:"token_url" eru:"required"`
	JwkUrl              string      `json:"jwk_url"`
	Identifiers         Identifiers `json:"identifiers"`
	TokenKey            string      `json:"token_key"`
}

func (oAuth *OAuth) PerformPreSaveTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreSaveTask - Start")
	for _, v := range oAuth.Hydra.HydraClients {
		err = oAuth.Hydra.SaveHydraClient(ctx, v)
		if err != nil {
			return err
		}
	}
	return
}

func (oAuth *OAuth) PerformPreDeleteTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreDeleteTask - Start")
	for _, v := range oAuth.Hydra.HydraClients {
		err = oAuth.Hydra.RemoveHydraClient(ctx, v.ClientId)
		if err != nil {
			return err
		}
	}
	return
}

func (oAuth *OAuth) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &oAuth)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (oAuth *OAuth) GetUrl(ctx context.Context, state string) (urlStr string, oAuthParams OAuthParams, err error) {
	oAuthParams.ClientId = oAuth.OAuthConfig.ClientId
	oAuthParams.Scope = oAuth.OAuthConfig.Scope
	oAuthParams.RedirectURI = oAuth.OAuthConfig.RedirectURI
	oAuthParams.ResponseType = "code"
	oAuthParams.State = state

	params := url.Values{}
	f := reflect.ValueOf(oAuthParams)
	for i := 0; i < f.NumField(); i++ {
		tags := f.Type().Field(i).Tag.Get("json")
		if tags != "redirect_key" {
			if tags != "" && f.Field(i).String() != "" {
				params.Add(tags, f.Field(i).String())
			}
		}
	}
	urlStr = fmt.Sprint(oAuth.OAuthConfig.SsoBaseUrl, "?", params.Encode())
	oAuthParams.Url = urlStr
	return
}

func (oAuth *OAuth) Login(ctx context.Context, loginPostBody LoginPostBody, projectId string, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {
	logs.WithContext(ctx).Debug("Login - Start")

	headers := http.Header{}
	//headers.Set("Host", "localhost:8083")
	contentType := "application/x-www-form-urlencoded"
	if oAuth.OAuthConfig.TokenUrlContentType != "" {
		contentType = oAuth.OAuthConfig.TokenUrlContentType
	}
	headers.Set("Content-Type", contentType)

	glLoginFormBody := make(map[string]string)
	if oAuth.OAuthConfig.ClientId != "" {
		glLoginFormBody["client_id"] = oAuth.OAuthConfig.ClientId
	}

	if oAuth.OAuthConfig.ClientSecret != "" {
		glLoginFormBody["client_secret"] = oAuth.OAuthConfig.ClientSecret
	}

	if oAuth.OAuthConfig.RedirectURI != "" {
		glLoginFormBody["redirect_uri"] = oAuth.OAuthConfig.RedirectURI
	}

	if loginPostBody.IdpCode != "" {
		if oAuth.OAuthConfig.CodeKey == "" {
			glLoginFormBody["code"] = loginPostBody.IdpCode
		} else {
			glLoginFormBody[oAuth.OAuthConfig.CodeKey] = loginPostBody.IdpCode
		}

	}
	glLoginFormBody["grant_type"] = "authorization_code"
	var loginErr error
	var loginRes interface{}
	if contentType == "application/x-www-form-urlencoded" {
		loginRes, _, _, _, loginErr = utils.CallHttp(ctx, http.MethodPost, oAuth.OAuthConfig.TokenUrl, headers, glLoginFormBody, nil, nil, nil)
	} else if contentType == "application/json" {
		loginRes, _, _, _, loginErr = utils.CallHttp(ctx, http.MethodPost, oAuth.OAuthConfig.TokenUrl, headers, nil, nil, nil, glLoginFormBody)
	} else {
		loginErr = errors.New("invalid token api content type")
	}

	if loginErr != nil {
		logs.WithContext(ctx).Error(fmt.Sprint(map[string]interface{}{"request_id": loginPostBody.IdpRequestId, "error": fmt.Sprint(loginErr)}))
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	idToken := ""
	lMap := make(map[string]interface{})
	lMapOk := false
	if lMap, lMapOk = loginRes.(map[string]interface{}); lMapOk {
		if lToken, lTokensOk := lMap[oAuth.OAuthConfig.TokenKey]; lTokensOk {
			logs.WithContext(ctx).Info(fmt.Sprint(loginRes))
			idToken = lToken.(string)
		} else {
			logs.WithContext(ctx).Error("token could not be retrieved from IDP response")
		}
	} else {
		logs.WithContext(ctx).Error("token API response received from IDP is not a map")
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	logs.WithContext(ctx).Info(idToken)
	logs.WithContext(ctx).Info(fmt.Sprint(lMap))
	tokenMap := make(map[string]interface{})
	tokenMapOk := false
	strTokenEmail := ""
	nonce := ""
	sub := ""

	if idToken != "" && oAuth.OAuthConfig.JwkUrl != "" {
		tokens, tokensErr := jwt.DecryptTokenJWK(ctx, idToken, oAuth.OAuthConfig.JwkUrl)
		if tokensErr != nil {
			logs.WithContext(ctx).Error(tokensErr.Error())
			return Identity{}, LoginSuccess{}, tokensErr
		}
		if tokenMap, tokenMapOk = tokens.(map[string]interface{}); !tokenMapOk {
			logs.WithContext(ctx).Error("token recevied from IDP is not a map")
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		}

	} else {
		if oAuth.Hooks.USRP != "" {
			hookRes, hookErr := triggerHook(ctx, oAuth.Hooks.USRP, projectId, lMap)
			if hookErr != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("error while executing USRP hook :", hookErr.Error()))
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}
			if tokenMap, tokenMapOk = hookRes.(map[string]interface{}); !tokenMapOk {
				logs.WithContext(ctx).Error(fmt.Sprint("USRP hook response is not a map"))
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}
		} else {
			logs.WithContext(ctx).Error(fmt.Sprint("USRP hook not defined"))
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		}
	}

	if tokenNonce, tokenNonceOk := tokenMap["nonce"]; tokenNonceOk {
		nonce = tokenNonce.(string)
	}
	if nonce != loginPostBody.Nonce {
		logs.WithContext(ctx).Error(fmt.Sprint("incorrect nonce : ", nonce, " expected nonce : ", loginPostBody.Nonce))
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}
	if tokenSub, tokenSubOk := tokenMap[oAuth.OAuthConfig.Identifiers.UserId.IdpMapper]; tokenSubOk {
		sub = tokenSub.(string)
	}
	if tokenEmail, tokenEmailOk := tokenMap[oAuth.OAuthConfig.Identifiers.Email.IdpMapper]; tokenEmailOk {
		strTokenEmail = tokenEmail.(string)
	}
	var output []map[string]interface{}
	var outputErr error
	query := models.Queries{}
	query.Query = oAuth.AuthDb.GetDbQuery(ctx, SELECT_IDENTITY_SUB)
	query.Vals = append(query.Vals, sub)
	query.Vals = append(query.Vals, strTokenEmail)
	output, outputErr = utils.ExecuteDbFetch(ctx, oAuth.AuthDb.GetConn(), query)
	if outputErr != nil {
		err = outputErr
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}
	if identity.Attributes == nil {
		identity.Attributes = make(map[string]interface{})
	}
	identity.Attributes[oAuth.OAuthConfig.TokenKey] = lMap[oAuth.OAuthConfig.TokenKey]
	logs.WithContext(ctx).Info(fmt.Sprint(output))
	//creating Just-In-Time user if not found in eru database
	if len(output) == 0 {
		if oAuth.Hooks.USRR != "" {
			_, hookErr := triggerHook(ctx, oAuth.Hooks.USRR, projectId, tokenMap)
			if hookErr != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("error while executing USRR hook :", hookErr.Error()))
				return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
			}
		} else {
			logs.WithContext(ctx).Error(fmt.Sprint("USRR hook not defined"))
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		}

		//execute query again
		output, outputErr = utils.ExecuteDbFetch(ctx, oAuth.AuthDb.GetConn(), query)
		if outputErr != nil {
			err = outputErr
			logs.WithContext(ctx).Error(err.Error())
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		}

	}
	logs.WithContext(ctx).Info(fmt.Sprint(output))
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

	if withTokens {
		loginChallenge, loginChallengeCookies, loginChallengeErr := oAuth.Hydra.GetLoginChallenge(ctx)
		if loginChallengeErr != nil {
			err = loginChallengeErr
			return
		}

		consentChallenge, loginAcceptRequestCookies, loginAcceptErr := oAuth.Hydra.AcceptLoginRequest(ctx, identity.Id, loginChallenge, loginChallengeCookies)
		if loginAcceptErr != nil {
			err = loginAcceptErr
			return
		}
		identityHolder := make(map[string]interface{})
		identityHolder["identity"] = identity
		eruTokens, cosentAcceptErr := oAuth.Hydra.AcceptConsentRequest(ctx, identityHolder, consentChallenge, loginAcceptRequestCookies)
		if cosentAcceptErr != nil {
			err = cosentAcceptErr
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		eruTokens.Id = identity.Id
		logs.WithContext(ctx).Info(fmt.Sprint(identity))
		logs.WithContext(ctx).Info(fmt.Sprint(eruTokens))
		return identity, eruTokens, nil
	}
	return identity, LoginSuccess{}, nil
}

func (oAuth *OAuth) GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error) {
	logs.WithContext(ctx).Debug("GetUserInfo - Start")
	return oAuth.Hydra.GetUserInfo(ctx, access_token)
}

func (oAuth *OAuth) Register(ctx context.Context, registerUser RegisterUser, projectId string) (identity Identity, tokens LoginSuccess, err error) {
	logs.WithContext(ctx).Debug("Register - Start")

	userTraits := UserTraits{}
	userAttrs := make(map[string]string)

	if identity.Attributes == nil {
		identity.Attributes = make(map[string]interface{})
	}
	identifierFound := false
	var requiredIdentifiers []string
	var insertQueries []*models.Queries
	identity.Id = uuid.New().String()

	if oAuth.OAuthConfig.Identifiers.Email.Enable {
		userTraits.Email = registerUser.Email
		userTraits.EmailVerified = true //for sso, consider email as verified if received from IDP
		identity.Attributes["email"] = userTraits.Email
		identity.Attributes["email_verified"] = userTraits.EmailVerified

		if registerUser.Email != "" {
			identifierFound = true
			insertQueryIcEmail := models.Queries{}
			insertQueryIcEmail.Query = oAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
			insertQueryIcEmail.Vals = append(insertQueryIcEmail.Vals, uuid.New().String(), identity.Id, userTraits.Email, "email")
			insertQueryIcEmail.Rank = 2

			insertQueries = append(insertQueries, &insertQueryIcEmail)
		} else {
			requiredIdentifiers = append(requiredIdentifiers, "email")
		}
	}
	if oAuth.OAuthConfig.Identifiers.Mobile.Enable {
		userTraits.Mobile = registerUser.Mobile
		userTraits.MobileVerified = true //for sso, consider mobile as verified if received from IDP
		identity.Attributes["mobile"] = userTraits.Mobile
		identity.Attributes["mobile_verified"] = userTraits.MobileVerified
		if registerUser.Mobile != "" {
			identifierFound = true
			insertQueryIcMobile := models.Queries{}
			insertQueryIcMobile.Query = oAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
			insertQueryIcMobile.Vals = append(insertQueryIcMobile.Vals, uuid.New().String(), identity.Id, userTraits.Mobile, "mobile")
			insertQueryIcMobile.Rank = 3
			insertQueries = append(insertQueries, &insertQueryIcMobile)
		} else {
			requiredIdentifiers = append(requiredIdentifiers, "mobile")
		}
	}
	if oAuth.OAuthConfig.Identifiers.Username.Enable {
		userTraits.Username = registerUser.Username
		identity.Attributes["username"] = userTraits.Username
		if registerUser.Username != "" {
			identifierFound = true
			insertQueryIcUsername := models.Queries{}
			insertQueryIcUsername.Query = oAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
			insertQueryIcUsername.Vals = append(insertQueryIcUsername.Vals, uuid.New().String(), identity.Id, userTraits.Username, "userName")
			insertQueryIcUsername.Rank = 4
			insertQueries = append(insertQueries, &insertQueryIcUsername)
		} else {
			requiredIdentifiers = append(requiredIdentifiers, "userName")
		}
	}

	if !identifierFound {
		err = errors.New(fmt.Sprint("missing mandatory indentifiers : ", strings.Join(requiredIdentifiers, " , ")))
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, LoginSuccess{}, err
	}

	userTraits.FirstName = registerUser.FirstName
	identity.Attributes["first_name"] = userTraits.FirstName

	userTraits.LastName = registerUser.LastName
	identity.Attributes["last_name"] = userTraits.LastName

	userTraitsBytes, userTraitsBytesErr := json.Marshal(userTraits)
	if userTraitsBytesErr != nil {
		err = userTraitsBytesErr
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	identity.Status = "ACTIVE"
	identity.AuthDetails = IdentityAuth{}
	identity.Attributes["sub"] = identity.Id
	identity.Attributes["idp"] = oAuth.AuthName

	userAttrs["sub"] = identity.Id
	userAttrs["idp"] = oAuth.AuthName

	for k, v := range registerUser.UserAttributes {
		userAttrs[k] = v
		identity.Attributes[k] = v
	}

	userAttrsBytes, userAttrsBytesErr := json.Marshal(userAttrs)
	if userAttrsBytesErr != nil {
		err = userAttrsBytesErr
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	insertQuery := models.Queries{}
	insertQuery.Query = oAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY)
	insertQuery.Vals = append(insertQuery.Vals, identity.Id, oAuth.AuthName, identity.Attributes["idp_sub"], string(userTraitsBytes), string(userAttrsBytes))
	insertQuery.Rank = 1
	insertQueries = append(insertQueries, &insertQuery)

	sort.Sort(models.QueriesSorter(insertQueries))

	insertOutput, err := utils.ExecuteDbSave(ctx, oAuth.AuthDb.GetConn(), insertQueries)
	logs.WithContext(ctx).Info(fmt.Sprint(insertOutput))
	if err != nil {
		if strings.Contains(err.Error(), "unique_identity_credential") {
			return Identity{}, LoginSuccess{}, errors.New("username already exists")
		}
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	//tokens, err = oAuth.makeTokens(ctx, identity)

	if oAuth.Hooks.SWEF != "" {
		oAuth.sendWelcomeEmail(ctx, "abaradia@gmail.com", userTraits.FirstName, projectId, "email")
	} else {
		logs.WithContext(ctx).Info("SWEF hook not defined")
	}
	return identity, tokens, nil
}
