package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"github.com/eru-tech/eru/eru-ql/ql"
	eru_writes "github.com/eru-tech/eru/eru-read-write/eru_writes"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"strconv"
	//"../server"
)

func ProjectMyQuerySaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectMyQuerySaveHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		queryName := vars["queryname"]
		queryType := vars["querytype"]

		var err error
		if queryType == "graphql" {
			var gqd ql.GraphQLData
			if err := json.NewDecoder(r.Body).Decode(&gqd); err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				logs.WithContext(r.Context()).Error(err.Error())
				return
			}
			err = s.SaveMyQuery(r.Context(), projectID, queryName, queryType, "", gqd.Query, gqd.Variables, s, "", gqd.SecurityRule)
		} else if queryType == "sql" {
			var sqd ql.SQLData
			if err := json.NewDecoder(r.Body).Decode(&sqd); err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				logs.WithContext(r.Context()).Error(err.Error())
				return
			}

			err = s.SaveMyQuery(r.Context(), projectID, queryName, queryType, sqd.DBAlias, sqd.Query, sqd.Variables, s, sqd.Cols, sqd.SecurityRule)
		} else {
			err = errors.New("Incorrect query type")
		}
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})

		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprint("Query ", queryName, " saved successfully")})
		}
		return
	}
}

func ProjectMyQueryRemoveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectMyQueryRemoveHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		queryName := vars["queryname"]

		err := s.RemoveMyQuery(r.Context(), projectID, queryName, s)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprint("Query ", queryName, " removed successfully")})
		}
		return
	}
}

func ProjectMyQueryListHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectMyQueryListHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		queryType := vars["querytype"]

		myqueries, err := s.GetMyQueries(r.Context(), projectID, queryType)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"myqueries": myqueries})
		}
		return
	}
}

func ProjectMyQueryConfigHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectMyQueryConfigHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		queryName := vars["queryname"]

		myquery, err := s.GetMyQuery(r.Context(), projectID, queryName)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		} else {
			server_handlers.FormatResponse(w, 200)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"myquery": myquery})
		}
		return
	}
}

func ProjectMyQueryExecuteHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("ProjectMyQueryExecuteHandler - Start")
		//logs.WithContext(r.Context()).Info(time.Now(), " Start --------------------------------------------------- ")
		//time.Sleep(time.Duration(3000) * time.Millisecond)
		//logs.WithContext(r.Context()).Info(time.Now(), " End --------------------------------------------------- ")
		//claims := r.Header.Get("claims")

		vars := mux.Vars(r)
		projectID := vars["project"]
		queryName := vars["queryname"]
		outputType := vars["outputtype"]

		projectConfig, err := s.GetProjectConfigObject(r.Context(), projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenObj := make(map[string]interface{})
		tokenStr := r.Header.Get(projectConfig.TokenSecret.HeaderKey)
		if tokenStr != "" {
			err = json.Unmarshal([]byte(tokenStr), &tokenObj)
			if err != nil {
				logs.WithContext(r.Context()).Error(fmt.Sprint("error while unmarshalling token claim : ", err.Error()))
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}

		postBody := make(map[string]interface{})

		if err := json.NewDecoder(r.Body).Decode(&postBody); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			logs.WithContext(r.Context()).Error(err.Error())
			return
		}

		datasources, err := s.GetDataSources(r.Context(), projectID)
		if err != nil {
			logs.WithContext(r.Context()).Error(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		var res []map[string]interface{}
		//var queries []string
		myQuery, err := s.GetMyQuery(r.Context(), projectID, queryName)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			logs.WithContext(r.Context()).Error(err.Error())
			return
		}
		// overwriting variables with same names
		if myQuery.QueryName != "" {
			qlInterface := ql.GetQL(myQuery.QueryType)
			if qlInterface == nil {
				server_handlers.FormatResponse(w, 400)
				err = errors.New("Invalid Query Type")
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				logs.WithContext(r.Context()).Error(err.Error())
				return
			}
			isPublic := false
			isPublic, err = strconv.ParseBool(r.Header.Get("is_public"))
			if err != nil {
				// do nothing - silently execute with is_public as false
			}

			qlInterface.SetQLData(r.Context(), myQuery, postBody, true, tokenObj, isPublic, outputType)
			res, _, err = qlInterface.Execute(r.Context(), projectID, datasources, s, outputType)
			/*
				if err != nil {
					server_handlers.FormatResponse(w, 400)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					logs.WithContext(r.Context()).Error(err.Error())
					return
				}
			*/
		} else {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": errors.New(fmt.Sprint("query ", queryName, " not found")).Error()})
			return
		}
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			if res == nil {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else if outputType == eru_writes.OutputTypeExcel {
			ewd := eru_writes.ExcelWriteData{}
			ewd.ColumnarDataHeaderFirstRow = true
			//cf := eru_writes.CellFormatter{}
			//cf.DataTypes = []string{"int", "string", "boolean", "float", "date"}
			//ewd.CellFormat = cf

			var b []byte // creates IO Writer
			for _, v := range res {
				for k, excelData := range v {
					if records, ok := excelData.([][]interface{}); ok {
						if ewd.ColumnarDataMap == nil {
							ewd.ColumnarDataMap = make(map[string][][]interface{})
						}
						ewd.ColumnarDataMap[k] = records
					} else {
						err = errors.New(fmt.Sprint("incorrect excel data format"))
						server_handlers.FormatResponse(w, 400)
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
					}
				}
			}

			b, err = ewd.WriteColumnar(r.Context())
			if err != nil {
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			}

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			w.Header().Set("Content-Disposition", "attachment; filename=query.xlsx")
			_, _ = io.Copy(w, bytes.NewReader(b))
			return
		} else if outputType == eru_writes.OutputTypeCsv {
			b := &bytes.Buffer{} // creates IO Writer
			ww := csv.NewWriter(b)
			for _, v := range res {
				for _, csvData := range v {
					if records, ok := csvData.([][]interface{}); ok {
						var csvStrData [][]string
						tmpArray, tmpErr := json.Marshal(records)
						if tmpErr != nil {
							err = tmpErr
							server_handlers.FormatResponse(w, 400)
							_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
						}
						tmpErr = json.Unmarshal(tmpArray, &csvStrData)
						if tmpErr != nil {
							err = tmpErr
							server_handlers.FormatResponse(w, 400)
							_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
						}
						ww.WriteAll(csvStrData)
					} else {
						err = errors.New(fmt.Sprint("inccorect csv data format"))
						server_handlers.FormatResponse(w, 400)
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
					}
				}
			}

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", "attachment; filename=query.csv")
			_, _ = io.Copy(w, bytes.NewReader(b.Bytes()))
			return
		} else {
			server_handlers.FormatResponse(w, 200)
		}
		_ = json.NewEncoder(w).Encode(res)
		//logs.WithContext(r.Context()).Info(fmt.Sprint("---------------------------"))
		//logs.WithContext(r.Context()).Info(fmt.Sprint(w.Header()))
		return
	}
}

func GraphqlExecuteHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("GraphqlExecuteHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		outputType := vars["outputtype"]
		projectConfig, err := s.GetProjectConfigObject(r.Context(), projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenObj := make(map[string]interface{})
		tokenStr := r.Header.Get(projectConfig.TokenSecret.HeaderKey)

		if tokenStr != "" {
			err = json.Unmarshal([]byte(tokenStr), &tokenObj)
			if err != nil {
				logs.WithContext(r.Context()).Error(fmt.Sprint("error while unmarshalling token claim : ", err.Error()))
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}
		datasources, err := s.GetDataSources(r.Context(), projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		var gqd ql.GraphQLData
		gqd.IsPublic = false
		gqd.IsPublic, err = strconv.ParseBool(r.Header.Get("is_public"))
		if err != nil {
			// do nothing - silently execute with is_public as false
		}

		if err := json.NewDecoder(r.Body).Decode(&gqd); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			logs.WithContext(r.Context()).Error(err.Error())
			return
		}

		if gqd.Variables == nil {
			gqd.Variables = make(map[string]interface{})
		}
		gqd.Variables[module_model.RULEPREFIX_TOKEN] = tokenObj
		gqd.FinalVariables = gqd.Variables
		gqd.ExecuteFlag = true

		res, queryObjs, err := gqd.Execute(r.Context(), projectID, datasources, s, outputType)
		_ = queryObjs
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			if res == nil {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			server_handlers.FormatResponse(w, 200)
		}
		_ = json.NewEncoder(w).Encode(res)
		return
	}
}

func SqlExecuteHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("SqlExecuteHandler - Start")
		vars := mux.Vars(r)
		projectID := vars["project"]
		outputType := vars["outputtype"]
		projectConfig, err := s.GetProjectConfigObject(r.Context(), projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenObj := make(map[string]interface{})
		tokenStr := r.Header.Get(projectConfig.TokenSecret.HeaderKey)
		if tokenStr != "" {
			err = json.Unmarshal([]byte(tokenStr), &tokenObj)
			if err != nil {
				logs.WithContext(r.Context()).Error(fmt.Sprint("error while unmarshalling token claim : ", err.Error()))
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}

		datasources, err := s.GetDataSources(r.Context(), projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		var sqd ql.SQLData
		sqd.IsPublic = false
		sqd.IsPublic, err = strconv.ParseBool(r.Header.Get("is_public"))
		if err != nil {
			// do nothing - silently execute with is_public as false
		}

		if err := json.NewDecoder(r.Body).Decode(&sqd); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			logs.WithContext(r.Context()).Error(err.Error())
			return
		}

		if sqd.Variables == nil {
			sqd.Variables = make(map[string]interface{})
		}
		sqd.Variables[module_model.RULEPREFIX_TOKEN] = tokenObj
		sqd.FinalVariables = sqd.Variables
		sqd.ExecuteFlag = true
		res, queryObjs, err := sqd.Execute(r.Context(), projectID, datasources, s, outputType)
		_ = queryObjs
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			if res == nil {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		} else {
			server_handlers.FormatResponse(w, 200)
		}
		_ = json.NewEncoder(w).Encode(res)
		return
	}
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hello" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}
	res := make(map[string]string)
	res["Hellow"] = "World"
	server_handlers.FormatResponse(w, 200)
	_ = json.NewEncoder(w).Encode(res)
}

func EchoHandler(w http.ResponseWriter, r *http.Request) {
	res := make(map[string]interface{})
	res["Host"] = r.Host
	res["Header"] = r.Header
	res["URL"] = r.URL
	res["Body"] = r.Body
	res["Method"] = r.Method
	res["MultipartForm"] = r.MultipartForm
	res["RequestURI"] = r.RequestURI
	server_handlers.FormatResponse(w, 200)
	_ = json.NewEncoder(w).Encode(res)

}
