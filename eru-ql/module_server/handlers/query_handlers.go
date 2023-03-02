package handlers

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"github.com/eru-tech/eru/eru-ql/ql"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
	//"../server"
)

func ProjectMyQuerySaveHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		queryName := vars["queryname"]
		queryType := vars["querytype"]

		log.Print(projectID)
		var err error
		if queryType == "graphql" {
			var gqd ql.GraphQLData
			if err := json.NewDecoder(r.Body).Decode(&gqd); err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				log.Print(err)
				return
			}
			err = s.SaveMyQuery(projectID, queryName, queryType, "", gqd.Query, gqd.Variables, s, "", gqd.SecurityRule)
		} else if queryType == "sql" {
			var sqd ql.SQLData
			if err := json.NewDecoder(r.Body).Decode(&sqd); err != nil {
				server_handlers.FormatResponse(w, 400)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				log.Print(err)
				return
			}
			log.Print("r.Body == ")
			log.Print(r.Body)
			err = s.SaveMyQuery(projectID, queryName, queryType, sqd.DBAlias, sqd.Query, sqd.Variables, s, sqd.Cols, sqd.SecurityRule)
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
		vars := mux.Vars(r)
		projectID := vars["project"]
		queryName := vars["queryname"]
		log.Print(projectID)

		err := s.RemoveMyQuery(projectID, queryName, s)
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
		vars := mux.Vars(r)
		projectID := vars["project"]
		queryType := vars["querytype"]
		log.Print(projectID)

		myqueries, err := s.GetMyQueries(projectID, queryType)
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
		vars := mux.Vars(r)
		projectID := vars["project"]
		queryName := vars["queryname"]
		log.Print(projectID)

		myquery, err := s.GetMyQuery(projectID, queryName)
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
		log.Print(time.Now(), " Start --------------------------------------------------- ")
		//time.Sleep(time.Duration(3000) * time.Millisecond)
		log.Print(time.Now(), " End --------------------------------------------------- ")
		claims := r.Header.Get("claims")
		log.Println(claims)
		vars := mux.Vars(r)
		projectID := vars["project"]
		queryName := vars["queryname"]
		outputType := vars["outputtype"]
		log.Print("projectID = ", projectID)

		projectConfig, err := s.GetProjectConfigObject(projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenObj := make(map[string]interface{})
		tokenStr := r.Header.Get(projectConfig.TokenSecret.HeaderKey)
		log.Print("tokenStr = ", tokenStr)
		if tokenStr != "" {
			err = json.Unmarshal([]byte(tokenStr), &tokenObj)
			if err != nil {
				log.Print("error while unmarshalling token claim")
				log.Print(err)
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}

		postBody := make(map[string]interface{})

		if err := json.NewDecoder(r.Body).Decode(&postBody); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			log.Print(err)
			return
		}
		//log.Println(postBody)
		datasources, err := s.GetDataSources(projectID)
		if err != nil {
			log.Print(err.Error())
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		var res []map[string]interface{}
		//var queries []string
		myQuery, err := s.GetMyQuery(projectID, queryName)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			log.Print(err)
			return
		}
		// overwriting variables with same names
		if myQuery.QueryName != "" {
			qlInterface := ql.GetQL(myQuery.QueryType)
			if qlInterface == nil {
				server_handlers.FormatResponse(w, 400)
				err = errors.New("Invalid Query Type")
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				log.Print(err)
				return
			}
			isPublic := false
			isPublic, err = strconv.ParseBool(r.Header.Get("is_public"))
			if err != nil {
				// do nothing - silently execute with is_public as false
			}

			qlInterface.SetQLData(myQuery, postBody, true, tokenObj, isPublic, outputType)
			res, _, err = qlInterface.Execute(projectID, datasources, s)
			/*
				if err != nil {
					server_handlers.FormatResponse(w, 400)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					log.Print(err)
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
		} else if outputType == "csv" {
			//tmpFileName := fmt.Sprint(uuid.New().String(),".csv")
			//	csvFile, csvErr := os.Create(tmpFileName)
			//if csvErr != nil {
			//	err = errors.New(fmt.Sprint("failed creating csv file"))
			//	log.Print(csvErr)
			//		server_handlers.FormatResponse(w, 400)
			//		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			//	}
			//ww := csv.NewWriter(csvFile)
			b := &bytes.Buffer{} // creates IO Writer
			ww := csv.NewWriter(b)
			for _, v := range res {
				for _, csvData := range v {
					if records, ok := csvData.([][]string); ok {
						ww.WriteAll(records)
					} else {
						err = errors.New(fmt.Sprint("inccorect csv data format"))
						server_handlers.FormatResponse(w, 400)
						_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
					}
				}
			}
			//	defer func() {
			//		csvFile.Close()
			//		e := os.Remove(tmpFileName)
			//		if e != nil {
			//			log.Print(e)
			//		}
			//	}()
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", "attachment; filename=query.csv")
			_, _ = io.Copy(w, bytes.NewReader(b.Bytes()))
			return
		} else {
			server_handlers.FormatResponse(w, 200)
		}
		_ = json.NewEncoder(w).Encode(res)
		return
	}
}

func GraphqlExecuteHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		log.Print(projectID)

		projectConfig, err := s.GetProjectConfigObject(projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenObj := make(map[string]interface{})
		tokenStr := r.Header.Get(projectConfig.TokenSecret.HeaderKey)

		log.Print("tokenStr = ", tokenStr)
		if tokenStr != "" {
			err = json.Unmarshal([]byte(tokenStr), &tokenObj)
			if err != nil {
				log.Print("error while unmarshalling token claim")
				log.Print(err)
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}
		datasources, err := s.GetDataSources(projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		//var t interface{}
		//terr := json.NewDecoder(r.Body).Decode(&t)
		//if terr != nil {
		//	log.Print(terr.Error())
		//}
		//log.Print(t)

		var gqd ql.GraphQLData
		gqd.IsPublic = false
		gqd.IsPublic, err = strconv.ParseBool(r.Header.Get("is_public"))
		if err != nil {
			// do nothing - silently execute with is_public as false
		}
		log.Print("gqd.IsPublic = ", gqd.IsPublic)

		if err := json.NewDecoder(r.Body).Decode(&gqd); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			log.Print(err)
			return
		}
		//log.Print(gqd)
		if gqd.Variables == nil {
			gqd.Variables = make(map[string]interface{})
		}
		gqd.Variables[module_model.RULEPREFIX_TOKEN] = tokenObj
		gqd.FinalVariables = gqd.Variables
		gqd.ExecuteFlag = true
		/*
			queryNames, err := gqd.CheckIfMutationByQuery()
			log.Print(queryNames)
			for _, queryName := range queryNames {
				myQuery, err := s.GetMyQuery(projectID, queryName)
				if err != nil {
					server_handlers.FormatResponse(w, 400)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					log.Print(err)
					return
				}
				// overwriting variables with same names
				if myQuery != nil {
					qlInterface := ql.GetQL(myQuery.QueryType)
					if qlInterface == nil {
						server_handlers.FormatResponse(w, 400)
						err = errors.New("Invalid Query Type")
						json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
						log.Print(err)
						return
					}
					qlInterface.SetQLData(*myQuery, gqd.FinalVariables, false) //passing false as we only need the query in execute function and not actual result
					_, queryMap, err := qlInterface.Execute(datasources)
					if err != nil {
						server_handlers.FormatResponse(w, 400)
						json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
						log.Print(err)
						return
					}
					log.Print(queryMap)
					gqd.MutationSelect=queryMap[0] // picking up first element as it is assumed that query used for insert will only have 1 doc definition (thus one query only)
				} else {
					server_handlers.FormatResponse(w, 400)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": errors.New(fmt.Sprint("query ", queryName, " not found")).Error()})
					return
				}
			}
		*/
		res, queryObjs, err := gqd.Execute(projectID, datasources, s)
		_ = queryObjs
		//log.Print("queryObjs printed below")
		//log.Print(queryObjs)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			if res == nil {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			//_ = json.NewEncoder(w).Encode(map[string]interface{}{"errors": []interface{}{err.Error()}})
			//fmt.Fprintf(w, err.Error())
		} else {
			server_handlers.FormatResponse(w, 200)
		}
		_ = json.NewEncoder(w).Encode(res)
		return
	}
}

