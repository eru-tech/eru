package handlers

import (
	"encoding/json"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-store/store"
	"github.com/gorilla/mux"
	"net/http"
)

var ServerName = "unkown"
var AllowedOrigins = ""
var RequestIdKey = "request_id"

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hello" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}
	fmt.Fprintf(w, fmt.Sprint("Hello ", ServerName))
}

func EchoHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm((1 << 20) * 10)
	logs.Logger.Info(fmt.Sprint("r.ParseMultipartForm error = ", err))
	formData := r.MultipartForm
	res := make(map[string]interface{})
	res["FormData"] = formData
	res["Host"] = r.Host
	res["Header"] = r.Header
	res["URL"] = r.URL
	tmplBodyFromReq := json.NewDecoder(r.Body)
	tmplBodyFromReq.DisallowUnknownFields()
	var tmplBody interface{}
	if err := tmplBodyFromReq.Decode(&tmplBody); err != nil {
		logs.Logger.Error(err.Error())
	}
	res["Body"] = tmplBody
	res["Method"] = r.Method
	res["MultipartForm"] = r.MultipartForm
	res["RequestURI"] = r.RequestURI
	res["RemoteAddr"] = r.RemoteAddr
	res["Response"] = r.Response
	res["Cookies"] = r.Cookies()
	FormatResponse(w, 200)
	_ = json.NewEncoder(w).Encode(res)
	logs.Logger.Info("w.Header() from echo handler")
	logs.Logger.Info(fmt.Sprint(w.Header()))

}

func SaveVarHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveVarHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		varJson := json.NewDecoder(r.Body)
		varJson.DisallowUnknownFields()
		var sVar store.Vars
		if err := varJson.Decode(&sVar); err == nil {
			err = s.SaveVar(r.Context(), projectId, sVar, s)
			if err != nil {
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Variable with key ", sVar.Key, " saved successfully.")})
	}
}

func RemoveVarHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RemoveVarHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		varKey := vars["key"]
		err := s.RemoveVar(r.Context(), projectId, varKey, s)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Variable with key ", varKey, " removed successfully.")})
	}
}

func SaveEnvVarHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveSecretHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		varJson := json.NewDecoder(r.Body)
		varJson.DisallowUnknownFields()
		var sVar store.EnvVars
		if err := varJson.Decode(&sVar); err == nil {
			err = s.SaveEnvVar(r.Context(), projectId, sVar, s)
			if err != nil {
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Env. Variable with key ", sVar.Key, " saved successfully.")})
	}
}

func RemoveEnvVarHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RemoveSecretHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		varKey := vars["key"]
		err := s.RemoveEnvVar(r.Context(), projectId, varKey, s)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Env. Variable with key ", varKey, " removed successfully.")})
	}
}

func SaveSecretHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveSecretHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		varJson := json.NewDecoder(r.Body)
		varJson.DisallowUnknownFields()
		var sVar store.Secrets
		if err := varJson.Decode(&sVar); err == nil {
			err = s.SaveSecret(r.Context(), projectId, sVar, s)
			if err != nil {
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Secret with key ", sVar.Key, " saved successfully.")})
	}
}

func RemoveSecretHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RemoveSecretHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		varKey := vars["key"]
		err := s.RemoveSecret(r.Context(), projectId, varKey, s)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Secret with key ", varKey, " removed successfully.")})
	}
}

func FetchVarsHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RemoveSecretHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		variables, err := s.FetchVars(r.Context(), projectId)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(variables)
	}
}
