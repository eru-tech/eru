package ql

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
	"log"
	"reflect"
	"strconv"
	"strings"
)

type GraphQLData struct {
	QLData
	Operation   string                 `json:"operation"`
	QueryObject map[string]QueryObject `json:"_"`
}

var KeyWords = []string{"null"}

func (gqd *GraphQLData) SetQLData(mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool, tokenObj map[string]interface{}, isPublic bool, outputType string) {
	gqd.SetQLDataCommon(mq, vars, executeFlag, tokenObj, isPublic, outputType)
	//gqd.Query=mq.Query
	//gqd.Variables=mq.Vars
	//gqd.SetFinalVars(vars)
}

/*
	func (gqd *GraphQLData) CheckIfMutationByQuery() (selectQuery []string, err error) {
		log.Print("inside CheckIfMutationByQuery __________________________________________________________")
		doc, err := gqd.parseGraphQL()
		if err != nil {
			log.Print(err)
			return nil, err
		}
		for _, docDef := range doc.Definitions {
			op := ast.Node(docDef).(*ast.OperationDefinition)
			switch op.Operation {
			case "mutation":
				for _, v := range op.SelectionSet.Selections {
					field := v.(*ast.Field)
					for _, ff := range field.Arguments {
						if ff.Name.Value == "query" {
							selectQuery = append(selectQuery, ff.Value.GetValue().(string))
						}
					}
				}
			default:
				//do nothing
			}
		}
		log.Print(selectQuery)
		return selectQuery, err
	}
*/
func (gqd *GraphQLData) parseGraphQL() (d *ast.Document, err error) {
	s := source.NewSource(&source.Source{
		Body: []byte(gqd.Query),
		Name: gqd.Operation,
	})
	d, err = parser.Parse(parser.ParseParams{Source: s})
	if err != nil {
		log.Print(err)
		return nil, err
	}
	return d, err
}

func (gqd *GraphQLData) getSqlForQuery(projectId string, datasources map[string]*module_model.DataSource, query string, s module_store.ModuleStoreI, tokenObj map[string]interface{}, isPublic bool) (err error) {
	log.Print("query = ", query)
	mq, err := s.GetMyQuery(projectId, query)
	if err != nil {
		log.Print(err)
		return err
	}
	//mq == nil changed to below
	if mq.QueryName == "" {
		log.Print("------------")
		err = errors.New(fmt.Sprint("Query ", query, " not found"))
		log.Print(err)
		return err
	}
	log.Print("mq == ", mq)
	qlInterface := GetQL(mq.QueryType)
	if qlInterface == nil {
		err = errors.New("Invalid Query Type")
		log.Print(err)
		return err
	}
	qlInterface.SetQLData(mq, gqd.FinalVariables, false, tokenObj, isPublic, "") //passing false as we only need the query in execute function and not actual result
	_, queryObjs, err := qlInterface.Execute(projectId, datasources, s)
	//log.Print("queryObjs[0].Type ==", queryObjs[0].Type)
	for i, q := range queryObjs {
		queryObjs[i].Type = strings.ToUpper(strings.Split(q.Query, " ")[0])
	}
	//log.Print(queryObjs[0].Type)
	if gqd.QueryObject == nil {
		gqd.QueryObject = make(map[string]QueryObject)
	}
	gqd.QueryObject[query] = queryObjs[0] // setting first result as query used in mutation select will usually be single query
	log.Print("queryObjs[0] = ", queryObjs[0])
	//log.Print("gqd.QueryObject[query]")
	//log.Print(gqd.QueryObject[query])
	//log.Print("gqd.MutationSelect")
	//log.Print(gqd.QueryObject)
	return nil
}

