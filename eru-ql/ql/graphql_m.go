package ql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/graphql-go/graphql/language/ast"
	"reflect"
	"strings"
)

type SQLObjectM struct {
	MainTableName       string
	MainAliasName       string
	MutationRecords     []module_model.MutationRecord
	TxnFlag             bool
	QueryType           string
	MutationReturn      module_model.MutationReturn
	MutationSelectQuery string
	MutationSelectCols  string
	NestedDoc           bool
	SingleTxn           bool
	openTxn             bool
	closeTxn            bool
	//MainTableDB     string
	WhereClause interface{}
	QueryObject map[string]QueryObject
	DBQuery     string
	//SortClause      interface{}
	//JoinClause      map[string]interface{}
	//DistinctResults bool
	//HasAggregate    bool
	//Limit           int
	//Skip            int
	//Columns         SQLCols
	//tables          [][]module_model.Tables
	//tableNames      map[string]string
	//queryLevel      int
	//querySubLevel   []int
	PreparedQuery bool
	OverwriteDoc  map[string]map[string]interface{} `json:"-"`
}

func (sqlObj *SQLObjectM) ProcessMutationGraphQL(ctx context.Context, sel ast.Selection, vars map[string]interface{}, datasource *module_model.DataSource) (err error) {
	//myself.CheckMe()
	logs.WithContext(ctx).Debug("ProcessMutationGraphQL - Start")
	field := sel.(*ast.Field)
	docsFound := false
	if field.Alias != nil {
		sqlObj.MainAliasName = field.Alias.Value
	} else {
		sqlObj.MainAliasName = field.Name.Value
	}
	sqlObj.TxnFlag = true //default to true
	sqlObj.NestedDoc = false
	// resetting below variables as it is called in loop and
	// in case of SingleTxn, same sqlObj object is used for all records in loop
	sqlObj.MutationSelectQuery = ""
	sqlObj.MutationSelectCols = ""
	sqlObj.MutationRecords = nil

	sqlObj.MutationReturn.ReturnError = false  //default to false
	sqlObj.MutationReturn.ReturnDoc = false    //default to false
	sqlObj.MutationReturn.ReturnFields = " * " //set default to * - below code will override it if user has asked for specific fields only
	if field.SelectionSet != nil {
		for _, ss := range field.SelectionSet.Selections {
			field := ss.(*ast.Field)
			switch field.Name.Value {
			case "error":
				sqlObj.MutationReturn.ReturnError = true
				if field.Alias != nil {
					sqlObj.MutationReturn.ReturnErrorAlias = field.Alias.Value
				} else {
					sqlObj.MutationReturn.ReturnErrorAlias = field.Name.Value
				}
			case "returning":
				sqlObj.MutationReturn.ReturnDoc = true
				if field.Alias != nil {
					sqlObj.MutationReturn.ReturnDocAlias = field.Alias.Value
				} else {
					sqlObj.MutationReturn.ReturnDocAlias = field.Name.Value
				}
				retField := ss.(*ast.Field)
				//TODO to handle nested returning clause - to pass in insert query only fields of respective object.
				// Currently all fields are passing causing sql to fail
				if retField.SelectionSet != nil {
					var retFieldArray []string
					for _, rf := range retField.SelectionSet.Selections {
						rfNew := rf.(*ast.Field)
						retFieldArray = append(retFieldArray, rfNew.Name.Value)
					}
					sqlObj.MutationReturn.ReturnFields = strings.Join(retFieldArray, ",")
				}
				//else {
				//	sqlObj.MutationReturn.ReturnFields = " * "
				//}
			default:
				//do nothing
			}
		}
	}
	var docs interface{}

	for _, ff := range field.Arguments {
		switch ff.Name.Value {

		case "docs":
			docsFound = true
			varValue, e := ParseAstValue(ctx, ff.Value, vars)
			if e != nil {
				return e
			}
			docs = varValue

		case "query":
			varValue, err := ParseAstValue(ctx, ff.Value, vars)
			if err != nil {
				return err
			}
			sqlObj.MutationSelectQuery = sqlObj.QueryObject[varValue.(string)].Query
			sqlObj.MutationSelectCols = sqlObj.QueryObject[varValue.(string)].Cols
			sqlObj.QueryType = "insertselect"
			sqlObj.MakeMutationQuery(ctx, nil, sqlObj.MainTableName)
			logs.WithContext(ctx).Info(sqlObj.DBQuery)
			logs.WithContext(ctx).Info(fmt.Sprint("sqlObj.PreparedQuery = ", sqlObj.PreparedQuery))

			//docsFound = true
		case "txn":
			if !sqlObj.SingleTxn { //ignore txn flag for each query if singleTxn directive exists
				varValue, err := ParseAstValue(ctx, ff.Value, vars)
				if err != nil {
					return err
				}
				v, e := varValue.(bool)
				if !e {
					return errors.New("error while parsing value of 'txn'")
				}
				sqlObj.TxnFlag = v
			}
		case "where":
			v, e := ParseAstValue(ctx, ff.Value, vars)
			_ = v
			if e != nil {
				// TODO: return returnresult error
				return e
			}
			//wc, _ := sqlObj.processWhereClause(v, "", false)
			//sqlObj.WhereClause = fmt.Sprint(" where ", wc)
			sqlObj.WhereClause = v
		default:
			//do nothing
		}
	}
	if docsFound {
		sqlObj.MutationRecords, err = sqlObj.processMutationDoc(ctx, docs, datasource, sqlObj.MainTableName, sqlObj.NestedDoc, nil)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			//TODO to pass this error as query result
			// return nil, err
			// no need to return here
		}
	}
	if sqlObj.QueryType == "delete" || sqlObj.PreparedQuery {
		sqlObj.MutationRecords = make([]module_model.MutationRecord, 1) // dummy record added so that it enters for loop in ExecuteMutationQuery function
		sqlObj.MakeMutationQuery(ctx, &sqlObj.MutationRecords[0], sqlObj.MainTableName)
	} else if !docsFound {
		logs.WithContext(ctx).Warn("docs not found")
		return errors.New("missing 'docs' keyword - document to mutate not found") //TODO this error is not returned in graphql error
	}
	return nil
}

