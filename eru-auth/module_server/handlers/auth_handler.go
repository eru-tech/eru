package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"math/rand"
	"net/http"
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
		fetchTokenObj := make(map[string]interface{})
		//storageObj := new(storage.Storage)
		if err := fetchTokenFromReq.Decode(&fetchTokenObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		refreshTokenStr := ""
		if refreshToken, ok := fetchTokenObj["refresh_token"]; !ok {
			rtErr := errors.New("refresh_token attribute missing in request body")
			logs.WithContext(r.Context()).Error(rtErr.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": rtErr})
			return
		} else {
			if refreshTokenStr, ok = refreshToken.(string); !ok {
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

		loginSuccess, err := authObjI.FetchTokens(r.Context(), refreshTokenStr)
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
		tokenHeaderKey, err := authObjI.GetAttribute(r.Context(), "TokenHeaderKey")
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
		logs.WithContext(r.Context()).Debug("LoginHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]

		authObjI, err := s.GetAuth(r.Context(), projectId, authName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		loginPostBodyFromReq := json.NewDecoder(r.Body)
		loginPostBodyFromReq.DisallowUnknownFields()

		var loginPostBody auth.LoginPostBody

		if err = loginPostBodyFromReq.Decode(&loginPostBody); err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		msParams, err := s.GetPkceEvent(r.Context(), loginPostBody.IdpRequestId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		loginPostBody.CodeVerifier = msParams.CodeVerifier
		loginPostBody.Nonce = msParams.Nonce
		res, tokens, err := authObjI.Login(r.Context(), loginPostBody, true)
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

		var recoveryPostBody auth.RecoveryPostBody

		if err = recoveryReq.Decode(&recoveryPostBody); err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		res, err := authObjI.GenerateRecoveryCode(r.Context(), recoveryPostBody)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": res})
			return
		}
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
			gatewayName, gnerr := gatewayI.GetAttribute("GatewayName")
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
			//todo : to generate otp based on project setting 4 digits or 6 digits
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
		identity.Attributes = userAttributes
		err = authObjI.UpdateUser(r.Context(), identity)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": "user updated successfully"})
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

		err = authObjI.ChangePassword(r.Context(), r, changePasswordObj)
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

func GetSsoUrl(s module_store.ModuleStoreI) http.HandlerFunc {
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

		url, msParams, err := authObjI.GetUrl(r.Context(), state)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		err = s.SavePkceEvent(r.Context(), msParams, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"url": url, "requestId": msParams.ClientRequestId})
		return
	}
}
