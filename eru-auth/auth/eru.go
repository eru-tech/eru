package auth

import (
	"context"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	erusha "github.com/eru-tech/eru/eru-crypto/sha"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/uuid"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	SELECT_IDENTITY_SUB               = "select * from eruauth_identities where identity_provider_id = ??? or traits->>'email'= ???"
	INSERT_IDENTITY                   = "insert into eruauth_identities (identity_id,identity_provider,identity_provider_id,traits,attributes) values (???,???,???,???,???)"
	UPDATE_IDENTITY                   = "update eruauth_identities set traits = ??? , attributes = ??? where identity_id = ???"
	INSERT_IDENTITY_CREDENTIALS       = "insert into eruauth_identity_credentials (identity_credential_id , identity_id, identity_credential, identity_credential_type) values (???,???,???,???)"
	DELETE_IDENTITY_CREDENTIALS       = "delete from eruauth_identity_credentials where identity_id = ??? and identity_credential_type = ??? and identity_credential = ??? "
	DELETE_IDENTITY_CREDENTIALS_BY_ID = "delete from eruauth_identity_credentials where identity_id = ??? "
	INSERT_IDENTITY_PASSWORD          = "insert into eruauth_identity_passwords (identity_password_id,identity_id,identity_password) values (??? , ??? , ???) on conflict ON CONSTRAINT unique_identity_id do update set identity_password=EXCLUDED.identity_password"
	SELECT_LOGIN                      = "select a.* , case when is_active=true then 'Active' else 'Inactive' end status from eruauth_identities a inner join eruauth_identity_credentials b on a.identity_id=b.identity_id and lower(b.identity_credential) = lower(???) inner join eruauth_identity_passwords c on a.identity_id=c.identity_id and c.identity_password= ???"
	SELECT_LOGIN_ID                   = "select a.* , case when is_active=true then 'Active' else 'Inactive' end status from eruauth_identities a inner join eruauth_identity_passwords c on a.identity_id=c.identity_id and c.identity_password= ??? where a.identity_id= ???"
	SELECT_IDENTITY                   = "select a.* , case when is_active=true then 'Active' else 'Inactive' end status from eruauth_identities a  where a.identity_id = ???"
	SELECT_IDENTITY_CREDENTIAL        = "select b.traits->>'first_name' first_name , a.* from eruauth_identity_credentials a left join eruauth_identities b on a.identity_id=b.identity_id where a.identity_credential = ???"
	INSERT_OTP                        = "insert into eruauth_otp (otp_id, otp, identity_credential,identity_credential_type,otp_purpose) values (??? , ??? , ???,??? , ???)"
	VERIFY_OTP                        = "select b.identity_id, a.* from eruauth_otp a left join eruauth_identity_credentials b on a.identity_credential=b.identity_credential where identity_id = ??? and otp = ??? and a.identity_credential = ??? and a.created_date + (5 * interval '1 minute') >= LOCALTIMESTAMP and otp_purpose = ???"
	VERIFY_RECOVERY_OTP               = "select b.identity_id, a.* from eruauth_otp a left join eruauth_identity_credentials b on a.identity_credential=b.identity_credential where otp = ??? and a.identity_credential = ??? and a.created_date + (5 * interval '1 minute') >= LOCALTIMESTAMP and otp_purpose = ???"
	CHANGE_PASSWORD                   = "update eruauth_identity_passwords set updated_date=LOCALTIMESTAMP, identity_password= ??? where identity_id= ???"
	INSERT_DELETED_IDENTITY           = "insert into eruauth_deleted_identities (identity_id,identity_provider,identity_provider_id,traits,attributes,is_active,identity_password) select a.identity_id,identity_provider,identity_provider_id,traits,attributes,is_active, b.identity_password  from eruauth_identities a left join eruauth_identity_passwords b on a.identity_id=b.identity_id where a.identity_id= ???"
	DELETE_IDENTITY_PASSWORD          = "delete from eruauth_identity_passwords where identity_id= ???"
	DELETE_IDENTITY                   = "delete from eruauth_identities where identity_id= ???"
	ERU_LOGIN_FALLBACK                = "with i as (select c.config->>'hashed_password' hp, a.* , case when is_active=true then 'Active' else 'Inactive' end status from eruauth_identities a inner join eruauth_identity_credentials b on a.identity_id=b.identity_id and lower(b.identity_credential) = lower(???) inner join ory_identity_credentials c on a.identity_id=c.identity_id::text) select crypt(???,hp) = hp pmatch, i.* from i"
)

