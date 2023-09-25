package auth

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	erusha "github.com/eru-tech/eru/eru-crypto/sha"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/uuid"
	"sort"
	"strings"
)

type EruAuth struct {
	Auth
	EruConfig EruConfig   `json:"eruConfig" eru:"required"`
	Hydra     HydraConfig `json:"hydra" eru:"required"`
}

type EruConfig struct {
	Identifiers Identifiers `json:"identifiers" eru:"required"`
}

func (eruAuth *EruAuth) Register(ctx context.Context, registerUser RegisterUser) (identity Identity, loginSuccess LoginSuccess, err error) {
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
		if registerUser.Email != "" {
			userTraits.Email = registerUser.Email
			identity.Attributes["email"] = userTraits.Email

			insertQueryIcEmail := models.Queries{}
			insertQueryIcEmail.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
			insertQueryIcEmail.Vals = append(insertQueryIcEmail.Vals, uuid.New().String(), identity.Id, userTraits.Email, "email")
			insertQueryIcEmail.Rank = 2

			insertQueries = append(insertQueries, &insertQueryIcEmail)

			identifierFound = true
		} else {
			requiredIdentifiers = append(requiredIdentifiers, "email")
		}
	}
	if eruAuth.EruConfig.Identifiers.Mobile.Enable {
		if registerUser.Mobile != "" {
			userTraits.Mobile = registerUser.Mobile
			identity.Attributes["mobile"] = userTraits.Mobile

			insertQueryIcMobile := models.Queries{}
			insertQueryIcMobile.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
			insertQueryIcMobile.Vals = append(insertQueryIcMobile.Vals, uuid.New().String(), identity.Id, userTraits.Mobile, "mobile")
			insertQueryIcMobile.Rank = 3
			insertQueries = append(insertQueries, &insertQueryIcMobile)

			identifierFound = true
		} else {
			requiredIdentifiers = append(requiredIdentifiers, "mobile")
		}
	}
	if eruAuth.EruConfig.Identifiers.Username.Enable {
		if registerUser.Username != "" {
			identifierFound = true
			userTraits.Username = registerUser.Username
			identity.Attributes["userName"] = userTraits.Username

			insertQueryIcUsername := models.Queries{}
			insertQueryIcUsername.Query = eruAuth.AuthDb.GetDbQuery(ctx, INSERT_IDENTITY_CREDENTIALS)
			insertQueryIcUsername.Vals = append(insertQueryIcUsername.Vals, uuid.New().String(), identity.Id, userTraits.Username, "userName")
			insertQueryIcUsername.Rank = 4
			insertQueries = append(insertQueries, &insertQueryIcUsername)

			identifierFound = true
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
	identity.Attributes["firstName"] = userTraits.FirstName

	userTraits.LastName = registerUser.LastName
	identity.Attributes["lastName"] = userTraits.LastName

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
	identity.Attributes["idpSub"] = identity.Id
	identity.Attributes["emailVerified"] = false
	identity.Attributes["mobileVerified"] = false

	userAttrs["sub"] = identity.Id
	userAttrs["idp"] = eruAuth.AuthName
	userAttrs["idpSub"] = identity.Id

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
	passwordHash := hex.EncodeToString(erusha.NewSHA512([]byte(registerUser.Password)))
	logs.WithContext(ctx).Info(registerUser.Password)
	logs.WithContext(ctx).Info(passwordHash)
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
	loginChallenge, loginChallengeCookies, loginChallengeErr := eruAuth.Hydra.GetLoginChallenge(ctx)
	if loginChallengeErr != nil {
		err = loginChallengeErr
		return
	}

	consentChallenge, loginAcceptRequestCookies, loginAcceptErr := eruAuth.Hydra.AcceptLoginRequest(ctx, identity.Id, loginChallenge, loginChallengeCookies)
	if loginAcceptErr != nil {
		err = loginAcceptErr
		return
	}
	identityHolder := make(map[string]interface{})
	identityHolder["identity"] = identity
	eruTokens, cosentAcceptErr := eruAuth.Hydra.AcceptConsentRequest(ctx, identityHolder, consentChallenge, loginAcceptRequestCookies)
	if cosentAcceptErr != nil {
		err = cosentAcceptErr
		return
	}
	return identity, eruTokens, nil
}

func (eruAuth *EruAuth) GetUserInfo(ctx context.Context, access_token string) (identity Identity, err error) {
	logs.WithContext(ctx).Debug("GetUserInfo - Start")
	return eruAuth.Hydra.GetUserInfo(ctx, access_token)
}

func (eruAuth *EruAuth) PerformPreSaveTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreSaveTask - Start")
	// Do Nothing
	return
}

