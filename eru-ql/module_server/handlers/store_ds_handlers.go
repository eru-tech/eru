package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strings"
	//"../server"
)

func DefaultDriverConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		dbType := vars["dbType"] // not used for now but we may have different defaults for different db type
		log.Print(dbType)
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"driverconfig": ds.DefaultDriverConfig})
	}
}

func DefaultOtherDBConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		dbType := vars["dbType"] // not used for now but we may have different defaults for different db type
		log.Print(dbType)
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"OtherDbConfig": ds.DefaultOtherConfig})
	}
}

func DefaultDBSecurityRulesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		dbType := vars["dbType"] // not used for now but we may have different defaults for different db type
		log.Print(dbType)
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"DbSecurityRules": ds.DefaultDbSecurityRules})
	}
}

func ProjectDataSourceConfigHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		datasource, err := s.GetDataSource(projectId, dbAlias)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"datasource": datasource})
		return
	}
}

func ProjectDataSourceSaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		dsFromReq := json.NewDecoder(r.Body)
		dsFromReq.DisallowUnknownFields()

		var datasource module_model.DataSource

		if err := dsFromReq.Decode(&datasource); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := eru_utils.ValidateStruct(datasource, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveDataSource(projectId, &datasource, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{dbAlias: datasource})
		}
		return
	}
}

func ProjectDataSourceRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]

		err := s.RemoveDataSource(projectId, dbAlias, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("datasource config for ", dbAlias, " removed successfully")})
		}
		return
	}
}

func ProjectDataSourceListHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]

		datasources, err := s.GetDataSources(projectId)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"datasources": datasources})
		}
		return
	}
}

func ProjectDataSourceSchemaHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]

		datasource, err := s.UpdateSchemaTables(projectId, dbAlias, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"SchemaTables": datasource.SchemaTables, "OtherTables": datasource.OtherTables})
		}
		return
	}
}

func ProjectDataSourceSchemaAddTableHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]

		res, err := s.AddSchemaTable(projectId, dbAlias, tableName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(res)
		}
		return
	}
}

func ProjectDataSourceSchemaRemoveTableHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]

		res, err := s.RemoveSchemaTable(projectId, dbAlias, tableName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(res)
		}
		return
	}
}
func ProjectDataSourceSchemaAddJoinHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Print(vars)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		var tj module_model.TableJoins
		if err := json.NewDecoder(r.Body).Decode(&tj); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			log.Print(err)
			return
		}
		log.Print(projectId, " ", dbAlias)
		log.Print(tj)
		res, err := s.AddSchemaJoin(projectId, dbAlias, &tj, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(res)
		}
		return
	}
}
func ProjectDataSourceSchemaRemoveJoinHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		log.Print(vars)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		var tj module_model.TableJoins
		if err := json.NewDecoder(r.Body).Decode(&tj); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			log.Print(err)
			return
		}
		log.Print(projectId, " ", dbAlias)
		log.Print(tj)
		res, err := s.RemoveSchemaJoin(projectId, dbAlias, &tj, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(res)
		}
		return
	}
}

func ProjectDataSourceSchemaSaveTableHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)
		log.Println(tableName)
		tableFromReq := json.NewDecoder(r.Body)
		tableFromReq.DisallowUnknownFields()

		var tableObj map[string]module_model.TableColsMetaData

		if err := tableFromReq.Decode(&tableObj); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := eru_utils.ValidateStruct(tableObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveSchemaTable(projectId, dbAlias, tableName, tableObj, s)
		log.Println(err)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Table ", tableName, " created successfully")})
		}
		return
	}
}
func ProjectDataSourceSchemaTransformTableHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)
		log.Println(tableName)
		transformRulesFromReq := json.NewDecoder(r.Body)
		transformRulesFromReq.DisallowUnknownFields()

		var transformRules module_model.TransformRules

		if err := transformRulesFromReq.Decode(&transformRules); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := eru_utils.ValidateStruct(transformRules, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveTableTransformation(projectId, dbAlias, tableName, transformRules, s)
		log.Println(err)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Table Transformation for ", tableName, " set successfully")})
		}
		return
	}
}

func ProjectDataSourceSchemaSecureTableHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)
		log.Println(tableName)
		securityRulesFromReq := json.NewDecoder(r.Body)
		securityRulesFromReq.DisallowUnknownFields()

		var securityRules module_model.SecurityRules

		if err := securityRulesFromReq.Decode(&securityRules); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := eru_utils.ValidateStruct(securityRules, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveTableSecurity(projectId, dbAlias, tableName, securityRules, s)
		log.Println(err)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Table Security for ", tableName, " set successfully")})
		}
		return
	}
}

func ProjectDataSourceSchemaDropTableHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)
		log.Println(tableName)

		err := s.DropSchemaTable(projectId, dbAlias, tableName, s)
		log.Println(err)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Table ", tableName, " dropped successfully")})
		}
		return
	}
}
