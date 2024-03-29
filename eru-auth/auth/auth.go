package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	routes_store "github.com/eru-tech/eru/eru-routes/module_store"
	"github.com/eru-tech/eru/eru-routes/routes"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/uuid"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type AuthI interface {
	//Login(req *http.Request) (res interface{}, cookies []*http.Cookie, err error)
	SetAuthDb(authDbI AuthDbI)
	GetAuthDb() (authDbI AuthDbI)
	Login(ctx context.Context, loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error)
	Register(ctx context.Context, registerUser RegisterUser, projectId string) (identity Identity, loginSuccess LoginSuccess, err error)
	RemoveUser(ctx context.Context, removeUser RemoveUser) (err error)
	Logout(ctx context.Context, req *http.Request) (res interface{}, resStatusCode int, err error)
	VerifyToken(ctx context.Context, tokenType string, token string) (res interface{}, err error)
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error)
	FetchTokens(ctx context.Context, refresh_token string, userId string) (res interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) (err error)
	PerformPreSaveTask(ctx context.Context) (err error)
	PerformPreDeleteTask(ctx context.Context) (err error)
	GetUser(ctx context.Context, userId string) (identity Identity, err error)
	UpdateUser(ctx context.Context, identityToUpdate Identity, userId string, token map[string]interface{}) (err error)
	ChangePassword(ctx context.Context, tokenObj map[string]interface{}, userId string, changePasswordObj ChangePassword) (err error)
	GenerateRecoveryCode(ctx context.Context, recoveryIdentifier RecoveryPostBody, projectId string, silentFlag bool) (msg string, err error)
	GenerateVerifyCode(ctx context.Context, verifyIdentifier VerifyPostBody, projectId string, silentFlag bool) (msg string, err error)
	CompleteRecovery(ctx context.Context, recoveryPassword RecoveryPassword, cookies []*http.Cookie) (msg string, err error)
	VerifyRecovery(ctx context.Context, recoveryPassword RecoveryPassword) (res map[string]string, cookies []*http.Cookie, err error)
	VerifyCode(ctx context.Context, verifyCode VerifyCode, tokenObj map[string]interface{}, withToken bool) (res interface{}, err error)
	GetUrl(ctx context.Context, state string) (url string, msParams MsParams, err error)
}