type EruAuth struct {
	Auth
	EruConfig EruConfig `json:"eru_config" eru:"required"`
	//Hydra     HydraConfig `json:"hydra" eru:"required"`
}

type EruConfig struct {
	LoginFallback bool        `json:"login_fallback"`
	Identifiers   Identifiers `json:"identifiers" eru:"required"`
}

func (eruAuth *EruAuth) Register(ctx context.Context, registerUser RegisterUser, projectId string) (identity Identity, tokens LoginSuccess, err error) {
	logs.WithContext(ctx).Debug("Register - Start")

	if registerUser.Password == "" {
		err = errors.New("password is mandatory while registering user")
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, LoginSuccess{}, err
	}

	userTraits := UserTraits{}
	userAttrs := make(map[string]string)

	if identity.Attributes == nil {
		identity.Attributes = make(map[string]interface{})
	}
	identifierFound := false
	var requiredIdentifiers []string
	var insertQueries []*models.Queries
	identity.Id = uuid.New().String()

	if eruAuth.EruConfig.Identifiers.Email.Enable {
		userTraits.Email = registerUser.Email
		identity.Attributes["email"] = userTraits.Email
		if registerUser.Email != "" {
			identifierFound = true
			insertQueryIcEmail := models.Queries{}
			insertQueryIcEmail.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
			insertQueryIcEmail.Vals = append(insertQueryIcEmail.Vals, uuid.New().String(), identity.Id, userTraits.Email, "email")
			insertQueryIcEmail.Rank = 2

			insertQueries = append(insertQueries, &insertQueryIcEmail)
		} else {
			requiredIdentifiers = append(requiredIdentifiers, "email")
		}
	}
	if eruAuth.EruConfig.Identifiers.Mobile.Enable {
		userTraits.Mobile = registerUser.Mobile
		identity.Attributes["mobile"] = userTraits.Mobile
		if registerUser.Mobile != "" {
			identifierFound = true
			insertQueryIcMobile := models.Queries{}
			insertQueryIcMobile.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
			insertQueryIcMobile.Vals = append(insertQueryIcMobile.Vals, uuid.New().String(), identity.Id, userTraits.Mobile, "mobile")
			insertQueryIcMobile.Rank = 3
			insertQueries = append(insertQueries, &insertQueryIcMobile)
		} else {
			requiredIdentifiers = append(requiredIdentifiers, "mobile")
		}
	}
	if eruAuth.EruConfig.Identifiers.Username.Enable {
		userTraits.Username = registerUser.Username
		identity.Attributes["username"] = userTraits.Username
		if registerUser.Username != "" {
			identifierFound = true
			insertQueryIcUsername := models.Queries{}
			insertQueryIcUsername.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
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
	identity.Attributes["idp"] = eruAuth.AuthName
	identity.Attributes["idp_sub"] = identity.Id
	identity.Attributes["email_verified"] = false
	identity.Attributes["mobile_verified"] = false

	userAttrs["sub"] = identity.Id
	userAttrs["idp"] = eruAuth.AuthName
	userAttrs["idp_sub"] = identity.Id

	userAttrsBytes, userAttrsBytesErr := json.Marshal(userAttrs)
	if userAttrsBytesErr != nil {
		err = userAttrsBytesErr
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	insertQuery := models.Queries{}
	insertQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY)
	insertQuery.Vals = append(insertQuery.Vals, identity.Id, eruAuth.AuthName, identity.Id, string(userTraitsBytes), string(userAttrsBytes))
	insertQuery.Rank = 1
	insertQueries = append(insertQueries, &insertQuery)

	insertPQuery := models.Queries{}
	insertPQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_PASSWORD)

	passwordBytes, passwordErr := b64.StdEncoding.DecodeString(registerUser.Password)
	if passwordErr != nil {
		logs.WithContext(ctx).Error(passwordErr.Error())
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}
	passwordHash := hex.EncodeToString(erusha.NewSHA512(passwordBytes))

	insertPQuery.Vals = append(insertPQuery.Vals, uuid.New().String(), identity.Id, passwordHash)
	insertPQuery.Rank = 5
	insertQueries = append(insertQueries, &insertPQuery)

	sort.Sort(models.QueriesSorter(insertQueries))

	insertOutput, err := utils.ExecuteDbSave(ctx, eruAuth.AuthDb.GetConn(), insertQueries)
	logs.WithContext(ctx).Info(fmt.Sprint(insertOutput))
	if err != nil {
		if strings.Contains(err.Error(), "unique_identity_credential") {
			return Identity{}, LoginSuccess{}, errors.New("username already exists")
		}
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	tokens, err = eruAuth.makeTokens(ctx, identity)

	//loginChallenge, loginChallengeCookies, loginChallengeErr := eruAuth.Hydra.GetLoginChallenge(ctx)
	//if loginChallengeErr != nil {
	//	err = loginChallengeErr
	//	return
	//}
	//
	//consentChallenge, loginAcceptRequestCookies, loginAcceptErr := eruAuth.Hydra.AcceptLoginRequest(ctx, identity.Id, loginChallenge, loginChallengeCookies)
	//if loginAcceptErr != nil {
	//	err = loginAcceptErr
	//	return
	//}
	//identityHolder := make(map[string]interface{})
	//identityHolder["identity"] = identity
	//eruTokens, cosentAcceptErr := eruAuth.Hydra.AcceptConsentRequest(ctx, identityHolder, consentChallenge, loginAcceptRequestCookies)
	//if cosentAcceptErr != nil {
	//	err = cosentAcceptErr
	//	return
	//}

	if eruAuth.Hooks.SWEF != "" {
		eruAuth.sendWelcomeEmail(ctx, userTraits.Email, userTraits.FirstName, projectId, "email")
	} else {
		logs.WithContext(ctx).Info("SWEF hook not defined")
	}

	return identity, tokens, nil
}

func (eruAuth *EruAuth) PerformPreSaveTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreSaveTask - Start")
	for _, v := range eruAuth.Hydra.HydraClients {
		err = eruAuth.Hydra.SaveHydraClient(ctx, v)
		if err != nil {
			return err
		}
	}
	return
}

func (eruAuth *EruAuth) PerformPreDeleteTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreDeleteTask - Start")
	for _, v := range eruAuth.Hydra.HydraClients {
		err = eruAuth.Hydra.RemoveHydraClient(ctx, v.ClientId)
		if err != nil {
			return err
		}
	}
	return
}

