package handlers

import (
	"encoding/json"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
	//"../server"
)

func DefaultDriverConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("DefaultDriverConfigHandler - Start")
		vars := mux.Vars(r)
		dbType := vars["dbType"] // not used for now but we may have different defaults for different db type
		_ = dbType
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"driverconfig": ds.DefaultDriverConfig})
	}
}

func DefaultOtherDBConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("DefaultOtherDBConfigHandler - Start")
		vars := mux.Vars(r)
		dbType := vars["dbType"] // not used for now but we may have different defaults for different db type
		_ = dbType
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"OtherDbConfig": ds.DefaultOtherConfig})
	}
}

func DefaultDBSecurityRulesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("DefaultDBSecurityRulesHandler - Start")
		vars := mux.Vars(r)
		dbType := vars["dbType"] // not used for now but we may have different defaults for different db type
		_ = dbType
		server_handlers.FormatResponse(w, 200)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"DbSecurityRules": ds.DefaultDbSecurityRules})
	}
}

func ProjectDataSourceConfigHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectDataSourceConfigHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		datasource, err := s.GetDataSource(r.Context(), projectId, dbAlias)
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSaveHandler - Start")
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
			err := eru_utils.ValidateStruct(r.Context(), datasource, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveDataSource(r.Context(), projectId, &datasource, s)
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]

		err := s.RemoveDataSource(r.Context(), projectId, dbAlias, s)
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceListHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		datasources, err := s.GetDataSources(r.Context(), projectId)
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]

		datasource, err := s.UpdateSchemaTables(r.Context(), projectId, dbAlias, s)
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaAddTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]

		res, err := s.AddSchemaTable(r.Context(), projectId, dbAlias, tableName, s)
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaRemoveTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]

		res, err := s.RemoveSchemaTable(r.Context(), projectId, dbAlias, tableName, s)
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaAddJoinHandler - Start")
		vars := mux.Vars(r)

		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		var tj module_model.TableJoins
		if err := json.NewDecoder(r.Body).Decode(&tj); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			logs.WithContext(r.Context()).Error(err.Error())
			return
		}

		res, err := s.AddSchemaJoin(r.Context(), projectId, dbAlias, &tj, s)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaRemoveJoinHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		var tj module_model.TableJoins
		if err := json.NewDecoder(r.Body).Decode(&tj); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			logs.WithContext(r.Context()).Error(err.Error())
			return
		}

		res, err := s.RemoveSchemaJoin(r.Context(), projectId, dbAlias, &tj, s)
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaSaveTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)

		tableFromReq := json.NewDecoder(r.Body)
		tableFromReq.DisallowUnknownFields()

		var tableObj map[string]module_model.TableColsMetaData

		if err := tableFromReq.Decode(&tableObj); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := eru_utils.ValidateStruct(r.Context(), tableObj, "")
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveSchemaTable(r.Context(), projectId, dbAlias, tableName, tableObj, s)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaTransformTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)

		transformRulesFromReq := json.NewDecoder(r.Body)
		transformRulesFromReq.DisallowUnknownFields()

		var transformRules module_model.TransformRules

		if err := transformRulesFromReq.Decode(&transformRules); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := eru_utils.ValidateStruct(r.Context(), transformRules, "")
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveTableTransformation(r.Context(), projectId, dbAlias, tableName, transformRules, s)

		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Table Transformation for ", tableName, " set successfully")})
		}
		return
	}
}

func ProjectDataSourceSchemaMasColumnHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaMasColumnHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		colName := vars["colname"]
		tableName = strings.Replace(tableName, "___", ".", 1)

		columnMaskingFromReq := json.NewDecoder(r.Body)
		columnMaskingFromReq.DisallowUnknownFields()

		var columnMasking module_model.ColumnMasking

		if err := columnMaskingFromReq.Decode(&columnMasking); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := eru_utils.ValidateStruct(r.Context(), columnMasking, "")
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveColumnMasking(r.Context(), projectId, dbAlias, tableName, colName, columnMasking, s)

		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("column masking for ", colName, " set successfully")})
		}
		return
	}
}

func ProjectDataSourceSchemaSecureTableHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaSecureTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)

		securityRulesFromReq := json.NewDecoder(r.Body)
		securityRulesFromReq.DisallowUnknownFields()

		var securityRules module_model.SecurityRules

		if err := securityRulesFromReq.Decode(&securityRules); err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		} else {
			err := eru_utils.ValidateStruct(r.Context(), securityRules, "")
			if err != nil {
				logs.WithContext(r.Context()).Error(err.Error())
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
				return
			}
		}
		err := s.SaveTableSecurity(r.Context(), projectId, dbAlias, tableName, securityRules, s)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
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
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaDropTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)

		err := s.DropSchemaTable(r.Context(), projectId, dbAlias, tableName, s)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Table ", tableName, " dropped successfully")})
		}
		return
	}
}