func (eruAuth *EruAuth) PerformPreDeleteTask(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("PerformPreSaveTask - Start")
	// Do Nothing
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

func (eruAuth *EruAuth) UpdateUser(ctx context.Context, identity Identity, userId string, token map[string]interface{}) (err error) {
	logs.WithContext(ctx).Debug("UpdateUser - Start")
	userTraits := UserTraits{}
	userAttrs := make(map[string]interface{})
	var queries []*models.Queries
	tokenAttributes, tokenErr := getTokenAttributes(ctx, token)

	if tokenErr {
		return errors.New("User not found")
	}

	logs.WithContext(ctx).Info(fmt.Sprint(tokenAttributes))
	for k, v := range tokenAttributes {
		logs.WithContext(ctx).Info(fmt.Sprint(k, " ", v))
		if k == "email" {
			if emailAttr, emailAttrOk := identity.Attributes["email"]; emailAttrOk {
				logs.WithContext(ctx).Info(fmt.Sprint(k, " ", v))
				if v != emailAttr {
					if v != "" {
						delQuery := models.Queries{}
						delQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, DELETE_IDENTITY_CREDENTIALS)
						delQuery.Vals = append(delQuery.Vals, identity.Id, k)
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
			}
		}

		if k == "mobile" {
			if mobileAttr, mobileAttrOk := identity.Attributes["mobile"]; mobileAttrOk {
				if v != mobileAttr {
					if v != "" {
						delQuery := models.Queries{}
						delQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, DELETE_IDENTITY_CREDENTIALS)
						delQuery.Vals = append(delQuery.Vals, identity.Id, k)
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
			}
		}

		if k == "userName" {
			if userNameAttr, userNameAttrOk := identity.Attributes["userName"]; userNameAttrOk {
				if v != userNameAttr {
					if v != "" {
						delQuery := models.Queries{}
						delQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, DELETE_IDENTITY_CREDENTIALS)
						delQuery.Vals = append(delQuery.Vals, identity.Id, k)
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
			}
		}
		if k == "firstName" {
			if firstNameAttr, firstNameAttrOk := identity.Attributes["firstName"]; firstNameAttrOk {
				userTraits.FirstName = firstNameAttr.(string)
			} else {
				userTraits.FirstName = v.(string)
			}
		}
		if k == "lastName" {
			if lastNameAttr, lastNameAttrOk := identity.Attributes["lastName"]; lastNameAttrOk {
				userTraits.LastName = lastNameAttr.(string)
			} else {
				userTraits.LastName = v.(string)
			}
		}
		if k == "emailVerified" {
			if emailVerifiedAttr, emailVerifiedAttrOk := identity.Attributes["emailVerified"]; emailVerifiedAttrOk {
				userTraits.EmailVerified = emailVerifiedAttr.(bool)
			} else {
				userTraits.EmailVerified = v.(bool)
			}
		}
		if k == "mobileVerified" {
			if mobileVerifiedAttr, mobileVerifiedAttrOk := identity.Attributes["mobileVerified"]; mobileVerifiedAttrOk {
				userTraits.MobileVerified = mobileVerifiedAttr.(bool)
			} else {
				userTraits.MobileVerified = v.(bool)
			}
		}
		userAttrs[k] = v
	}

	for k, v := range identity.Attributes {
		userAttrs[k] = v
	}

	userTraitsBytes, userTraitsBytesErr := json.Marshal(userTraits)
	if userTraitsBytesErr != nil {
		err = userTraitsBytesErr
		logs.WithContext(ctx).Error(err.Error())
		return errors.New("something went wrong - please try again")
	}

	userAttrsBytes, userAttrsBytesErr := json.Marshal(userAttrs)
	if userAttrsBytesErr != nil {
		err = userAttrsBytesErr
		logs.WithContext(ctx).Error(err.Error())
		return errors.New("something went wrong - please try again")
	}

	updateQuery := models.Queries{}
	updateQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, UPDATE_IDENTITY)
	updateQuery.Vals = append(updateQuery.Vals, string(userTraitsBytes), string(userAttrsBytes), identity.Id)
	updateQuery.Rank = 7
	queries = append(queries, &updateQuery)

	sort.Sort(models.QueriesSorter(queries))

	insertOutput, err := utils.ExecuteDbSave(ctx, eruAuth.AuthDb.GetConn(), queries)
	logs.WithContext(ctx).Info(fmt.Sprint(insertOutput))
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return errors.New("something went wrong - please try again")
	}
	return
}

func (eruAuth *EruAuth) Login(ctx context.Context, loginPostBody LoginPostBody, withTokens bool) (identity Identity, loginSuccess LoginSuccess, err error) {
	logs.WithContext(ctx).Debug("Login - Start")

	loginQuery := models.Queries{}
	loginQuery.Query = eruAuth.AuthDb.GetDbQuery(ctx, SELECT_LOGIN)
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
		return Identity{}, LoginSuccess{}, err
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
		loginChallenge, loginChallengeCookies, loginChallengeErr := eruAuth.Hydra.GetLoginChallenge(ctx)
		if loginChallengeErr != nil {
			err = loginChallengeErr
			return
		}

		consentChallenge, loginAcceptRequestCookies, loginAcceptErr := eruAuth.Hydra.AcceptLoginRequest(ctx, identity.Id, loginChallenge, loginChallengeCookies)
		if loginAcceptErr != nil {
			err = loginAcceptErr
			return
		}
		identityHolder := make(map[string]interface{})
		identityHolder["identity"] = identity
		eruTokens, cosentAcceptErr := eruAuth.Hydra.AcceptConsentRequest(ctx, identityHolder, consentChallenge, loginAcceptRequestCookies)
		if cosentAcceptErr != nil {
			err = cosentAcceptErr
			return
		}
		return identity, eruTokens, nil
	}
	return identity, LoginSuccess{}, nil
}