func (gqd *GraphQLData) Execute(projectId string, datasources map[string]*module_model.DataSource, s module_store.ModuleStoreI) (res []map[string]interface{}, queryObjs []QueryObject, err error) {
	log.Print("inside Execute of GraphQL")
	singleTxn := false
	errFound := false
	doc, err := gqd.parseGraphQL()
	if err != nil {
		log.Print(err)
		return nil, nil, err
	}
	//doc := ast.Node(d).(*ast.Document)
	for _, docDef := range doc.Definitions {
		op := ast.Node(docDef).(*ast.OperationDefinition)
		graphQLs := make([]ds.SqlMakerI, len(op.SelectionSet.Selections))
		//graphQLs := make([]ds.SqlMakerI, len(op.SelectionSet.Selections))
		var result map[string]interface{}
		var results []map[string]interface{}
		for _, v := range op.Directives {
			if v.Name.Value == "singleTxn" {
				singleTxn = true
			}
			log.Print(v.Name.Value)
		}
		var returnAliasStrings []string
		var mainAliasNames []string
		breakForLoop := false
		for i, v := range op.SelectionSet.Selections {
			if breakForLoop {
				break
			}
			errMsg := "-"
			openTxn := false
			closeTxn := false
			queryObj := QueryObject{}
			if singleTxn && i == 0 {
				openTxn = true
			}
			if singleTxn && i == len(op.SelectionSet.Selections)-1 {
				closeTxn = true
			}
			//TODO - to handle if no directive/dbalias is received - conn is not getting closed as there is panic error in below line
			dbAlias := v.(*ast.Field).Directives[0].Name.Value
			datasource := datasources[dbAlias]
			if datasource == nil {
				return nil, nil, errors.New(fmt.Sprint("dbAlias ", dbAlias, " not found"))
			}

			if i == 0 {
				graphQLs[i] = ds.GetSqlMaker(datasource.DbName)
			} else if singleTxn {
				graphQLs[i] = graphQLs[0]
			} else {
				graphQLs[i] = ds.GetSqlMaker(datasource.DbName)
			}

			switch op.Operation {
			case "query":
				sqlObj := SQLObjectQ{}
				sqlObj.ProjectId = projectId
				sqlObj.FinalVariables = gqd.QLData.FinalVariables
				field := v.(*ast.Field)
				sqlObj.MainTableName = strings.Replace(field.Name.Value, "___", ".", -1) //replacing schema___tablename with schema.tablename

				if sqlObj.OverwriteDoc == nil {
					sqlObj.OverwriteDoc = make(map[string]map[string]interface{})
				}

				sqlObj.OverwriteDoc[sqlObj.MainTableName], err = gqd.setOverwriteDoc(projectId, dbAlias, sqlObj.MainTableName, s, op.Operation, module_model.QUERY_TYPE_SELECT)
				if err != nil {
					errMsg = err.Error()
					errFound = true
				}

				if sqlObj.SecurityClause == nil {
					sqlObj.SecurityClause = make(map[string]string)
				}
				sqlObj.SecurityClause[sqlObj.MainTableName], err = getTableSecurityRule(projectId, dbAlias, sqlObj.MainTableName, s, op.Operation, gqd.FinalVariables)
				if err != nil {
					errMsg = err.Error()
					errFound = true
				}

				err = sqlObj.ProcessGraphQL(v, datasource, graphQLs[i], gqd.FinalVariables, s) //TODO to handle if err recd.

				queryObj.Query = sqlObj.DBQuery
				queryObj.Cols = strings.Join(sqlObj.Columns.ColNames, " , ")
				mainAliasNames = append(mainAliasNames, sqlObj.MainAliasName)
				if gqd.ExecuteFlag {
					qrm := module_model.QueryResultMaker{}
					qrm.MainTableName = sqlObj.MainTableName
					qrm.MainAliasName = sqlObj.MainAliasName
					qrm.Tables = sqlObj.tables
					qrm.QueryLevel = sqlObj.queryLevel
					qrm.QuerySubLevel = sqlObj.querySubLevel
					qrm.SQLQuery = sqlObj.DBQuery

					if gqd.OutputType == "csv" {
						result, err = graphQLs[i].ExecuteQueryForCsv(qrm.SQLQuery, datasource)
						log.Print(err)
					} else {
						result, err = graphQLs[i].ExecuteQuery(datasource, qrm)
					}
					if err != nil {
						log.Print("error printed below fromc all of ExecuteQuery")
						log.Print(err)
						errMsg = err.Error()
						errFound = true
					}
					//log.Print("result is printed below")
					//log.Print(result)
					if result != nil {
						res = append(res, result)
					}
				}
			case "mutation":
				sqlObj := SQLObjectM{}
				field := v.(*ast.Field)
				tempStr := strings.SplitN(field.Name.Value, "_", 2)
				log.Print("tempStr = ", tempStr)
				sqlObj.QueryType = tempStr[0]
				sqlObj.MainTableName = strings.Replace(tempStr[1], "___", ".", -1)
				//log.Print("sqlObj.MainTableName = ", sqlObj.MainTableName)
				if sqlObj.OverwriteDoc == nil {
					sqlObj.OverwriteDoc = make(map[string]map[string]interface{})
				}
				sqlObj.OverwriteDoc[sqlObj.MainTableName], err = gqd.setOverwriteDoc(projectId, dbAlias, sqlObj.MainTableName, s, op.Operation, sqlObj.QueryType)
				if err != nil {
					errMsg = err.Error()
					log.Print(err)
					errFound = true
				}
				/*
					tr, err := s.GetTableTransformation(projectId, dbAlias, sqlObj.MainTableName)
					if err != nil {
						log.Print("error from GetTableTransformation = ", err.Error())
						errMsg = fmt.Sprint("error from GetTableTransformation = ", err.Error())
						errFound = true
					}
					log.Print(tr)
					if sqlObj.OverwriteDoc == nil {
						sqlObj.OverwriteDoc = make(map[string]map[string]interface{})
					}
					sqlObj.OverwriteDoc[sqlObj.MainTableName], err = gqd.ProcessTransformRule(tr.TransformInput)
					if err != nil {
						log.Print(err)
						errMsg = fmt.Sprint("TransformRule failed : ", err.Error())
						errFound = true
					}
					log.Print("sqlObj.OverwriteDoc")
					log.Print(sqlObj.OverwriteDoc)
				*/
				err = gqd.getSqlForQuery(projectId, datasources, sqlObj.MainTableName, s, nil, gqd.IsPublic)
				if err == nil {
					sqlObj.PreparedQuery = true
				}
				//log.Print("sqlObj.QueryType == ", sqlObj.QueryType)
				//log.Print("gqd.QueryObject[sqlObj.MainTableName].Type == ", gqd.QueryObject[sqlObj.MainTableName].Type)
				// TODO - commented below to handle WITH queries - to reconsider if below check is needed
				//if sqlObj.PreparedQuery && strings.ToUpper(sqlObj.QueryType) != gqd.QueryObject[sqlObj.MainTableName].Type {
				//	errFound = true
				//	err = errors.New(fmt.Sprint(sqlObj.MainTableName, " query is not of type ", gqd.QueryObject[sqlObj.MainTableName].Type))
				//	errMsg = err.Error()
				//}
				selectQuery := ""
				for _, ff := range field.Arguments {
					if ff.Name.Value == "query" {
						selectQuery = ff.Value.GetValue().(string)
					}
				}
				//log.Print(selectQuery)
				if selectQuery != "" {
					err = gqd.getSqlForQuery(projectId, datasources, selectQuery, s, nil, gqd.IsPublic)
					//todo consider passing token from finalvariables
					if err != nil {
						errFound = true
						log.Print(err)
						errMsg = err.Error()
					}
				}
				//log.Print("singleTxn === ", singleTxn)
				sqlObj.SingleTxn = singleTxn
				sqlObj.openTxn = openTxn
				sqlObj.closeTxn = closeTxn
				sqlObj.QueryObject = gqd.QueryObject
				err = sqlObj.ProcessMutationGraphQL(v, gqd.FinalVariables, datasource)
				log.Print("error in ProcessMutationGraphQL printed below")
				log.Print(err)
				//TODO connection close on this error - to handle the same
				mainAliasNames = append(mainAliasNames, sqlObj.MainAliasName)
				//log.Print("sqlObj.QueryType == ", sqlObj.QueryType)
				//TODO to loop on MutationRecords and pass query
				//queryObj.Query = sqlObj.DBQuery
				log.Print("gqd.ExecuteFlag && !errFound = ", gqd.ExecuteFlag, " ", !errFound)
				if gqd.ExecuteFlag && !errFound {
					//TODO can remove this mrm object and directly set values to graphQLs[i]
					log.Print("inside gqd.ExecuteFlag && !errFound")
					mrm := module_model.MutationResultMaker{}
					mrm.MainTableName = sqlObj.MainTableName
					mrm.MainAliasName = sqlObj.MainAliasName
					mrm.MutationReturn = sqlObj.MutationReturn
					mrm.MutationRecords = sqlObj.MutationRecords
					mrm.SingleTxn = sqlObj.SingleTxn
					mrm.OpenTxn = sqlObj.openTxn
					mrm.CloseTxn = sqlObj.closeTxn
					mrm.TxnFlag = sqlObj.TxnFlag
					mrm.IsNested = sqlObj.NestedDoc
					mrm.QueryType = sqlObj.QueryType
					mrm.DBQuery = sqlObj.DBQuery
					mrm.PreparedQuery = sqlObj.PreparedQuery
					//log.Println("mrm printed below after conversion from sqlobj")
					//log.Println(mrm)
					results, err = graphQLs[i].ExecuteMutationQuery(datasource, graphQLs[i], mrm)
					if err != nil {
						errFound = true
						log.Print(err.Error())
						errMsg = err.Error()
						// no need to return here - error is returned as part of result - if asked in the query.
					}
				} else if errFound {
					rollBackErr := graphQLs[i].RollbackQuery()
					if rollBackErr != nil {
						log.Print("rollBackErr = ", rollBackErr)
						errMsg = rollBackErr.Error()
					}
					breakForLoop = true
				}
				returnResult := make(map[string]interface{})
				if sqlObj.MutationReturn.ReturnError {
					returnResult[sqlObj.MutationReturn.ReturnErrorAlias] = errMsg
				}
				if sqlObj.MutationReturn.ReturnDoc {
					returnResult[sqlObj.MutationReturn.ReturnDocAlias] = results
				}
				resObj := make(map[string]interface{})
				returnAliasStrings = append(returnAliasStrings, sqlObj.MutationReturn.ReturnDocAlias)
				resObj[mainAliasNames[i]] = returnResult
				res = append(res, resObj)

				log.Print("err err err err")
				log.Print(err)

			default:
				log.Print(op.Operation)
				//do nothing
			}
			queryObjs = append(queryObjs, queryObj)
		}
		if errFound && singleTxn {
			for i, _ := range res {
				log.Print("returnAliasStrings[i] == ", i, " ", returnAliasStrings[i])
				obj, check := res[i][mainAliasNames[i]].(map[string]interface{})
				if check {
					if obj[returnAliasStrings[i]] != nil {
						obj[returnAliasStrings[i]] = make(map[string]interface{})
					}
				}
			}
			return res, queryObjs, errors.New("ERROR")
		}
	}
	//log.Print(res)
	//log.Print(queryObjs)
	return res, queryObjs, err
}

