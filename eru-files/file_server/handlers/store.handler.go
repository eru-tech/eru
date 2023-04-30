package server

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-files/file_model"
	"github.com/eru-tech/eru/eru-files/module_store"
	"github.com/eru-tech/eru/eru-files/storage"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type t interface {
	t1()
}

func StoreCompareHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StoreCompareHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]

		projectJson := json.NewDecoder(r.Body)
		projectJson.DisallowUnknownFields()
		var postBody interface{}
		storeCompareMap := make(map[string]map[string]interface{})
		storeCompare := file_model.StoreCompare{}

		if err := projectJson.Decode(&postBody); err == nil {
			storeCompareMap["projects"] = make(map[string]interface{})
			storeCompareMap["projects"][projectID] = postBody
			postBodyBytes, pbbErr := json.Marshal(storeCompareMap)
			if pbbErr != nil {
				logs.Logger.Error(pbbErr.Error())
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": pbbErr.Error()})
				return
			}
			compareStore := module_store.GetStore(strings.ToUpper(os.Getenv("STORE_TYPE")))
			umErr := module_store.UnMarshalStore(r.Context(), postBodyBytes, compareStore)
			if umErr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": umErr.Error()})
				return
			}
			compareProject, cpErr := compareStore.GetProjectConfig(r.Context(), projectID)
			if cpErr != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": cpErr.Error()})
				return
			}
			myPrj, err := s.GetProjectConfig(r.Context(), projectID)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			storeCompare, err = myPrj.CompareProject(r.Context(), *compareProject)

		} else {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(storeCompare)

	}
}
func ProjectSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectSaveHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.SaveProject(r.Context(), projectID, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project ", projectID, " created successfully")})
		}
	}
}

func ProjectRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectRemoveHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.RemoveProject(r.Context(), projectID, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project ", projectID, " removed successfully")})
		}
	}
}

func ProjectListHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectListHandler - Start")
		projectIds := s.GetProjectList(r.Context())
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"projects": projectIds})
	}
}

func ProjectConfigHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectConfigHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		project, err := s.GetProjectConfig(r.Context(), projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"project": project})
		}
	}
}

func StorageSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StorageSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]
		storageType := vars["storagetype"]

		storageFromReq := json.NewDecoder(r.Body)
		storageFromReq.DisallowUnknownFields()

		storageObj := storage.GetStorage(storageType)
		if err := storageFromReq.Decode(&storageObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			//err := file_utils.ValidateStruct(storageObj, "") //TODO to uncomment this code and validate the incoming json
			//if err != nil {
			//	server_handlers.FormatResponse(w, 400)
			//	json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
			//	return
			//}
		}
		//err := storageObj.Save(s,projectId,storageName)
		err := s.SaveStorage(r.Context(), storageObj, projectId, s, true)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("storage config for ", storageName, " saved successfully")})
		}
		return
	}
}

func StorageRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StorageRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		err := s.RemoveStorage(r.Context(), storageName, projectId, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore(r.Context(), "", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("storage config for ", storageName, " removed successfully")})
		}
		return
	}
}

func RsaKeyPairGenerateHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("RsaKeyPairGenerateHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		keyPairName := vars["keypairname"]

		reqBody := json.NewDecoder(r.Body)
		reqBody.DisallowUnknownFields()

		reqBodyObj := make(map[string]string)
		//storageObj := new(storage.Storage)
		if err := reqBody.Decode(&reqBodyObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		bits := reqBodyObj["bits"]
		bitsInt, err := strconv.Atoi(bits)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		overwrite := reqBodyObj["overwrite"]
		overwriteB, err := strconv.ParseBool(overwrite)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		kp, err := s.GenerateRsaKeyPair(r.Context(), projectID, keyPairName, bitsInt, overwriteB, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"keyPair": kp})
		}
	}
}
func AesKeyGenerateHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("AesKeyGenerateHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		keyName := vars["keyname"]

		reqBody := json.NewDecoder(r.Body)
		reqBody.DisallowUnknownFields()

		reqBodyObj := make(map[string]string)
		//storageObj := new(storage.Storage)
		if err := reqBody.Decode(&reqBodyObj); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		bits := reqBodyObj["bits"]
		bitsInt, err := strconv.Atoi(bits)
		overwrite := reqBodyObj["overwrite"]
		overwriteB, err := strconv.ParseBool(overwrite)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		aesKey, err := s.GenerateAesKey(r.Context(), projectID, keyName, bitsInt, overwriteB, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"aesKey": aesKey})
		}
	}
}