func SqlExecuteHandler(s module_store.ModuleStoreI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		projectID := vars["project"]
		log.Print(projectID)

		projectConfig, err := s.GetProjectConfigObject(projectID)
		if err != nil {
			server_handlers.FormatResponse(w, 400)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		tokenObj := make(map[string]interface{})
		tokenStr := r.Header.Get(projectConfig.TokenSecret.HeaderKey)
		log.Print("tokenStr = ", tokenStr)
		if tokenStr != "" {
			err = json.Unmarshal([]byte(tokenStr), &tokenObj)
			if err != nil {
				log.Print("error while unmarshalling token claim")
				log.Print(err)
				server_handlers.FormatResponse(w, 400)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
		}

		datasources, err := s.GetDataSources(projectID)
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
		log.Print("gqd.IsPublic = ", sqd.IsPublic)

		if err := json.NewDecoder(r.Body).Decode(&sqd); err != nil {
			server_handlers.FormatResponse(w, 400)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			log.Print(err)
			return
		}
		//log.Print(sqd)
		if sqd.Variables == nil {
			sqd.Variables = make(map[string]interface{})
		}
		sqd.Variables[module_model.RULEPREFIX_TOKEN] = tokenObj
		sqd.FinalVariables = sqd.Variables
		sqd.ExecuteFlag = true
		res, queryObjs, err := sqd.Execute(projectID, datasources, s)
		_ = queryObjs
		//log.Print("queryObjs printed below")
		//log.Print(queryObjs)
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
	log.Print("log testing")
	server_handlers.FormatResponse(w, 200)
	_ = json.NewEncoder(w).Encode(res)
}

func EchoHandler(w http.ResponseWriter, r *http.Request) {

	for k, v := range r.Header {
		log.Println(k, " = ", v)
		//w.Header()[k] = v
	}
	//w.WriteHeader(200)
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

	/*t, err := io.Copy(w, r.Body)
	if err != nil {
		log.Println("================")
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	log.Println(t)

	*/

}