// parseAstValue returns an interface that can be casted to string
func ParseAstValue(value ast.Value, vars map[string]interface{}) (interface{}, error) {
	//log.Println("inside ParseAstValue for ")
	//log.Print(value.GetKind())
	switch value.GetKind() {
	case kinds.ObjectValue:

		o := map[string]interface{}{}
		obj := value.(*ast.ObjectValue)
		for _, v := range obj.Fields {
			temp, err := ParseAstValue(v.Value, vars)
			if err != nil {
				return nil, err
			}
			o[adjustObjectKey(v.Name.Value)] = temp
		}
		return o, nil

	case kinds.ListValue:
		listValue := value.(*ast.ListValue)
		array := make([]interface{}, len(listValue.Values))
		for i, v := range listValue.Values {
			val, err := ParseAstValue(v, vars)
			if err != nil {
				return nil, err
			}
			array[i] = val
		}
		return array, nil

	case kinds.EnumValue:
		v := value.(*ast.EnumValue).Value
		if strings.Contains(v, "__") {
			v = strings.ReplaceAll(v, "__", ".")
		}
		/*
			val, err := LoadValue(v, store)
			if err == nil {
				return val, nil
			}
		*/

		return v, nil

	case kinds.StringValue:
		v := value.(*ast.StringValue).Value
		if strings.Contains(v, "__") {
			v = strings.ReplaceAll(v, "__", ".")
		}

		for varsK, varsV := range vars {
			if str, ok := varsV.(string); ok {
				v = strings.ReplaceAll(v, fmt.Sprint("$", varsK), str)
			}
		}
		vBytes, err := processTemplate("variable", v, vars, "string", "")
		if err != nil {
			log.Print(err)
			return nil, err
		}
		if string(vBytes) != "" {
			v = string(vBytes)
		}
		log.Print(v)
		/*
			val, err := LoadValue(v, store)
			if err == nil {
				return val, nil
			}
		*/
		return v, nil

	case kinds.IntValue:
		intValue := value.(*ast.IntValue)

		// Convert string to int
		val, err := strconv.Atoi(intValue.Value)
		if err != nil {
			return nil, err
		}

		return val, nil

	case kinds.FloatValue:
		floatValue := value.(*ast.FloatValue)

		// Convert string to int
		//log.Print("floatValue.Value ==", floatValue.Value)
		val, err := strconv.ParseFloat(floatValue.Value, 64)
		if err != nil {
			return nil, err
		}

		return val, nil

	case kinds.BooleanValue:
		boolValue := value.(*ast.BooleanValue)
		return boolValue.Value, nil

	case kinds.Variable:
		t := value.(*ast.Variable)
		//log.Println("t = ", t.Name.Value)
		if strings.HasPrefix(t.Name.Value, "$") {
			log.Println("key has $ prefix")
		}
		return replaceVariableValue(t.Name.Value, vars)

	default:
		return nil, errors.New("Invalid data type `" + value.GetKind() + "` for value " + string(value.GetLoc().Source.Body)[value.GetLoc().Start:value.GetLoc().End])
	}
}