func (sqlObj *SQLObjectM) processMutationDoc(ctx context.Context, d interface{}, datasource *module_model.DataSource, parentTableName string, nested bool, jc []string) (mr []module_model.MutationRecord, e error) {
	logs.WithContext(ctx).Debug(fmt.Sprint("ProcessMutationGraphQL - Start : ", parentTableName, " ", nested))

	sqlObj.NestedDoc = nested // updating if recursive call is made
	docs, err := d.([]interface{})
	if !err {
		dd, er := d.(map[string]interface{}) // checking if docs is a single document without array
		if !er {
			return nil, errors.New("error while parsing value of 'docs'")
		}
		docs = append(docs, dd)
	} else if sqlObj.QueryType == "update" {
		return nil, errors.New("value of 'docs' cannot be an array")
	}
	mr = make([]module_model.MutationRecord, len(docs))
	for i, doc := range docs {

		insertDoc, err := doc.(map[string]interface{})
		if !err {
			return nil, errors.New(fmt.Sprintf("error while parsing document at index ", i))
		}

		for k, v := range sqlObj.OverwriteDoc[parentTableName] {
			insertDoc[k] = v
		}
		var jsonFields []string

		var updateCols []string
		//var colsPlaceholder []string
		var values []interface{}
		var childRecords []module_model.MutationRecord
		var cols []string
		var colsIfNotNested []string
		var valuesIfNotNested []interface{}

		//ii := 1
		mr[i] = module_model.MutationRecord{}
		colNo := 1
		for k, kv := range insertDoc {
			if i == 0 && !sqlObj.NestedDoc { // picking up columns only from first record in array as structure of all records should be same for non nested docs
				colsIfNotNested = append(colsIfNotNested, k)
			}
			isArray := false
			var a1 []interface{}
			a1, isArray = kv.([]interface{})         //check if value is array of nested object
			a2, isMap := kv.(map[string]interface{}) // check if value is single nested object (not an array)
			if isMap {
				a1 = append(a1, a2) // making single nested object into array of one element - to have common code to execute both conditions
				isArray = true
			}
			childTableName := strings.Replace(k, "___", ".", -1)
			tj, e := datasource.GetTableJoins(ctx, parentTableName, childTableName, nil)
			if e != nil {
				if isArray {
					kv1, ee := json.Marshal(kv)
					if ee != nil {
						logs.WithContext(ctx).Error(ee.Error())
						return nil, ee
					}
					kv = string(kv1)
					jsonFields = append(jsonFields, k)
					isArray = false
				}
			}
			if isArray {
				var joinCols []string
				if tj.Table1Name == parentTableName {
					joinCols = tj.Table1Cols
				} else {
					joinCols = tj.Table2Cols
				}
				//TODO ensure parent query has returning fields required for child join values
				if e != nil {
					return nil, e
				}
				childRecords, e = sqlObj.processMutationDoc(ctx, a1, datasource, childTableName, true, joinCols)
				if e != nil {
					logs.WithContext(ctx).Error(e.Error())
					return nil, e
				}
				if mr[i].ChildRecords == nil {
					mr[i].ChildRecords = make(map[string][]module_model.MutationRecord)
				}
				mr[i].ChildRecords[childTableName] = childRecords
				if mr[i].TableJoins == nil {
					mr[i].TableJoins = make(map[string]module_model.TableJoins)
				}
				mr[i].TableJoins[childTableName] = tj
			} else {
				cols = append(cols, k)
				if kv != nil {
					switch reflect.TypeOf(kv).Kind() {
					case reflect.String:
						str := kv.(string)
						if strings.HasPrefix(str, "FIELD_") {
							updateCols = append(updateCols, fmt.Sprint(k, " = ", fmt.Sprint(strings.Replace(str, "FIELD_", "", 1))))
						} else {
							updateCols = append(updateCols, fmt.Sprint(k, " = ", "$UpdateColPlaceholder", colNo))
							values = append(values, kv)
							colNo++
						}
					default:
						updateCols = append(updateCols, fmt.Sprint(k, " = ", "$UpdateColPlaceholder", colNo))
						values = append(values, kv)
						colNo++
					}
				} else {
					updateCols = append(updateCols, fmt.Sprint(k, " = ", "$UpdateColPlaceholder", colNo))
					values = append(values, kv)
					colNo++
				}
			}
		}
		colFound := false
		for _, jcol := range jc {
			for _, qcol := range cols {
				if jcol == qcol {
					colFound = true
					break
				}
			}
			if !colFound {
				cols = append(cols, jcol)
				values = append(values, nil)
			}
		}
		mr[i].Cols = strings.Join(cols, ",")
		mr[i].UpdatedCols = strings.Join(updateCols, ",")
		//mr[i].ColsPlaceholder = strings.Join(colsPlaceholder, ",")
		mr[i].ColsPlaceholder = "$ColsPlaceholder"
		mr[i].Values = values
		mr[i].NonNestedCols = strings.Join(colsIfNotNested, ",")
		for _, c := range strings.Split(mr[0].NonNestedCols, ",") {
			nonNestedValue := insertDoc[c]
			for _, jf := range jsonFields {
				if jf == c {
					nonNestedValue1, ee := json.Marshal(nonNestedValue)
					if ee != nil {
						logs.WithContext(ctx).Error(ee.Error())
						return
					}
					nonNestedValue = string(nonNestedValue1)
				}
			}
			valuesIfNotNested = append(valuesIfNotNested, nonNestedValue)
		}
		mr[i].NonNestedValues = valuesIfNotNested
		sqlObj.MakeMutationQuery(ctx, &mr[i], parentTableName)
	}
	if !sqlObj.NestedDoc && len(docs) > 0 {
		mr[0].Cols = mr[0].NonNestedCols
		sqlObj.MakeMutationQuery(ctx, &mr[0], parentTableName)
	}
	return mr, nil
}

