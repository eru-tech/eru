package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func StoreCompareHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("StoreCompareHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]

		projectJson := json.NewDecoder(r.Body)
		projectJson.DisallowUnknownFields()
		var compareProject module_model.Project
		storeCompare := module_model.StoreCompare{}

		if err := projectJson.Decode(&compareProject); err == nil {
			myPrj, err := s.GetProjectConfig(r.Context(), projectID)
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			//if !cmp.Equal(*myPrj, compareProject, cmp.AllowUnexported(module_model.Project{}), cmp.Reporter(&diffR)) {
			//log.Println(diffR.Output())
			//}

			for _, mq := range myPrj.MyQueries {
				var diffR utils.DiffReporter
				qFound := false
				for _, cq := range compareProject.MyQueries {
					if mq.QueryName == cq.QueryName {
						qFound = true
						if !cmp.Equal(mq, cq, cmp.Reporter(&diffR)) {
							if storeCompare.MismatchQuries == nil {
								storeCompare.MismatchQuries = make(map[string]interface{})
							}
							storeCompare.MismatchQuries[mq.QueryName] = diffR.Output()
							log.Println("___________++++++++++++__________________")
							log.Println(mq.QueryName)
							log.Println(storeCompare.MismatchQuries[mq.QueryName])
							log.Println("___________++++++++++++__________________")
						}
						break
					}
				}
				if !qFound {
					storeCompare.DeleteQueries = append(storeCompare.DeleteQueries, mq.QueryName)
				}
			}

			for _, cq := range compareProject.MyQueries {
				qFound := false
				for _, mq := range myPrj.MyQueries {
					if mq.QueryName == cq.QueryName {
						qFound = true
						break
					}
				}
				if !qFound {
					storeCompare.NewQueries = append(storeCompare.NewQueries, cq.QueryName)
				}
			}

			//compare datasources
			for _, md := range myPrj.DataSources {
				var diffR utils.DiffReporter
				dsFound := false
				for _, cd := range compareProject.DataSources {
					if md.DbAlias == cd.DbAlias {
						dsFound = true
						if !cmp.Equal(md, cd, cmpopts.IgnoreFields(module_model.DataSource{}, "Con"), cmpopts.IgnoreFields(module_model.TableColsMetaData{}, "ColPosition"), cmp.Reporter(&diffR)) {
							if storeCompare.MismatchDataSources == nil {
								storeCompare.MismatchDataSources = make(map[string]interface{})
							}
							storeCompare.MismatchDataSources[md.DbAlias] = diffR.Output()

						}
						break
					}
				}
				if !dsFound {
					storeCompare.DeleteQueries = append(storeCompare.DeleteDataSources, md.DbAlias)
				}
			}

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
			logs.WithContext(r.Context()).Error(err.Error())
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
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"project": project})
		}
	}
}

func ProjectConfigSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectConfigSaveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		prjConfigFromReq := json.NewDecoder(r.Body)
		prjConfigFromReq.DisallowUnknownFields()

		var projectCOnfig module_model.ProjectConfig

		if err := prjConfigFromReq.Decode(&projectCOnfig); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := utils.ValidateStruct(r.Context(), projectCOnfig, "")
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}

		err := s.SaveProjectConfig(r.Context(), projectId, projectCOnfig, s)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("project config config for ", projectId, " saved successfully")})
		}
	}
}

func ProjectGenerateAesKeyHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectGenerateAesKeyHandler - Start")
		bytes := make([]byte, 32) //generate a random 32 byte key for AES-256
		_, err := rand.Read(bytes)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"key": hex.EncodeToString(bytes)})
		}
		return
	}
}