func (eruAuth *EruAuth) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &eruAuth)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (eruAuth *EruAuth) UpdateUser(ctx context.Context, identity Identity, userId string, token map[string]interface{}) (tokens interface{}, err error) {
	logs.WithContext(ctx).Debug("UpdateUser - Start")
	userTraits := UserTraits{}
	var userTraitsArray []string
	userAttrs := make(map[string]interface{})
	var queries []*models.Queries
	tokenAttributes, tokenErr := getTokenAttributes(ctx, token)

	if tokenErr {
		return LoginSuccess{}, errors.New("User not found")
	}

	logs.WithContext(ctx).Info(fmt.Sprint(tokenAttributes))
	for k, v := range tokenAttributes {
		userTraitsFound := false
		if k == "email" {
			userTraitsFound = true
			userTraitsArray = append(userTraitsArray, k)
			if emailAttr, emailAttrOk := identity.Attributes["email"]; emailAttrOk {
				logs.WithContext(ctx).Info(fmt.Sprint(k, " ", v))
				if v != emailAttr {
					if v != "" {
						delQuery := models.Queries{}
						delQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, DELETE_IDENTITY_CREDENTIALS)
						delQuery.Vals = append(delQuery.Vals, identity.Id, k, v)
						delQuery.Rank = 1
						queries = append(queries, &delQuery)
					}

					userTraits.Email = emailAttr.(string)
					insertQueryIcEmail := models.Queries{}
					insertQueryIcEmail.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
					insertQueryIcEmail.Vals = append(insertQueryIcEmail.Vals, uuid.New().String(), identity.Id, userTraits.Email, "email")
					insertQueryIcEmail.Rank = 2

					queries = append(queries, &insertQueryIcEmail)
				} else {
					userTraits.Email = emailAttr.(string)
				}
			} else {
				userTraits.Email = v.(string)
			}
		}

		if k == "mobile" {
			logs.WithContext(ctx).Info(fmt.Sprint("inside mobile for = ", v))
			userTraitsFound = true
			userTraitsArray = append(userTraitsArray, k)
			if mobileAttr, mobileAttrOk := identity.Attributes["mobile"]; mobileAttrOk {
				logs.WithContext(ctx).Info(fmt.Sprint("mobileAttr = ", mobileAttr))
				if v != mobileAttr {
					logs.WithContext(ctx).Info(fmt.Sprint(v, "!=", mobileAttr))
					if v != "" {
						logs.WithContext(ctx).Info("inside v != \"\"")
						delQuery := models.Queries{}
						delQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, DELETE_IDENTITY_CREDENTIALS)
						delQuery.Vals = append(delQuery.Vals, identity.Id, k, v)
						delQuery.Rank = 3
						queries = append(queries, &delQuery)
					}
					userTraits.Mobile = mobileAttr.(string)
					insertQueryIcMobile := models.Queries{}
					insertQueryIcMobile.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
					insertQueryIcMobile.Vals = append(insertQueryIcMobile.Vals, uuid.New().String(), identity.Id, userTraits.Mobile, "mobile")
					insertQueryIcMobile.Rank = 4
					queries = append(queries, &insertQueryIcMobile)
				} else {
					userTraits.Mobile = mobileAttr.(string)
				}
			} else {
				userTraits.Mobile = v.(string)
			}
		}

		if k == "username" {
			userTraitsFound = true
			userTraitsArray = append(userTraitsArray, k)
			if userNameAttr, userNameAttrOk := identity.Attributes["username"]; userNameAttrOk {
				if v != userNameAttr {
					if v != "" {
						delQuery := models.Queries{}
						delQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, DELETE_IDENTITY_CREDENTIALS)
						delQuery.Vals = append(delQuery.Vals, identity.Id, k, v)
						delQuery.Rank = 5
						queries = append(queries, &delQuery)
					}

					userTraits.Username = userNameAttr.(string)
					insertQueryIcUsername := models.Queries{}
					insertQueryIcUsername.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
					insertQueryIcUsername.Vals = append(insertQueryIcUsername.Vals, uuid.New().String(), identity.Id, userTraits.Username, "userName")
					insertQueryIcUsername.Rank = 6

					queries = append(queries, &insertQueryIcUsername)
				} else {
					userTraits.Username = userNameAttr.(string)
				}
			} else {
				userTraits.Username = v.(string)
			}
		}
		if k == "first_name" {
			userTraitsFound = true
			userTraitsArray = append(userTraitsArray, k)
			if firstNameAttr, firstNameAttrOk := identity.Attributes["first_name"]; firstNameAttrOk {
				userTraits.FirstName = firstNameAttr.(string)
			} else {
				userTraits.FirstName = v.(string)
			}
		}
		if k == "last_name" {
			userTraitsFound = true
			userTraitsArray = append(userTraitsArray, k)
			if lastNameAttr, lastNameAttrOk := identity.Attributes["last_name"]; lastNameAttrOk {
				userTraits.LastName = lastNameAttr.(string)
			} else {
				userTraits.LastName = v.(string)
			}
		}
		if k == "email_verified" {
			userTraitsFound = true
			userTraitsArray = append(userTraitsArray, k)
			if emailVerifiedAttr, emailVerifiedAttrOk := identity.Attributes["email_verified"]; emailVerifiedAttrOk {
				userTraits.EmailVerified = emailVerifiedAttr.(bool)
			} else {
				userTraits.EmailVerified = v.(bool)
			}
		}
		if k == "mobile_verified" {
			userTraitsFound = true
			userTraitsArray = append(userTraitsArray, k)
			if mobileVerifiedAttr, mobileVerifiedAttrOk := identity.Attributes["mobile_verified"]; mobileVerifiedAttrOk {
				userTraits.MobileVerified = mobileVerifiedAttr.(bool)
			} else {
				userTraits.MobileVerified = v.(bool)
			}
		}
		if !userTraitsFound {
			userAttrs[k] = v
		}
	}
	for k, v := range identity.Attributes {
		kFound := false
		for _, ut := range userTraitsArray {
			if ut == k {
				kFound = true
				break
			}
		}
		if !kFound {
			userAttrs[k] = v
		}
	}

	userTraitsBytes, userTraitsBytesErr := json.Marshal(userTraits)
	if userTraitsBytesErr != nil {
		err = userTraitsBytesErr
		logs.WithContext(ctx).Error(err.Error())
		return LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	userAttrsBytes, userAttrsBytesErr := json.Marshal(userAttrs)
	if userAttrsBytesErr != nil {
		err = userAttrsBytesErr
		logs.WithContext(ctx).Error(err.Error())
		return LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	updateQuery := models.Queries{}
	updateQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, UPDATE_IDENTITY)
	updateQuery.Vals = append(updateQuery.Vals, string(userTraitsBytes), string(userAttrsBytes), identity.Id)
	updateQuery.Rank = 7
	queries = append(queries, &updateQuery)

	sort.Sort(models.QueriesSorter(queries))
	for _, v := range queries {
		logs.WithContext(ctx).Info(fmt.Sprint(v))
	}
	insertOutput, err := utils.ExecuteDbSave(ctx, eruAuth.AuthDb.GetConn(), queries)
	logs.WithContext(ctx).Info(fmt.Sprint(insertOutput))
	if err != nil {
		if strings.Contains(err.Error(), "unique_identity_credential") {
			return LoginSuccess{}, errors.New("user with same credentials already exists")
		}
		logs.WithContext(ctx).Error(err.Error())
		return LoginSuccess{}, errors.New("something went wrong - please try again")
	}
	tokens, err = eruAuth.makeTokens(ctx, identity)

	return
}

