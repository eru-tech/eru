package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"math/rand"
	"net/http"
	"strconv"
)

func UserInfoHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("UserInfoHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		userInfoFromReq := json.NewDecoder(r.Body)
		userInfoFromReq.DisallowUnknownFields()
		userInfoObj := make(map[string]interface{})
		//storageObj := new(storage.Storage)
		if err := userInfoFromReq.Decode(&userInfoObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		accessTokenStr := ""
		if accessToken, ok := userInfoObj["access_token"]; !ok {
			atErr := errors.New("access_token attribute missing in request body")
			logs.WithContext(r.Context()).Error(atErr.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": atErr})
			return
		} else {
			if accessTokenStr, ok = accessToken.(string); !ok {
				atErr := errors.New("Incorrect access_token recevied in request body")
				logs.WithContext(r.Context()).Error(atErr.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": atErr})
				return
			}
		}

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(r.Context()).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return
		}
		identity, err := authObjI.GetUserInfo(r.Context(), accessTokenStr)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(identity)
		return
	}
}

func FetchTokensHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("FetchTokensHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		fetchTokenFromReq := json.NewDecoder(r.Body)

		fetchTokenFromReq.DisallowUnknownFields()
		type fetchToken struct {
			RefreshToken string `json:"refresh_token" eru:"required"`
			Id           string `json:"id" eru:"required"`
		}
		var fetchTokenObj fetchToken

		if err := fetchTokenFromReq.Decode(&fetchTokenObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), fetchTokenObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		logs.WithContext(r.Context()).Info(fmt.Sprint(fetchTokenObj))
		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(r.Context()).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return
		}

		loginSuccess, err := authObjI.FetchTokens(r.Context(), fetchTokenObj.RefreshToken, fetchTokenObj.Id)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(loginSuccess)
		return
	}
}

func VerifyTokenHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("VerifyTokenHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		tokenType := vars["tokentype"]
		authName := vars["authname"]

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenHeaderKey, err := authObjI.GetAttribute(r.Context(), "token_header_key")
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenToVerify := r.Header.Get(tokenHeaderKey.(string))

		res, err := authObjI.VerifyToken(r.Context(), tokenType, tokenToVerify)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(res)
		return
	}
}

func LoginHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Info("LoginHandler - Start")
		ctx := context.WithValue(r.Context(), "Erufuncbaseurl", module_store.Erufuncbaseurl)
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]
		logs.WithContext(r.Context()).Info(projectId)
		logs.WithContext(r.Context()).Info(authName)
		authObjI, err := s.GetAuth(ctx, projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		logs.WithContext(r.Context()).Info(fmt.Sprint(authObjI))

		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(ctx).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return

		}
		logs.WithContext(r.Context()).Info(fmt.Sprint("auth conn is set"))
		utils.PrintRequestBody(r.Context(), r, "from login handler in eruauth")
		loginPostBodyFromReq := json.NewDecoder(r.Body)
		loginPostBodyFromReq.DisallowUnknownFields()

		var loginPostBody auth.LoginPostBody

		if err = loginPostBodyFromReq.Decode(&loginPostBody); err != nil {
			logs.WithContext(ctx).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		logs.WithContext(r.Context()).Info(fmt.Sprint(loginPostBody))
		msParams := auth.OAuthParams{}
		if authName == "ms" {
			msParams, err = s.GetPkceEvent(ctx, loginPostBody.IdpRequestId, s)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}

		loginPostBody.CodeVerifier = msParams.CodeVerifier
		loginPostBody.Nonce = msParams.Nonce
		logs.WithContext(r.Context()).Info(fmt.Sprint("before login = ", loginPostBody))
		res, tokens, err := authObjI.Login(ctx, loginPostBody, projectId, true)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, http.StatusOK)
			if tokens.IdToken != "" {
				_ = json.NewEncoder(w).Encode(tokens)
			} else {
				_ = json.NewEncoder(w).Encode(res)
			}
			return
		}
	}
}

func GetRecoveryCodeHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GetRecoveryCodeHandler - Start")
		ctx := context.WithValue(r.Context(), "Erufuncbaseurl", module_store.Erufuncbaseurl)
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		params := r.URL.Query()
		isSilentStr := params.Get("silent")
		silentFlag := false
		silentFlag, _ = strconv.ParseBool(isSilentStr)

		authObjI, err := s.GetAuth(ctx, projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(ctx).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return
		}

		recoveryReq := json.NewDecoder(r.Body)
		recoveryReq.DisallowUnknownFields()

		var recoveryPostBody auth.RecoveryPostBody

		if err = recoveryReq.Decode(&recoveryPostBody); err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		res, err := authObjI.GenerateRecoveryCode(ctx, recoveryPostBody, projectId, silentFlag)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, http.StatusOK)
			if authName == "ory" {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": res})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "code sent successfully"})
			return
		}
	}
}

func GetVerifyCodeHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GetVerifyCodeHandler - Start")
		ctx := context.WithValue(r.Context(), "Erufuncbaseurl", module_store.Erufuncbaseurl)
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		params := r.URL.Query()
		isSilentStr := params.Get("silent")
		silentFlag := false
		var silentFlagErr error
		silentFlag, silentFlagErr = strconv.ParseBool(isSilentStr)
		logs.WithContext(ctx).Info(fmt.Sprint("silentFlag = ", silentFlag))
		if silentFlagErr != nil {
			logs.WithContext(ctx).Info(silentFlagErr.Error())
		}

		verifyPostBodyFromReq := json.NewDecoder(r.Body)
		verifyPostBodyFromReq.DisallowUnknownFields()

		var verifyPostBody auth.VerifyPostBody

		if err := verifyPostBodyFromReq.Decode(&verifyPostBody); err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		authObjI, err := s.GetAuth(ctx, projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(ctx).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return

		}

		_, err = authObjI.GenerateVerifyCode(ctx, verifyPostBody, projectId, silentFlag)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "code sent successfully"})
			return
		}
	}
}

func CheckVerifyCodeHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("CheckVerifyCodeHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(r.Context()).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return

		}

		verifyReq := json.NewDecoder(r.Body)
		verifyReq.DisallowUnknownFields()

		var verifyCode auth.VerifyCode

		if err = verifyReq.Decode(&verifyCode); err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		tokenKey, tokenKeyErr := authObjI.GetAttribute(r.Context(), "token_header_key")
		if tokenKeyErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": tokenKeyErr.Error()})
			return
		}
		tokenStr := r.Header.Get(tokenKey.(string))
		tokenObj, tokenObjErr := getToken(r.Context(), tokenStr)
		if tokenObjErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": tokenObjErr.Error()})
			return
		}
		userId, userIdErr := getUserIdFromToken(tokenObj)
		if userIdErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": userIdErr.Error()})
			return
		}
		verifyCode.UserId = userId

		res, err := authObjI.VerifyCode(r.Context(), verifyCode, tokenObj, true)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "not verified"})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		if res != nil {
			_ = json.NewEncoder(w).Encode(res)
		} else {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "verified"})
		}
		return
	}
}

func VerifyRecoveryCodeHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("VerifyRecoveryCodeHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(r.Context()).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return
		}
		recoveryReq := json.NewDecoder(r.Body)
		recoveryReq.DisallowUnknownFields()

		var recoveryPassword auth.RecoveryPassword

		if err = recoveryReq.Decode(&recoveryPassword); err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		var cookies []*http.Cookie
		res := make(map[string]string)
		res, cookies, err = authObjI.VerifyRecovery(r.Context(), recoveryPassword)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			for _, c := range cookies {
				http.SetCookie(w, c)
				logs.WithContext(r.Context()).Debug(fmt.Sprint(c))
			}
			server_handlers.FormatResponse(w, http.StatusOK)
			_ = json.NewEncoder(w).Encode(res)
			return
		}
	}
}
func CompleteRecoveryHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("CompleteRecoveryHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		recoveryReq := json.NewDecoder(r.Body)
		recoveryReq.DisallowUnknownFields()

		var recoveryPassword auth.RecoveryPassword

		if err = recoveryReq.Decode(&recoveryPassword); err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		msg := ""
		for _, c := range r.Cookies() {
			logs.WithContext(r.Context()).Info(c.String())
		}
		msg, err = authObjI.CompleteRecovery(r.Context(), recoveryPassword, r.Cookies())
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": msg})
			return
		}
	}
}

func LogoutHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("LogoutHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		res, resStatusCode, err := authObjI.Logout(r.Context(), r)
		if err != nil {
			server_handlers.FormatResponse(w, resStatusCode)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, resStatusCode)
			_ = json.NewEncoder(w).Encode(res)
			return
		}
	}
}

func GenerateOtpHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GenerateOtpHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		gatewayType := vars["gatewaytype"]
		messageType := vars["messagetype"]
		channel := vars["channel"]
		gatewayI, err := s.GetGatewayFromType(r.Context(), gatewayType, channel, projectId)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			gatewayName, gnerr := gatewayI.GetAttribute("gateway_name")
			if gnerr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": gnerr.Error()})
				return
			}
			mt, mterr := s.GetMessageTemplate(r.Context(), gatewayName.(string), projectId, messageType)
			if mterr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": mterr.Error()})
				return
			}
			otp := fmt.Sprint(rand.Intn(999999-100000) + 100000)
			res, senderr := gatewayI.Send(r.Context(), mt.GetMessageText(otp), mt.TemplateId, r.URL.Query())
			if senderr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": senderr.Error()})
				return
			}
			server_handlers.FormatResponse(w, 200)
			//_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("OTP ", otp, " generated successfully")})
			_ = json.NewEncoder(w).Encode(res)
		}
	}
}

func GetUserHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GetUserHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		getUserReq := json.NewDecoder(r.Body)
		getUserReq.DisallowUnknownFields()
		getUserObj := make(map[string]interface{})
		//storageObj := new(storage.Storage)
		if err := getUserReq.Decode(&getUserObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		userIdStr := ""
		if userId, ok := getUserObj["id"]; !ok {
			rtErr := errors.New("id attribute missing in request body")
			logs.WithContext(r.Context()).Error(rtErr.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": rtErr})
			return
		} else {
			if userIdStr, ok = userId.(string); !ok {
				rtErr := errors.New("Incorrect refresh_token recevied in request body")
				logs.WithContext(r.Context()).Error(rtErr.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": rtErr})
				return
			}
		}

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		identity, err := authObjI.GetUser(r.Context(), userIdStr)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(identity)
		return
	}
}

func UpdateUserHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("UpdateUserHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		updateUserReq := json.NewDecoder(r.Body)
		updateUserReq.DisallowUnknownFields()
		updateUserObj := make(map[string]interface{})
		//storageObj := new(storage.Storage)
		if err := updateUserReq.Decode(&updateUserObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		identity := auth.Identity{}
		userAttributes := make(map[string]interface{})
		if userAttributesObj, ok := updateUserObj["attributes"]; !ok {
			rtErr := errors.New("attributes missing in request body")
			logs.WithContext(r.Context()).Error(rtErr.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": rtErr})
			return
		} else {
			if userAttributes, ok = userAttributesObj.(map[string]interface{}); !ok {
				rtErr := errors.New("incorrect post body")
				logs.WithContext(r.Context()).Error(rtErr.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": rtErr})
				return
			}
		}
		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(r.Context()).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return

		}

		identity.Attributes = userAttributes

		tokenKey, tokenKeyErr := authObjI.GetAttribute(r.Context(), "token_header_key")
		if tokenKeyErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": tokenKeyErr.Error()})
			return
		}
		tokenStr := r.Header.Get(tokenKey.(string))
		tokenObj, tokenObjErr := getToken(r.Context(), tokenStr)
		if tokenObjErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": tokenObjErr.Error()})
			return
		}
		userId, userIdErr := getUserIdFromToken(tokenObj)
		if userIdErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": userIdErr.Error()})
			return
		}
		identity.Id = userId
		var tokens interface{}
		tokens, err = authObjI.UpdateUser(r.Context(), identity, userId, tokenObj)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(tokens)
		return
	}
}

func ChangePasswordHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ChangePasswordHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		changePasswordReq := json.NewDecoder(r.Body)
		changePasswordReq.DisallowUnknownFields()
		changePasswordObj := auth.ChangePassword{}
		//storageObj := new(storage.Storage)
		if err := changePasswordReq.Decode(&changePasswordObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(r.Context()).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return

		}
		tokenKey, tokenKeyErr := authObjI.GetAttribute(r.Context(), "token_header_key")
		if tokenKeyErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": tokenKeyErr.Error()})
			return
		}
		tokenStr := r.Header.Get(tokenKey.(string))
		tokenObj, tokenObjErr := getToken(r.Context(), tokenStr)
		if tokenObjErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": tokenObjErr.Error()})
			return
		}
		userId, userIdErr := getUserIdFromToken(tokenObj)
		if userIdErr != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": userIdErr.Error()})
			return
		}

		err = authObjI.ChangePassword(r.Context(), tokenObj, userId, changePasswordObj)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": "password updated successfully"})
		return
	}
}

func GetSsoUrlHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GetSsoUrl - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		params := r.URL.Query()
		state := params.Get("state")
		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		responseBody := make(map[string]string)
		url, msParams, err := authObjI.GetUrl(r.Context(), state)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		responseBody["url"] = url

		pkceRequired, err := authObjI.GetAttribute(r.Context(), "pkce")
		if pkceRequired.(bool) {
			err = s.SavePkceEvent(r.Context(), msParams, s)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			responseBody["request_id"] = msParams.ClientRequestId
		}

		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(responseBody)
		return
	}
}

func RegisterHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RegisterHandler - Start")
		ctx := context.WithValue(r.Context(), "Erufuncbaseurl", module_store.Erufuncbaseurl)
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		authObjI, err := s.GetAuth(ctx, projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(ctx).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return

		}

		registerPostBodyFromReq := json.NewDecoder(r.Body)
		registerPostBodyFromReq.DisallowUnknownFields()

		var registerPostBody auth.RegisterUser

		if err = registerPostBodyFromReq.Decode(&registerPostBody); err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		res, tokens, err := authObjI.Register(ctx, registerPostBody, projectId)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, http.StatusOK)
			if tokens.IdToken != "" {
				_ = json.NewEncoder(w).Encode(tokens)
			} else {
				_ = json.NewEncoder(w).Encode(res)
			}
			return
		}
	}
}

func getToken(ctx context.Context, tokenStr string) (tokenObj map[string]interface{}, err error) {
	if tokenStr != "" {
		err = json.Unmarshal([]byte(tokenStr), &tokenObj)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("error while unmarshalling token claim : ", err.Error()))
			return
		}
	} else {
		err = errors.New("token not found")
	}
	return
}

func getUserIdFromToken(tokenObj map[string]interface{}) (userId string, err error) {
	if iObj, iObjOk := tokenObj["identity"]; iObjOk {
		if iObjMap, iObjMapOk := iObj.(map[string]interface{}); iObjMapOk {
			if uid, userIdOk := iObjMap["id"]; userIdOk {
				userId = uid.(string)
			}
		}
	}
	if userId == "" {
		err = errors.New("userid not found")
		return
	}
	return
}

func RemoveIdentityHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RemoveIdentityHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		if authObjI.GetAuthDb() != nil {
			authObjI.GetAuthDb().SetConn(s.GetConn())
		} else {
			logs.WithContext(r.Context()).Error("authObjI.GetAuthDb() is nil")
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Something went wrong, Please try again."})
			return

		}

		removeUserPostBodyFromReq := json.NewDecoder(r.Body)
		removeUserPostBodyFromReq.DisallowUnknownFields()

		var removeUserPostBody auth.RemoveUser

		if err = removeUserPostBodyFromReq.Decode(&removeUserPostBody); err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		err = authObjI.RemoveUser(r.Context(), removeUserPostBody)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "identity deleted successfully"})
		return
	}
}
