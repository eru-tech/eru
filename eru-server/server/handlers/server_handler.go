package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-events/events"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-repos/repos"
	kms "github.com/eru-tech/eru/eru-secret-manager/kms"
	sm "github.com/eru-tech/eru/eru-secret-manager/sm"
	"github.com/eru-tech/eru/eru-store/store"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
)

var ServerName = "unkown"
var RepoName = "unkown.json"
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

func EnvHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("EnvHandler - Start")
		vars := mux.Vars(r)
		env := vars["env"]
		env_value := os.Getenv(env)
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{env: env_value})
	}
}

func StateHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StateHandler - Start")
		type serviceState struct {
			NumCPU       int
			NumGoroutine int
			Mem          runtime.MemStats
		}
		serviceStateObj := serviceState{}

		serviceStateObj.NumCPU = runtime.NumCPU()
		serviceStateObj.NumGoroutine = runtime.NumGoroutine()
		runtime.ReadMemStats(&serviceStateObj.Mem)

		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"service_state": serviceStateObj})
	}
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
		if projectId == "" {
			projectId = "gateway"
		}
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
		if projectId == "" {
			projectId = "gateway"
		}
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
		logs.WithContext(r.Context()).Debug("SaveEnvVarHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("env. variable with key ", sVar.Key, " saved successfully")})
	}
}

func RemoveEnvVarHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RemoveSecretHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
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
		if projectId == "" {
			projectId = "gateway"
		}
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
		if projectId == "" {
			projectId = "gateway"
		}
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
		logs.WithContext(r.Context()).Debug("FetchVarsHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
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

func SaveRepoHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveRepoHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		repoType := vars["repotype"]
		varJson := json.NewDecoder(r.Body)
		varJson.DisallowUnknownFields()
		sRepoI := repos.GetRepo(repoType)
		if err := varJson.Decode(&sRepoI); err == nil {
			err = s.SaveRepo(r.Context(), projectId, sRepoI, s, true)
			if err != nil {
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Repo for project ", projectId, " saved successfully.")})
	}
}

func SaveRepoTokenHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveRepoTokenHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		varJson := json.NewDecoder(r.Body)
		varJson.DisallowUnknownFields()
		var sRepoToken repos.RepoToken
		if err := varJson.Decode(&sRepoToken); err == nil {
			err = s.SaveRepoToken(r.Context(), projectId, sRepoToken, s)
			if err != nil {
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Repo Token for project ", projectId, " saved successfully.")})
	}
}

func FetchSmHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Info("FetchSmHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		smObj, err := s.FetchSm(r.Context(), projectId)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(smObj)
	}
}

func LoadSmValueHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("LoadSmValueHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		err := s.LoadSmValue(r.Context(), projectId)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": "secret values loaded successfully"})
	}
}
func SetSmValueHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Info("SetSmValueHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}

		type smMap struct {
			SecretName  string            `json:"secret_name"`
			SecretValue map[string]string `json:"secret_value"`
		}
		smMapObj := smMap{}

		smJson := json.NewDecoder(r.Body)
		smJson.DisallowUnknownFields()
		if err := smJson.Decode(&smMapObj); err == nil {
			err = s.SetSmValue(r.Context(), projectId, smMapObj.SecretName, smMapObj.SecretValue)
			if err != nil {
				logs.WithContext(r.Context()).Info(err.Error())
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			logs.WithContext(r.Context()).Info("error")
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": "secret values set successfully"})
	}
}

func GetSmValueHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GetSmValueHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		if fmt.Sprint(strings.Split(r.Host, ":")[0]) != "localhost" {
			err := errors.New("you can call this route only locally")
			logs.WithContext(r.Context()).Error(err.Error())
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		type smMap struct {
			SecretName  string `json:"secret_name"`
			SecretKey   string `json:"secret_key"`
			ForceDelete bool   `json:"force_delete"`
		}
		smMapObj := smMap{}
		var secret_value interface{}
		smJson := json.NewDecoder(r.Body)
		smJson.DisallowUnknownFields()
		if err := smJson.Decode(&smMapObj); err == nil {
			secret_value, err = s.GetSmValue(r.Context(), projectId, smMapObj.SecretName, smMapObj.SecretKey, smMapObj.ForceDelete)
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			logs.WithContext(r.Context()).Info("error")
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{smMapObj.SecretKey: secret_value})
	}
}

func UnsetSmValueHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Info("UnsetSmValueHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}

		type smMap struct {
			SecretName string `json:"secret_name"`
			SecretKey  string `json:"secret_key"`
		}
		smMapObj := smMap{}

		smJson := json.NewDecoder(r.Body)
		smJson.DisallowUnknownFields()
		if err := smJson.Decode(&smMapObj); err == nil {
			err = s.UnsetSmValue(r.Context(), projectId, smMapObj.SecretName, smMapObj.SecretKey)
			if err != nil {
				logs.WithContext(r.Context()).Info(err.Error())
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			logs.WithContext(r.Context()).Info("error")
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": "secret values unset successfully"})
	}
}

func SaveSmHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveSmHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		smType := vars["smtype"]

		smJson := json.NewDecoder(r.Body)
		smJson.DisallowUnknownFields()

		var smObj = sm.GetSm(smType)
		if err := smJson.Decode(&smObj); err == nil {
			err = s.SaveSm(r.Context(), projectId, smObj, s, true)
			if err != nil {
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Secret Manager for project ", projectId, " saved successfully.")})
	}
}

func FetchRepoHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("FetchRepoHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		repo, err := s.FetchRepo(r.Context(), projectId)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(repo)
	}
}

func CommitRepoHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("CommitRepoHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		err := s.CommitRepo(r.Context(), projectId, s)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("config for project ", projectId, " commited successfully.")})
		//_ = json.NewEncoder(w).Encode(config)
	}
}

func FetchKmsHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Info("FetchKmsHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		kmsObj, err := s.FetchKms(r.Context(), projectId)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(kmsObj)
	}
}

func SaveKmsHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveKmsHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		kmsType := vars["kmstype"]

		kmsJson := json.NewDecoder(r.Body)
		kmsJson.DisallowUnknownFields()

		var kmsObj = kms.GetKms(kmsType)
		if err := kmsJson.Decode(&kmsObj); err == nil {
			err = s.SaveKms(r.Context(), projectId, kmsObj, s, true)
			if err != nil {
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Key for project ", projectId, " saved successfully.")})
	}
}

func RemoveKmsHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveKmsHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		kmsName := vars["kmsname"]
		cloudDelete := vars["clouddelete"]
		cd := false
		if cloudDelete == "true" {
			cd = true
		}
		deleteDays := vars["deletedays"]
		var dd int64 = 7
		var err error
		if deleteDays != "" {
			dd, err = strconv.ParseInt(deleteDays, 10, 32)
			if err != nil {
				FormatResponse(w, 400)
				logs.WithContext(r.Context()).Info(err.Error())
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid delete days"})
				return
			}
		}

		err = s.RemoveKms(r.Context(), projectId, kmsName, cd, int32(dd), s)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Key for project ", projectId, " removed successfully.")})
	}
}

func FetchEventsHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Info("FetchEventsHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		eventObj, err := s.FetchEvents(r.Context(), projectId)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(eventObj)
	}
}

func SaveEventHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SaveEventHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		eventType := vars["eventtype"]

		eventJson := json.NewDecoder(r.Body)
		eventJson.DisallowUnknownFields()

		var eventObj = events.GetEvent(eventType)
		if err := eventJson.Decode(&eventObj); err == nil {
			err = s.SaveEvent(r.Context(), projectId, eventObj, s, true)
			if err != nil {
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("event for project ", projectId, " saved successfully.")})
	}
}

func PublishEventHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("PublishEventHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		eventName := vars["eventname"]

		eventJson := json.NewDecoder(r.Body)
		eventJson.DisallowUnknownFields()

		type msgType struct {
			Msg interface{} `json:"msg"`
		}
		var msgObj msgType
		msgId := ""
		if err := eventJson.Decode(&msgObj); err == nil {
			msgId, err = s.PublishEvent(r.Context(), projectId, eventName, msgObj, s)
			if err != nil {
				FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("event published for project ", projectId, " : ", msgId)})
	}
}

func PollEventHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("PublishEventHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		eventName := vars["eventname"]

		eventJson := json.NewDecoder(r.Body)
		eventJson.DisallowUnknownFields()

		err := s.PollEvent(r.Context(), projectId, eventName, s)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("event message processed for project ", projectId, ".")})
	}
}

func RemoveEventHandler(s store.StoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RemoveEventHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		if projectId == "" {
			projectId = "gateway"
		}
		eventName := vars["eventname"]
		cloudDelete := vars["clouddelete"]
		cd := false
		if cloudDelete == "true" {
			cd = true
		}

		err := s.RemoveEvent(r.Context(), projectId, eventName, cd, s)
		if err != nil {
			FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("event for project ", projectId, " removed successfully.")})
	}
}