func (eruAuth *EruAuth) Login(ctx context.Context, loginPostBody LoginPostBody, projectId string, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {
	logs.WithContext(ctx).Info("Login - Start")

	loginQuery := models.Queries{}
	loginQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, SELECT_LOGIN)
	orgPassword := ""
	if eruAuth.EruConfig.LoginFallback {
		passwordBytes, passwordErr := b64.StdEncoding.DecodeString(loginPostBody.Password)
		if passwordErr != nil {
			logs.WithContext(ctx).Error(passwordErr.Error())
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		}
		loginPostBody.Password = hex.EncodeToString(erusha.NewSHA512(passwordBytes))
		orgPassword = string(passwordBytes)
	}

	loginQuery.Vals = append(loginQuery.Vals, loginPostBody.Username, loginPostBody.Password)
	loginQuery.Rank = 1
	loginOutput, err := utils.ExecuteDbFetch(ctx, eruAuth.AuthDb.GetConn(), loginQuery)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
	}

	if len(loginOutput) == 0 {
		err = errors.New("invalid credentials - please try again")
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Info(fmt.Sprint("eruAuth.EruConfig.LoginFallback  = ", eruAuth.EruConfig.LoginFallback))
		if !eruAuth.EruConfig.LoginFallback {
			return Identity{}, LoginSuccess{}, err
		} else {
			err = nil
			logs.WithContext(ctx).Info("executing login fallback query")
			loginQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, ERU_LOGIN_FALLBACK)
			loginQuery.Vals = nil
			loginQuery.Vals = append(loginQuery.Vals, loginPostBody.Username, orgPassword)
			loginOutput, err = utils.ExecuteDbFetch(ctx, eruAuth.AuthDb.GetConn(), loginQuery)
		}
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		}
		if len(loginOutput) == 0 {
			err = errors.New("invalid credentials - please try again")
			logs.WithContext(ctx).Error(fmt.Sprint("fallback : ", err.Error()))
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		}
		pMatch := loginOutput[0]["pmatch"].(bool)
		if !pMatch {
			err = errors.New("invalid credentials - please try again")
			logs.WithContext(ctx).Error(fmt.Sprint("fallback match : ", err.Error()))
			return Identity{}, LoginSuccess{}, errors.New("something went wrong - please try again")
		} else {
			var insertQueries []*models.Queries
			insertPQuery := models.Queries{}
			insertPQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_PASSWORD)
			insertPQuery.Vals = append(insertPQuery.Vals, uuid.New().String(), loginOutput[0]["identity_id"].(string), loginPostBody.Password)
			insertPQuery.Rank = 1
			insertQueries = append(insertQueries, &insertPQuery)
			_, err = utils.ExecuteDbSave(ctx, eruAuth.AuthDb.GetConn(), insertQueries)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("fallback save : ", err.Error()))
				//silently exit and proceed further
			}
		}
	}

	identity.Id = loginOutput[0]["identity_id"].(string)
	identity.Status = loginOutput[0]["status"].(string)
	identity.Attributes = make(map[string]interface{})

	if attrs, attrsOk := loginOutput[0]["attributes"].(*map[string]interface{}); attrsOk {
		for k, v := range *attrs {
			identity.Attributes[k] = v
		}
	}
	if traits, traitsOk := loginOutput[0]["traits"].(*map[string]interface{}); traitsOk {
		for k, v := range *traits {
			identity.Attributes[k] = v
		}
	}
	if withTokens {
		eruTokens, eruTokensErr := eruAuth.makeTokens(ctx, identity)
		return identity, eruTokens, eruTokensErr
	}
	return identity, LoginSuccess{}, nil
}