func replaceVariableValue(varName string, vars map[string]interface{}) (res interface{}, err error) {

	for _, kw := range KeyWords {
		if kw == varName {
			return fmt.Sprint("$", varName), nil
		}
	}

	if vars[varName] == nil {
		return nil, errors.New(fmt.Sprint("Variable value not found for '", varName, "'"))
	}
	switch reflect.TypeOf(vars[varName]).Kind() {
	case reflect.Slice:
		m := vars[varName].([]interface{})
		for i, v := range m {
			switch reflect.TypeOf(v).Kind() {
			case reflect.Map:
				m[i], err = processMapVariable(v.(map[string]interface{}), vars)
			default:
				// do nothing
			}
		}
		res = vars[varName]
	case reflect.Map:
		m := vars[varName].(map[string]interface{})
		res, err = processMapVariable(m, vars)
	default:
		res = vars[varName]
	}
	return res, err
}

func processMapVariable(m map[string]interface{}, vars map[string]interface{}) (interface{}, error) {
	var err error
	for k, v := range m {
		mapKey := k
		//log.Println("k = ", mapKey)
		if strings.HasPrefix(k, "$") {
			tempI, err := replaceVariableValue(strings.Replace(mapKey, "$", "", 1), vars)
			log.Print(err)
			if err == nil {
				log.Print(reflect.TypeOf(tempI).Kind())
				switch reflect.TypeOf(tempI).Kind() {
				case reflect.String, reflect.Float64, reflect.Int64:
					mapKey = fmt.Sprint(tempI)
				default:
					mapKey = tempI.(string)
				}
				m[mapKey] = m[k]
				delete(m, k)
			}
		}

		if v != nil {
			switch reflect.TypeOf(v).Kind() {
			case reflect.Map:
				m[mapKey], err = processMapVariable(v.(map[string]interface{}), vars)
			case reflect.Slice:
				s := v.([]interface{})
				for ii, sv := range s {
					switch reflect.TypeOf(sv).Kind() {
					case reflect.Map:
						s[ii], err = processMapVariable(sv.(map[string]interface{}), vars)
					default:
						// do nothing
					}
				}
			case reflect.String:
				if strings.HasPrefix(v.(string), "$") {
					tArray := strings.Split(v.(string), " ")
					tempI, err := replaceVariableValue(strings.Replace(tArray[0], "$", "", 1), vars)
					log.Print(err)
					if err == nil {
						log.Print(reflect.TypeOf(tempI).Kind())
						switch reflect.TypeOf(tempI).Kind() {
						case reflect.String, reflect.Float64, reflect.Int64:
							tArray[0] = fmt.Sprint(tempI)
							m[mapKey] = strings.Join(tArray, " ")
						default:
							m[mapKey] = tempI
						}
					} else {
						log.Print("removing key ", mapKey, " from list of variables")
						delete(m, mapKey)
					}
					log.Print("variable ", v.(string), " replace with ", m[mapKey])
				} else {
					vBytes, err := processTemplate("variable", v.(string), vars, "string", "")
					if err != nil {
						log.Print(err)
						return nil, err
					}
					if string(vBytes) == "" {
						m[mapKey] = v.(string)
					} else {
						m[mapKey] = string(vBytes)
					}

				}
			default:
				// do nothing - return as is
			}
		}
	}
	return m, err
}

