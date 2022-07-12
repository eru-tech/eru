package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// Scopes: OAuth 2.0 scopes provide a way to limit the amount of access that is granted to an access token.
var OauthConfig = &oauth2.Config{
	RedirectURL:  "http://127.0.0.1:8085/auth/openid/callback",
	ClientID:     "smartvalues",
	ClientSecret: "",
	Scopes:       []string{"openid", "offline"},
	Endpoint: oauth2.Endpoint{
		AuthURL:  "http://127.0.0.1:4444/oauth2/auth",
		TokenURL: "http://127.0.0.1:4444/oauth2/token",
	},
}

func GetLoginFlowHandlerandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kratosUrl := "http://127.0.0.1:4433/self-service/login/browser"
		log.Println(kratosUrl)
	}
}

func OpenIdLoginHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := OauthConfig.AuthCodeURL("eruhydra")
		log.Println(u)
		http.Redirect(w, r, u, http.StatusTemporaryRedirect)
	}
}

func OpenIdCallbackHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get authorization code from url query and exchange it for access token
		code := r.URL.Query().Get("code")
		token, err := OauthConfig.Exchange(r.Context(), code)
		if err != nil {
			log.Printf("unable to exchange code for token: %s\n", err)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(err)
			return
		}
		idt := token.Extra("id_token")
		log.Printf("Access Token:\n\t%s\n", token.AccessToken)
		log.Printf("Refresh Token:\n\t%s\n", token.RefreshToken)
		log.Printf("Expires in:\n\t%s\n", token.Expiry.Format(time.RFC1123))
		log.Printf("ID Token:\n\t%v\n\n", idt)

		fmt.Fprintf(w, "Callback Called")
	}
}

var store = sessions.NewCookieStore([]byte("secret-key"))

var appSession *sessions.Session

func setSessionValue(w http.ResponseWriter, r *http.Request, key string, value interface{}) {
	session := initSession(r)
	session.Values[key] = value
	log.Printf("set session with key %s and value %s\n", key, value)
	session.Save(r, w)
}
func initSession(r *http.Request) *sessions.Session {
	log.Println("session before get", appSession)

	if appSession != nil {
		return appSession
	}

	session, err := store.Get(r, "idp")
	appSession = session

	log.Println("session after get", session)
	if err != nil {
		panic(err)
	}
	return session
}

func LoginHydraHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("inside LoginHydraHandler")
		b := make([]byte, 32)
		_, err := rand.Read(b)
		if err != nil {
			log.Println("generate state failed: %v", err)
			return
		}
		state := base64.StdEncoding.EncodeToString(b)
		setSessionValue(w, r, "oauth2State", state)

		// start oauth2 authorization code flow
		redirectTo := OauthConfig.AuthCodeURL(state)
		log.Println("redirect to hydra, url: %s", redirectTo)
		res, headers, cookies, statusCode, err := utils.CallHttp("GET", redirectTo, nil, nil, nil, nil, nil)
		log.Println(res)
		log.Println(headers)
		log.Println(statusCode)
		log.Println(cookies)
		log.Println(err)
		//http.Redirect(w, r, redirectTo, http.StatusFound)
		return
	}
}

func UserInfoHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]
		log.Println(authName)
		log.Println(projectId)

		userInfoFromReq := json.NewDecoder(r.Body)
		userInfoFromReq.DisallowUnknownFields()
		userInfoObj := make(map[string]interface{})
		//storageObj := new(storage.Storage)
		if err := userInfoFromReq.Decode(&userInfoObj); err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		accessTokenStr := ""
		if accessToken, ok := userInfoObj["access_token"]; !ok {
			atErr := errors.New("access_token attribute missing in request body")
			log.Println(atErr)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": atErr})
			return
		} else {
			if accessTokenStr, ok = accessToken.(string); !ok {
				atErr := errors.New("Incorrect access_token recevied in request body")
				log.Println(atErr)
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": atErr})
				return
			}
		}

		authObjI, err := s.GetAuth(projectId, authName)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		identity, err := authObjI.GetUserInfo(accessTokenStr)
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
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]
		log.Println(authName)
		log.Println(projectId)

		fetchTokenFromReq := json.NewDecoder(r.Body)
		fetchTokenFromReq.DisallowUnknownFields()
		fetchTokenObj := make(map[string]interface{})
		//storageObj := new(storage.Storage)
		if err := fetchTokenFromReq.Decode(&fetchTokenObj); err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		refreshTokenStr := ""
		if refreshToken, ok := fetchTokenObj["refresh_token"]; !ok {
			rtErr := errors.New("refresh_token attribute missing in request body")
			log.Println(rtErr)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": rtErr})
			return
		} else {
			if refreshTokenStr, ok = refreshToken.(string); !ok {
				rtErr := errors.New("Incorrect refresh_token recevied in request body")
				log.Println(rtErr)
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": rtErr})
				return
			}
		}

		authObjI, err := s.GetAuth(projectId, authName)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		loginSuccess, err := authObjI.FetchTokens(refreshTokenStr)
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
		vars := mux.Vars(r)
		projectId := vars["project"]
		tokenType := vars["tokentype"]
		authName := vars["authname"]
		log.Println(authName)
		log.Println(projectId)
		log.Println(tokenType)

		authObjI, err := s.GetAuth(projectId, authName)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenHeaderKey, err := authObjI.GetAttribute("TokenHeaderKey")
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenToVerify := r.Header.Get(tokenHeaderKey.(string))
		log.Println(tokenToVerify)

		res, err := authObjI.VerifyToken(tokenType, tokenToVerify)
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
		vars := mux.Vars(r)
		projectId := vars["project"]
		authName := vars["authname"]
		log.Println(projectId)
		log.Println(authName)

		authObjI, err := s.GetAuth(projectId, authName)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		res, cookies, err := authObjI.Login(r)
		if err != nil {
			log.Println(err)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, http.StatusOK)

			log.Println("cookies = ", len(cookies))

			for _, v := range cookies {
				cookie := http.Cookie{Name: v.Name, Value: v.Value, Path: v.Path, Expires: v.Expires, MaxAge: v.MaxAge, HttpOnly: v.HttpOnly, Secure: v.Secure}
				log.Println(cookie.Path)
				log.Println(cookie.Name)
				log.Println(cookie.HttpOnly)
				log.Println(cookie.Expires)
				http.SetCookie(w, &cookie)
				w.Header().Add("Set-Cookie", v.String())
			}
			//expire := time.Now().Add(20 * time.Minute) // Expires in 20 minutes
			cookie := http.Cookie{Name: "abc", Value: "xyz", Path: "/"}
			http.SetCookie(w, &cookie)
			_ = json.NewEncoder(w).Encode(res)
			log.Println(w.Header())
			return
		}
	}
}
func GenerateOtpHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		gatewayType := vars["gatewaytype"]
		messageType := vars["messagetype"]
		channel := vars["channel"]
		log.Println("inside GenerateOtpHandler")
		gatewayI, err := s.GetGatewayFromType(gatewayType, channel, projectId)
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
			mt, mterr := s.GetMessageTemplate(gatewayName.(string), projectId, messageType)
			if mterr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": mterr.Error()})
				return
			}
			//todo : to generate otp based on project setting 4 digits or 6 digits
			otp := fmt.Sprint(rand.Intn(999999-100000) + 100000)
			res, senderr := gatewayI.Send(mt.GetMessageText(otp), mt.TemplateId, r.URL.Query())
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