const (
	SELECT_IDENTITY_SUB            = "select * from eruauth_identities where identity_provider_id = ???"
	INSERT_IDENTITY                = "insert into eruauth_identities (identity_id,identity_provider,identity_provider_id,traits,attributes) values (???,???,???,???,???)"
	UPDATE_IDENTITY                = "update eruauth_identities set traits = ??? , attributes = ??? where identity_id = ???"
	INSERT_IDENTITY_CREDENTIALS    = "insert into eruauth_identity_credentials (identity_credential_id , identity_id, identity_credential, identity_credential_type) values (???,???,???,???)"
	DELETE_IDENTITY_CREDENTIALS    = "delete from eruauth_identity_credentials where identity_id = ??? and identity_credential_type = ??? and identity_credential = ??? "
	INSERT_IDENTITY_PASSWORD       = "insert into eruauth_identity_passwords (identity_password_id,identity_id,identity_password) values (??? , ??? , ???)"
	SELECT_LOGIN                   = "select a.* , case when is_active=true then 'Active' else 'Inactive' end status from eruauth_identities a inner join eruauth_identity_credentials b on a.identity_id=b.identity_id and b.identity_credential= ??? inner join eruauth_identity_passwords c on a.identity_id=c.identity_id and c.identity_password= ???"
	SELECT_LOGIN_ID                = "select a.* , case when is_active=true then 'Active' else 'Inactive' end status from eruauth_identities a inner join eruauth_identity_passwords c on a.identity_id=c.identity_id and c.identity_password= ??? where a.identity_id= ???"
	SELECT_IDENTITY                = "select a.* , case when is_active=true then 'Active' else 'Inactive' end status from eruauth_identities a  where a.identity_id = ???"
	SELECT_IDENTITY_CREDENTIAL     = "select b.traits->>'firstName' first_name , a.* from eruauth_identity_credentials a left join eruauth_identities b on a.identity_id=b.identity_id where a.identity_credential = ???"
	INSERT_OTP                     = "insert into eruauth_otp (otp_id, otp, identity_credential,identity_credential_type,otp_purpose) values (??? , ??? , ???,??? , ???)"
	VERIFY_OTP                     = "select b.identity_id, a.* from eruauth_otp a left join eruauth_identity_credentials b on a.identity_credential=b.identity_credential where identity_id = ??? and otp = ??? and a.identity_credential = ??? and a.created_date + (5 * interval '1 minute') >= LOCALTIMESTAMP and otp_purpose = ???"
	VERIFY_RECOVERY_OTP            = "select b.identity_id, a.* from eruauth_otp a left join eruauth_identity_credentials b on a.identity_credential=b.identity_credential where otp = ??? and a.identity_credential = ??? and a.created_date + (5 * interval '1 minute') >= LOCALTIMESTAMP and otp_purpose = ???"
	CHANGE_PASSWORD                = "update eruauth_identity_passwords set updated_date=LOCALTIMESTAMP, identity_password= ??? where identity_id= ???"
	INSERT_DELETED_IDENTITY        = "insert into eruauth_deleted_identities (identity_id,identity_provider,identity_provider_id,traits,attributes,is_active,identity_password) select a.identity_id,identity_provider,identity_provider_id,traits,attributes,is_active, b.identity_password  from eruauth_identities a left join eruauth_identity_passwords b on a.identity_id=b.identity_id where a.identity_id= ???"
	DELETE_IDENTITY_PASSWORD       = "delete from eruauth_identity_passwords where identity_id= ???"
	DELETE_IDENTITY_CREDENTIALS_ID = "delete from eruauth_identity_credentials where identity_id= ???"
	DELETE_IDENTITY                = "delete from eruauth_identities where identity_id= ???"
)

const (
	OTP_PURPOSE_RECOVERY = "RECOVERY"
	OTP_PURPOSE_VERIFY   = "VERIFY"
)

type ChangePassword struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type LoginPostBody struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	IdpCode      string `json:"code"`
	IdpRequestId string `json:"requestId"`
	CodeVerifier string `json:"-"`
	Nonce        string `json:"-"`
}

type RecoveryPostBody struct {
	Username string `json:"username"`
}

type VerifyPostBody struct {
	Username       string `json:"username"`
	CredentialType string `json:"credentialType"`
}

type RecoveryPassword struct {
	Code     string `json:"code"`
	Id       string `json:"id"`
	Password string `json:"password"`
}

type VerifyCode struct {
	Code   string `json:"code"`
	Id     string `json:"id"`
	UserId string `json:"-"`
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
	Hooks          AuthHooks `eru:"optional"`
	AuthDb         AuthDbI   `json:"-"`
}

type AuthHooks struct {
	SRC  routes.Route
	SRCF routes.FuncGroup
	SVCF routes.FuncGroup
	SWEF routes.FuncGroup
}

type IdentifierConfig struct {
	Enable    bool   `json:"enable"`
	IdpMapper string `json:"idpMapper"`
}

type Identifiers struct {
	Email    IdentifierConfig `json:"email"`
	Mobile   IdentifierConfig `json:"mobile"`
	Username IdentifierConfig `json:"userName"`
}

type UserTraits struct {
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	Email          string `json:"email"`
	Mobile         string `json:"mobile"`
	Username       string `json:"userName"`
	EmailVerified  bool   `json:"emailVerified"`
	MobileVerified bool   `json:"mobileVerified"`
}

type RegisterUser struct {
	UserTraits
	Password string `json:"password"`
}

type RemoveUser struct {
	UserId string `json:"id"`
}

func (auth *Auth) SetAuthDb(authDbI AuthDbI) {
	auth.AuthDb = authDbI
}

func (auth *Auth) GetAuthDb() (authDbI AuthDbI) {
	return auth.AuthDb
}