func (eruAuth *EruAuth) GenerateRecoveryCode(ctx context.Context, recoveryIdentifier RecoveryPostBody, projectId string, silentFlag bool) (msg string, err error) {
	logs.WithContext(ctx).Debug("GenerateRecoveryCode - Start")

	recoveryQuery := models.Queries{}
	recoveryQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, SELECT_IDENTITY_CREDENTIAL)
	recoveryQuery.Vals = append(recoveryQuery.Vals, recoveryIdentifier.Username)
	recoveryQuery.Rank = 1

	recoveryOutput, err := utils.ExecuteDbFetch(ctx, eruAuth.AuthDb.GetConn(), recoveryQuery)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", errors.New("something went wrong - please try again")
	}

	if len(recoveryOutput) == 0 {
		err = errors.New("user not found")
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	name := ""
	if fn, fnOk := recoveryOutput[0]["first_name"]; fnOk {
		name = fn.(string)
	}
	credentialType := ""
	if ct, ctOk := recoveryOutput[0]["identity_credential_type"]; ctOk {
		credentialType = ct.(string)
	}
	otp := ""
	otp, err = eruAuth.generateOtp(ctx, recoveryIdentifier.Username, credentialType, OTP_PURPOSE_RECOVERY, silentFlag)
	if !silentFlag {
		err = eruAuth.sendCode(ctx, recoveryIdentifier.Username, otp, fmt.Sprint(time.Now().Add(time.Minute*5).Format("02 Jan 06 15:04 MST")), name, projectId, OTP_PURPOSE_RECOVERY, credentialType)
	}
	if err != nil {
		return "", err
	}

	return
}