func (sqlObj *SQLObjectM) MakeMutationQuery(ctx context.Context, doc *module_model.MutationRecord, tableName string) {
	logs.WithContext(ctx).Debug("MakeMutationQuery - Start")
	returningStr := ""
	strWhereClause, e := processWhereClause(ctx, sqlObj.WhereClause, "", sqlObj.MainTableName, false)
	if e != "" {
		logs.WithContext(ctx).Error(e)
		//TODO to return errors to main result
		//err := errors.New(e)
	}
	if strWhereClause != "" {
		strWhereClause = fmt.Sprint(" where ", strWhereClause)
	}
	//if sqlObj.MutationReturn.ReturnDoc { ### commented the conditional check to add returning clause - not we will always add
	//TODO to bring back conditional check
	if sqlObj.MutationReturn.ReturnDoc {
		returningStr = fmt.Sprint(" RETURNING ", sqlObj.MutationReturn.ReturnFields)
	}
	//}
	query := ""
	if sqlObj.PreparedQuery {
		query = fmt.Sprint(sqlObj.QueryObject[sqlObj.MainTableName].Query, " ", returningStr)
		sqlObj.DBQuery = query
		return
	}
	switch sqlObj.QueryType {
	case "insertselect":
		query = fmt.Sprint("insert into ", tableName, " ( ", sqlObj.MutationSelectCols, ") ", sqlObj.MutationSelectQuery, returningStr)
		sqlObj.DBQuery = query
	case "insert":

		/*for _, tjColVal := range tj.Table1Cols { //TODO test parent-child insertion extensively.
			colFound := false
			for colIdx, colVal := range cvColsArray {
				if tjColVal == colVal {
					colFound = true
					//colsPlaceholder = append(colsPlaceholder[:idx], colsPlaceholder[idx+1:]...)
					childRec.Values[colIdx] = resDoc[tjColVal]
					break
				}
			}
			if !colFound {
				cvColsArray = append(cvColsArray, tjColVal)
				//childRec.Values = append(childRec.Values, resDoc[tjColVal])
				cv[icv].Values = append(cv[icv].Values , resDoc[tjColVal])
				colsPlaceholder = append(colsPlaceholder, len(cvColsArray))
			}
		}
		*/

		query = fmt.Sprint("insert into ", tableName, " (", doc.Cols,
			") values ", doc.ColsPlaceholder, returningStr)
		doc.DBQuery = query
	case "update":
		query = fmt.Sprint("update ", tableName, " set ", doc.UpdatedCols,
			" ", strWhereClause, " ", returningStr)
		doc.DBQuery = query
	case "delete":
		query = fmt.Sprint("delete from ", tableName, " ", strWhereClause, " ", returningStr)
		sqlObj.DBQuery = query
	default:
		//do nothing
	}
}