func adjustObjectKey(key string) string {
	if strings.HasPrefix(key, "_") && key != "_id" {
		key = "$" + key[1:]
	}

	key = strings.ReplaceAll(key, "__", ".")

	return key
}

func (gqd *GraphQLData) setOverwriteDoc(projectId string, dbAlias string, tableName string, s module_store.ModuleStoreI, op string, queryType string) (overwriteDoc map[string]interface{}, err error) {
	log.Print("calling for GetTableTransformation = ", tableName)
	tr, err := s.GetTableTransformation(projectId, dbAlias, tableName)
	if err != nil {
		log.Print("error from GetTableTransformation = ", err.Error())
		err = errors.New(fmt.Sprint("error from GetTableTransformation = ", err.Error()))
		return nil, err
	}
	overwriteDoc = make(map[string]interface{})
	if op == "query" {
		overwriteDoc, err = gqd.ProcessTransformRule(tr.TransformOutput)
	} else if op == "mutation" {
		processTransformRule := false
		for _, v := range tr.TransformInput.ApplyOn {
			if v == queryType {
				processTransformRule = true
				break
			}
		}
		if processTransformRule {
			overwriteDoc, err = gqd.ProcessTransformRule(tr.TransformInput)
		}
	} else {
		err = errors.New(fmt.Sprint("Invalid Operation : ", op))
		log.Print(err)
	}

	if err != nil {
		log.Print(err)
		err = errors.New(fmt.Sprint("TransformRule failed : ", err.Error()))
		return nil, err
	}
	return
}

func getTableSecurityRule(projectId string, dbAlias string, tableName string, s module_store.ModuleStoreI, op string, vars map[string]interface{}) (ruleOutput string, err error) {
	sr, err := s.GetTableSecurityRule(projectId, dbAlias, tableName)
	if err != nil {
		err = errors.New(fmt.Sprint("error from getTableSecurityRule = ", err.Error()))
		log.Print(err)
		return "", err
	}

	if op == "query" {
		ruleOutput, err = processSecurityRule(sr.Select, vars)
	} else if op == "mutation" {
		ruleOutput, err = processSecurityRule(sr.Insert, vars) //todo to change it as per query type
	} else {
		err = errors.New(fmt.Sprint("Invalid Query Type : ", op))
		log.Print(err)
	}

	if err != nil {
		log.Print(err)
		err = errors.New(fmt.Sprint("SecurityRule failed : ", err.Error()))
		return "", err
	}
	return
}