func (eruAuth *EruAuth) GenerateVerifyCode(ctx context.Context, verifyIdentifier VerifyPostBody, projectId string, silentFlag bool) (msg string, err error) {
	logs.WithContext(ctx).Debug("GenerateVerifyCode - Start")
	verifyQuery := models.Queries{}
	verifyQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, SELECT_IDENTITY_CREDENTIAL)
	verifyQuery.Vals = append(verifyQuery.Vals, verifyIdentifier.Username)
	verifyQuery.Rank = 1

	verifyOutput, err := utils.ExecuteDbFetch(ctx, eruAuth.AuthDb.GetConn(), verifyQuery)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return "", errors.New("something went wrong - please try again")
	}
	logs.WithContext(ctx).Info(fmt.Sprint(verifyOutput))
	if len(verifyOutput) == 0 {
		err = errors.New("user not found")
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	name := " "
	logs.WithContext(ctx).Info(fmt.Sprint(verifyOutput))
	if fn, fnOk := verifyOutput[0]["first_name"]; fnOk {
		name = fn.(string)
	}
	logs.WithContext(ctx).Info(fmt.Sprint("name = ", name))
	if ct, ctOk := verifyOutput[0]["identity_credential_type"]; ctOk {
		if verifyIdentifier.CredentialType != ct.(string) {
			err = errors.New("user not found")
			logs.WithContext(ctx).Error(err.Error())
			return "", err
		}
	}
	otp := ""
	otp, err = eruAuth.generateOtp(ctx, verifyIdentifier.Username, verifyIdentifier.CredentialType, OTP_PURPOSE_VERIFY, silentFlag)

	if !silentFlag {
		err = eruAuth.sendCode(ctx, verifyIdentifier.Username, otp, fmt.Sprint(time.Now().Add(time.Minute*5).Format("02 Jan 06 15:04 MST")), name, projectId, OTP_PURPOSE_VERIFY, verifyIdentifier.CredentialType)
	}
	if err != nil {
		return "", err
	}

	return
}