func (auth *Auth) GetUrl(ctx context.Context, state string) (url string, msParams MsParams, err error) {
	err = errors.New("GetUrl Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return
}

func (auth *Auth) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	err := errors.New("MakeFromJson Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}

func (auth *Auth) GenerateRecoveryCode(ctx context.Context, recoveryIdentifier RecoveryPostBody, projectId string, silentFlag bool) (msg string, err error) {
	err = errors.New("GenerateRecoveryCode Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return "", err
}

func (auth *Auth) sendCode(ctx context.Context, credentialIdentifier string, recovery_code string, recovery_time string, name string, projectId string, purpose string, credentialType string) (err error) {
	logs.WithContext(ctx).Debug("sendRecoveryCode - Start")
	trReqVars := &routes.TemplateVars{}
	if trReqVars.Vars == nil {
		trReqVars.Vars = make(map[string]interface{})
	}
	trReqVars.Vars["credential_type"] = credentialType
	trReqVars.Vars[credentialType] = credentialIdentifier
	trReqVars.Vars["recovery_code"] = recovery_code
	trReqVars.Vars["recovery_time"] = recovery_time
	trReqVars.Vars["name"] = name

	r := &http.Request{}
	url := url.URL{
		Scheme: "",
		Host:   "",
		Path:   "/",
	}
	r.URL = &url
	rBytes, rBytesErr := json.Marshal(trReqVars.Vars)
	if rBytesErr != nil {
		return rBytesErr
	}
	r.Body = io.NopCloser(strings.NewReader(string(rBytes)))
	h := http.Header{}
	h.Set("content-type", "application/json")
	r.Header = h
	r.Header.Set("Content-Length", strconv.Itoa(len(rBytes)))
	r.ContentLength = int64(len(rBytes))

	srcHookFound := false
	logs.WithContext(ctx).Info(auth.Hooks.SRC.RouteName)
	if auth.Hooks.SRC.RouteName != "" {
		_, _, respErr := auth.Hooks.SRC.Execute(r.Context(), r, "/", false, "", trReqVars, 1)
		srcHookFound = true
		return respErr
	}
	if purpose == OTP_PURPOSE_RECOVERY {
		logs.WithContext(ctx).Info(auth.Hooks.SRCF.FuncGroupName)
		if auth.Hooks.SRCF.FuncGroupName != "" {
			var errArray []string
			rs := routes_store.ModuleStore{}
			for k, v := range auth.Hooks.SRCF.FuncSteps {
				fs := auth.Hooks.SRCF.FuncSteps[k]
				err = rs.LoadRoutesForFunction(ctx, fs, v.RouteName, projectId, "", v.Path, "", nil, nil)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					errArray = append(errArray, err.Error())
				}
			}
			if len(errArray) > 0 {
				err = errors.New(strings.Join(errArray, " , "))
				logs.WithContext(ctx).Error(err.Error())
				return
			}
			_, respErr := auth.Hooks.SRCF.Execute(r.Context(), r, 1, 1)
			srcHookFound = true
			return respErr
		}
		if !srcHookFound {
			logs.WithContext(ctx).Warn("SRC hook not defined for auth. Thus no email was triggered.")
		}
	}
	if purpose == OTP_PURPOSE_VERIFY {
		logs.WithContext(ctx).Info(auth.Hooks.SVCF.FuncGroupName)
		if auth.Hooks.SVCF.FuncGroupName != "" {
			var errArray []string
			rs := routes_store.ModuleStore{}
			for k, v := range auth.Hooks.SVCF.FuncSteps {
				fs := auth.Hooks.SVCF.FuncSteps[k]
				err = rs.LoadRoutesForFunction(ctx, fs, v.RouteName, projectId, "", v.Path, "", nil, nil)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					errArray = append(errArray, err.Error())
				}
			}
			if len(errArray) > 0 {
				err = errors.New(strings.Join(errArray, " , "))
				logs.WithContext(ctx).Error(err.Error())
				return
			}
			_, respErr := auth.Hooks.SVCF.Execute(r.Context(), r, 1, 1)
			srcHookFound = true
			return respErr
		}
		if !srcHookFound {
			logs.WithContext(ctx).Warn("SRC hook not defined for auth. Thus no email was triggered.")
		}
	}
	return
}

func (auth *Auth) sendWelcomeEmail(ctx context.Context, credentialIdentifier string, name string, projectId string, credentialType string) (err error) {
	logs.WithContext(ctx).Debug("sendWelcomeEmail - Start")
	trReqVars := &routes.TemplateVars{}
	if trReqVars.Vars == nil {
		trReqVars.Vars = make(map[string]interface{})
	}
	trReqVars.Vars["credential_type"] = credentialType
	trReqVars.Vars[credentialType] = credentialIdentifier
	trReqVars.Vars["name"] = name

	r := &http.Request{}
	url := url.URL{
		Scheme: "",
		Host:   "",
		Path:   "/",
	}
	r.URL = &url
	rBytes, rBytesErr := json.Marshal(trReqVars.Vars)
	if rBytesErr != nil {
		return rBytesErr
	}
	r.Body = io.NopCloser(strings.NewReader(string(rBytes)))
	h := http.Header{}
	h.Set("content-type", "application/json")
	r.Header = h
	r.Header.Set("Content-Length", strconv.Itoa(len(rBytes)))
	r.ContentLength = int64(len(rBytes))

	swefHookFound := false
	logs.WithContext(ctx).Info(auth.Hooks.SWEF.FuncGroupName)
	if auth.Hooks.SWEF.FuncGroupName != "" {
		var errArray []string
		rs := routes_store.ModuleStore{}
		for k, v := range auth.Hooks.SWEF.FuncSteps {
			fs := auth.Hooks.SWEF.FuncSteps[k]
			err = rs.LoadRoutesForFunction(ctx, fs, v.RouteName, projectId, "", v.Path, "", nil, nil)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				errArray = append(errArray, err.Error())
			}
		}
		if len(errArray) > 0 {
			err = errors.New(strings.Join(errArray, " , "))
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		_, respErr := auth.Hooks.SWEF.Execute(r.Context(), r, 1, 1)
		swefHookFound = true
		return respErr
	}
	if !swefHookFound {
		logs.WithContext(ctx).Warn("SWEF hook not defined for auth")
	}
	return
}

func (auth *Auth) CompleteRecovery(ctx context.Context, recoveryPassword RecoveryPassword, cookies []*http.Cookie) (msg string, err error) {
	err = errors.New("CompleteRecovery Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return "", err
}

func (auth *Auth) VerifyRecovery(ctx context.Context, recoveryPassword RecoveryPassword) (res map[string]string, cookies []*http.Cookie, err error) {
	err = errors.New("VerifyRecovery Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, nil, err
}

func (auth *Auth) VerifyToken(ctx context.Context, tokenType string, token string) (res interface{}, err error) {
	err = errors.New("VerifyToken Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func (auth *Auth) PerformPreSaveTask(ctx context.Context) (err error) {
	err = errors.New("PerformPreSaveTask Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}
func (auth *Auth) PerformPreDeleteTask(ctx context.Context) (err error) {
	err = errors.New("PerformPreDeleteTask Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}

func (auth *Auth) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "AuthType":
		return auth.AuthType, nil
	case "AuthName":
		return auth.AuthName, nil
	case "TokenHeaderKey":
		return auth.TokenHeaderKey, nil
	default:
		err := errors.New("Attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (auth *Auth) GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error) {
	err = errors.New("GetUserInfo Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return Identity{}, err
}

func (auth *Auth) GetUser(ctx context.Context, userId string) (identity Identity, err error) {
	err = errors.New("GetUser Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return Identity{}, err
}

func (auth *Auth) UpdateUser(ctx context.Context, identityToUpdate Identity, userId string, token map[string]interface{}) (err error) {
	err = errors.New("UpdateUser Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}

func (auth *Auth) FetchTokens(ctx context.Context, refresh_token string, userId string) (res interface{}, err error) {
	err = errors.New("FetchTokens Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func (auth *Auth) Login(ctx context.Context, loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {
	err = errors.New("Login Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return Identity{}, LoginSuccess{}, err
}

func (auth *Auth) Register(ctx context.Context, registerUser RegisterUser, projectId string) (identity Identity, loginSuccess LoginSuccess, err error) {
	err = errors.New("Register Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return Identity{}, LoginSuccess{}, err
}

func (auth *Auth) Logout(ctx context.Context, req *http.Request) (res interface{}, resStatusCode int, err error) {
	err = errors.New("Logout Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, 400, err
}

func (auth *Auth) ChangePassword(ctx context.Context, tokenObj map[string]interface{}, userId string, changePasswordObj ChangePassword) (err error) {
	err = errors.New("ChangePassword Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return err
}

func GetAuth(authType string) AuthI {
	switch authType {
	case "KRATOS-HYDRA":
		return new(KratosHydraAuth)
	case "MICROSOFT":
		return new(MsAuth)
	case "ERU":
		return new(EruAuth)
	default:
		return new(Auth)
	}
	return nil
}

func getTokenAttributes(ctx context.Context, token map[string]interface{}) (tokenObj map[string]interface{}, tokenErr bool) {
	if tokenIdentity, tokenIdentityOk := token["identity"]; tokenIdentityOk {
		if tokenIdentityMap, tokenIdentityMapOk := tokenIdentity.(map[string]interface{}); tokenIdentityMapOk {
			if tokenAttrs, tokenAttrsOk := tokenIdentityMap["attributes"]; tokenAttrsOk {
				if tokenAttrsMap, tokenAttrsMapOK := tokenAttrs.(map[string]interface{}); tokenAttrsMapOK {
					tokenObj = tokenAttrsMap
				} else {
					logs.WithContext(ctx).Error("token attributes is not a map")
					tokenErr = true
				}
			} else {
				logs.WithContext(ctx).Error("token attributes not found")
				tokenErr = true
			}
		} else {
			logs.WithContext(ctx).Error("token identity is not a map")
			tokenErr = true
		}
	} else {
		logs.WithContext(ctx).Error("token identity not found")
		tokenErr = true
	}
	return
}

func (auth *Auth) generateOtp(ctx context.Context, identity_credential string, identity_credential_type string, purpose string, silentFlag bool) (otp string, err error) {
	logs.WithContext(ctx).Debug("generateOtp - Start")
	if silentFlag {
		otp = "777777"
	} else {
		otp = fmt.Sprint(rand.Intn(999999-100000) + 100000)
	}
	otpQuery := models.Queries{}
	otpQuery.Query = auth.AuthDb.GetDbQuery(ctx, INSERT_OTP)
	otpQuery.Vals = append(otpQuery.Vals, uuid.New().String(), otp, identity_credential, identity_credential_type, purpose)
	otpQuery.Rank = 1

	_, err = utils.ExecuteDbFetch(ctx, auth.AuthDb.GetConn(), otpQuery)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", errors.New("something went wrong - please try again")
	}
	return
}

func (auth *Auth) GenerateVerifyCode(ctx context.Context, verifyIdentifier VerifyPostBody, projectId string, silentFlag bool) (msg string, err error) {
	logs.WithContext(ctx).Debug("GenerateVerifyCode - Start")
	logs.WithContext(ctx).Info("GenerateVerifyCode Method not implemented")
	return
}

func (auth *Auth) VerifyCode(ctx context.Context, verifyCode VerifyCode, tokenObj map[string]interface{}, withToken bool) (res interface{}, err error) {
	logs.WithContext(ctx).Debug("VerifyCode - Start")
	logs.WithContext(ctx).Info("VerifyCode Method not implemented")
	return
}
func (auth *Auth) RemoveUser(ctx context.Context, removeUser RemoveUser) (err error) {
	logs.WithContext(ctx).Debug("RemoveUser - Start")
	logs.WithContext(ctx).Info("RemoveUser Method not implemented")
	return
}
