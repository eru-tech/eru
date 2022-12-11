package server

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-files/module_store"
	"github.com/eru-tech/eru/eru-files/storage"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
)

type t interface {
	t1()
}

func ProjectSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.SaveProject(projectID, s, true)
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
		vars := mux.Vars(r)
		projectID := vars["project"]
		err := s.RemoveProject(projectID, s)
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
		//token, err := VerifyToken(r.Header.Values("Authorization")[0])
		//log.Print(token.Method)
		//log.Print(err)
		projectIds := s.GetProjectList()
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"projects": projectIds})
	}
}

func ProjectConfigHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		log.Print(projectID)
		project, err := s.GetProjectConfig(projectID)
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
		log.Print("inside StorageSaveHandler")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]
		storageType := vars["storagetype"]

		log.Println(projectId, " ", storageName, " ", storageType, " ")

		storageFromReq := json.NewDecoder(r.Body)
		storageFromReq.DisallowUnknownFields()
		//t := new(map[string]string)
		//if err1 := storageFromReq.Decode(t); err1 != nil {
		//log.Println("error " , err1)
		//}
		//log.Println(t)
		storageObj := storage.GetStorage(storageType)
		//storageObj := new(storage.Storage)
		if err := storageFromReq.Decode(&storageObj); err != nil {
			log.Println(err)
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
		err := s.SaveStorage(storageObj, projectId, s, true)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore("", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("storage config for ", storageName, " saved successfully")})
		}
		return
	}
}

func StorageRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("inside StorageRemoveHandler")
		vars := mux.Vars(r)
		projectId := vars["project"]
		storageName := vars["storagename"]

		err := s.RemoveStorage(storageName, projectId, s)
		if err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			s.SaveStore("", s)
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("storage config for ", storageName, " removed successfully")})
		}
		return
	}
}

func RsaKeyPairGenerateHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		log.Print(projectID)
		keyPairName := vars["keypairname"]

		reqBody := json.NewDecoder(r.Body)
		reqBody.DisallowUnknownFields()

		reqBodyObj := make(map[string]string)
		//storageObj := new(storage.Storage)
		if err := reqBody.Decode(&reqBodyObj); err != nil {
			log.Println(err)
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
		log.Println(bits)
		log.Println(overwrite)
		overwriteB, err := strconv.ParseBool(overwrite)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		kp, err := s.GenerateRsaKeyPair(projectID, keyPairName, bitsInt, overwriteB, s)
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
		vars := mux.Vars(r)
		projectID := vars["project"]
		log.Print(projectID)
		keyName := vars["keyname"]

		reqBody := json.NewDecoder(r.Body)
		reqBody.DisallowUnknownFields()

		reqBodyObj := make(map[string]string)
		//storageObj := new(storage.Storage)
		if err := reqBody.Decode(&reqBodyObj); err != nil {
			log.Println(err)
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		bits := reqBodyObj["bits"]
		bitsInt, err := strconv.Atoi(bits)
		overwrite := reqBodyObj["overwrite"]
		log.Println(bits)
		log.Println(overwrite)
		overwriteB, err := strconv.ParseBool(overwrite)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		aesKey, err := s.GenerateAesKey(projectID, keyName, bitsInt, overwriteB, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"aesKey": aesKey})
		}
	}
}