func (eruAuth *EruAuth) VerifyCode(ctx context.Context, verifyCode VerifyCode, tokenObj map[string]interface{}, withToken bool) (res interface{}, err error) {
	logs.WithContext(ctx).Debug("VerifyCode - Start")
	verifyQuery := models.Queries{}
	verifyQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, VERIFY_OTP)
	verifyQuery.Vals = append(verifyQuery.Vals, verifyCode.UserId, verifyCode.Code, verifyCode.Id, OTP_PURPOSE_VERIFY)
	verifyQuery.Rank = 1
	logs.WithContext(ctx).Info(fmt.Sprint(verifyQuery.Vals))
	verifyOutput, err := utils.ExecuteDbFetch(ctx, eruAuth.AuthDb.GetConn(), verifyQuery)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, errors.New("something went wrong - please try again")
	}

	if len(verifyOutput) == 0 {
		err = errors.New("code not found - please check and try again")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	credentialType := ""
	if ct, ctOk := verifyOutput[0]["identity_credential_type"]; ctOk {
		credentialType = ct.(string)
	}

	identity := Identity{}
	identity.Id = verifyCode.UserId
	identity.Attributes = make(map[string]interface{})
	if credentialType == "email" {
		identity.Attributes["email_verified"] = true
	}
	if credentialType == "mobile" {
		identity.Attributes["mobile_verified"] = true
	}
	res, err = eruAuth.UpdateUser(ctx, identity, verifyCode.UserId, tokenObj)
	if withToken {
		return res, err
	}
	return nil, err
}

