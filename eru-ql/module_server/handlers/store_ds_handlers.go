package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	eru_utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gorilla/mux"
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

func ProjectDataSourceConfigHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectDataSourceConfigHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		datasource, err := sh.Store.GetDataSource(r.Context(), projectId, dbAlias)
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

func ProjectDataSourceSaveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
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
		err := sh.Store.SaveDataSource(r.Context(), projectId, &datasource, sh.Store)
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

func ProjectDataSourceRemoveHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
		logs.WithContext(r.Context()).Debug("ProjectDataSourceRemoveHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]

		err := sh.Store.RemoveDataSource(r.Context(), projectId, dbAlias, sh.Store)
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

func ProjectDataSourceListHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectDataSourceListHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]

		datasources, err := sh.Store.GetDataSources(r.Context(), projectId)
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

func ProjectDataSourceSchemaHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]

		datasource, err := sh.Store.UpdateSchemaTables(r.Context(), projectId, dbAlias, sh.Store)
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

func ProjectDataSourceSchemaAddTableHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaAddTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]

		res, err := sh.Store.AddSchemaTable(r.Context(), projectId, dbAlias, tableName, sh.Store)
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

func ProjectDataSourceSchemaRemoveTableHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaRemoveTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]

		res, err := sh.Store.RemoveSchemaTable(r.Context(), projectId, dbAlias, tableName, sh.Store)
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
func ProjectDataSourceSchemaAddJoinHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
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

		res, err := sh.Store.AddSchemaJoin(r.Context(), projectId, dbAlias, &tj, sh.Store)
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
func ProjectDataSourceSchemaRemoveJoinHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
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

		res, err := sh.Store.RemoveSchemaJoin(r.Context(), projectId, dbAlias, &tj, sh.Store)
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

func ProjectDataSourceSchemaSaveTableHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
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
			for _, v := range tableObj {
				err := eru_utils.ValidateStruct(r.Context(), v, "")
				if err != nil {
					server_handlers.FormatResponse(w, 400)
					json.NewEncoder(w).Encode(map[string]interface{}{"error": fmt.Sprint("missing field in object : ", err.Error())})
					return
				}
			}
		}
		err := sh.Store.SaveSchemaTable(r.Context(), projectId, dbAlias, tableName, tableObj, sh.Store)
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
func ProjectDataSourceSchemaTransformTableHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
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
		err := sh.Store.SaveTableTransformation(r.Context(), projectId, dbAlias, tableName, transformRules, sh.Store)

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

func ProjectDataSourceSchemaMasColumnHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
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
		err := sh.Store.SaveColumnMasking(r.Context(), projectId, dbAlias, tableName, colName, columnMasking, sh.Store)

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

func ProjectDataSourceRemoveSecureTableHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
		logs.WithContext(r.Context()).Debug("ProjectDataSourceRemoveSecureTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)

		securityRulesFromReq := json.NewDecoder(r.Body)
		securityRulesFromReq.DisallowUnknownFields()

		err := sh.Store.RemoveTableSecurity(r.Context(), projectId, dbAlias, tableName, sh.Store)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": fmt.Sprint("Table Security for ", tableName, " removed successfully")})
		}
	}
}

func ProjectDataSourceGetSecureTableHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
		logs.WithContext(r.Context()).Debug("ProjectDataSourceGetSecureTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)

		securityRulesFromReq := json.NewDecoder(r.Body)
		securityRulesFromReq.DisallowUnknownFields()

		sr, err := sh.Store.GetTableSecurity(r.Context(), projectId, dbAlias, tableName)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(sr)
		}
	}
}

func ProjectDataSourceSchemaSecureTableHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
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
		err := sh.Store.SaveTableSecurity(r.Context(), projectId, dbAlias, tableName, securityRules, sh.Store)
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

func ProjectDataSourceSchemaDropTableHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Lock()
		defer sh.Unlock()
		logs.WithContext(r.Context()).Debug("ProjectDataSourceSchemaDropTableHandler - Start")
		vars := mux.Vars(r)
		projectId := vars["project"]
		dbAlias := vars["dbalias"]
		tableName := vars["tablename"]
		tableName = strings.Replace(tableName, "___", ".", 1)

		err := sh.Store.DropSchemaTable(r.Context(), projectId, dbAlias, tableName, sh.Store)
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