func (eruAuth *EruAuth) ChangePassword(ctx context.Context, tokenObj map[string]interface{}, userId string, changePasswordObj ChangePassword) (err error) {
	logs.WithContext(ctx).Debug("ChangePassword - Start")

	loginQuery := models.Queries{}
	loginQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, SELECT_LOGIN_ID)
	loginQuery.Vals = append(loginQuery.Vals, changePasswordObj.OldPassword, userId)
	loginQuery.Rank = 1

	loginOutput, err := utils.ExecuteDbFetch(ctx, eruAuth.AuthDb.GetConn(), loginQuery)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return errors.New("something went wrong - please try again")
	}

	if len(loginOutput) == 0 {
		err = errors.New("invalid credentials - please try again")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	var queries []*models.Queries
	cpQuery := models.Queries{}
	cpQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, CHANGE_PASSWORD)
	cpQuery.Vals = append(cpQuery.Vals, changePasswordObj.NewPassword, userId)
	cpQuery.Rank = 1
	queries = append(queries, &cpQuery)

	_, err = utils.ExecuteDbSave(ctx, eruAuth.AuthDb.GetConn(), queries)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return errors.New("something went wrong - please try again")
	}
	return
}

func (eruAuth *EruAuth) VerifyRecovery(ctx context.Context, recoveryPassword RecoveryPassword) (res map[string]string, cookies []*http.Cookie, err error) {
	logs.WithContext(ctx).Debug("VerifyRecovery - Start")
	logs.WithContext(ctx).Info(fmt.Sprint(recoveryPassword))
	verifyQuery := models.Queries{}
	verifyQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, VERIFY_RECOVERY_OTP)
	verifyQuery.Vals = append(verifyQuery.Vals, recoveryPassword.Code, recoveryPassword.Id, OTP_PURPOSE_RECOVERY)
	verifyQuery.Rank = 1

	verifyOutput, err := utils.ExecuteDbFetch(ctx, eruAuth.AuthDb.GetConn(), verifyQuery)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, errors.New("something went wrong - please try again")
	}

	if len(verifyOutput) == 0 {
		err = errors.New("code not found - please check and try again")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, err
	}
	userId := ""
	if id, idOk := verifyOutput[0]["identity_id"]; idOk {
		userId = id.(string)
	} else {
		err = errors.New("user not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, err
	}

	var queries []*models.Queries
	cpQuery := models.Queries{}

	cpQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_PASSWORD)
	cpQuery.Vals = append(cpQuery.Vals, uuid.New().String(), userId, recoveryPassword.Password)
	cpQuery.Rank = 1
	queries = append(queries, &cpQuery)

	resCp, err := utils.ExecuteDbSave(ctx, eruAuth.AuthDb.GetConn(), queries)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, errors.New("something went wrong - please try again")
	}
	logs.WithContext(ctx).Info(fmt.Sprint(len(resCp)))
	res = make(map[string]string)
	res["status"] = "account recovery successful"
	return
}

func (eruAuth *EruAuth) Logout(ctx context.Context, req *http.Request) (res interface{}, resStatusCode int, err error) {
	logs.WithContext(ctx).Debug("Logout - Start")

	refreshTokenFromReq := json.NewDecoder(req.Body)
	refreshTokenFromReq.DisallowUnknownFields()
	refreshTokenObj := make(map[string]string)
	if rtErr := refreshTokenFromReq.Decode(&refreshTokenObj); rtErr != nil {
		err = rtErr
		logs.WithContext(ctx).Error(err.Error())
		resStatusCode = 400
		return
	}
	refreshToken := ""
	if rt, ok := refreshTokenObj["refresh_token"]; ok {
		refreshToken = rt
	}
	if refreshToken == "" {
		err = errors.New("refresh token not found")
		logs.WithContext(ctx).Error(err.Error())
		resStatusCode = 400
		return
	}

	resStatusCode, err = eruAuth.Hydra.revokeToken(ctx, refreshToken)
	if res == nil {
		res = make(map[string]interface{})
	}
	return res, resStatusCode, err
}
