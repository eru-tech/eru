package ds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-security-rule/security_rule"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
	//"github.com/graphql-go/graphql/language/ast"
	//"github.com/graphql-go/graphql/language/kinds"
	"github.com/jmoiron/sqlx"
	"log"
	//"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type tablesInQuery struct {
	name   string
	nested bool
}

type SqlMaker struct {
	TestFlag      bool
	QueryType     string
	MainTableName string
	MainAliasName string
	TableNames    map[string]string
	//AllTableNamesNew [][]tablesInQuery
	MainTableDB     string
	WhereClause     string
	SortClause      string
	ColumnList      string
	DBColumns       string
	GroupList       string
	JoinClause      string
	ChildChange     string
	Limit           int
	Skip            int
	HasAggregate    bool
	DistinctResults bool
	queryLevel      int
	querySubLevel   []int
	tables          [][]module_model.Tables
	//resultHolder      []map[string]interface{}
	resultHolderNew [][]map[string]interface{}
	//resultIndexHolder []int
	resultIndexHolderNew [][]map[int]int
	result               map[string]interface{}
	//tempRA               []map[int]int
	//tempA                map[int]int
	//t                    int
	//InsertData     []map[string]interface{}
	MutationRecords     []module_model.MutationRecord
	TxnFlag             bool
	MutationReturn      module_model.MutationReturn
	MutationSelectQuery string
	MutationSelectCols  string
	SingleTxn           bool
	openTxn             bool
	closeTxn            bool
	tx                  *sqlx.Tx
	IsNested            bool
	DBQuery             string
	PreparedQuery       bool
}

type GraphqlResult struct {
	gqlr map[string]interface{}
	idx  int
}

var DefaultDriverConfig = module_model.DriverConfig{10, 2, time.Hour}
var DefaultOtherConfig = module_model.OtherDbConfig{1000, 60, false}
var emptyCustomRule = security_rule.CustomRule{}
var DefaultDbSecurityRules = module_model.SecurityRules{security_rule.SecurityRule{"Allow", emptyCustomRule}, security_rule.SecurityRule{"Deny", emptyCustomRule}, security_rule.SecurityRule{"Allow", emptyCustomRule}, security_rule.SecurityRule{"Allow", emptyCustomRule}, security_rule.SecurityRule{"Allow", emptyCustomRule}, security_rule.SecurityRule{"Deny", emptyCustomRule}, security_rule.SecurityRule{"Allow", emptyCustomRule}, security_rule.SecurityRule{"Allow", emptyCustomRule}}

type SqlMakerI interface {
	GetReturnAlias() string
	GetBaseSqlMaker() *SqlMaker
	//ProcessGraphQL(sel ast.Selection, vars map[string]interface{}, myself SqlMakerI, datasource *model.DataSource, ExecuteFlag bool) (res map[string]interface{}, query string, cols string , err error)
	//ProcessMutationGraphQL(sel ast.Selection, vars map[string]interface{}, myself SqlMakerI, datasource *model.DataSource, singleTxn bool, openTxn bool, closeTxn bool, query string, cols string) (res map[string]interface{}, err error)
	CheckMe()
	MakeQuery() (dbQuery string, err string)
	//MakeSQLQuery(sqlObject ql.SQLObject) (dbQuery string, err string)
	//MakeMutationQuery(idx int, docs []MutationRecord, tableName string) (dbQuery string)
	AddLimitSkipClause(query string, limit int, skip int, globalLimit int) (newQuery string)
	CreateConn(dataSource *module_model.DataSource) error
	ExecuteQuery(datasource *module_model.DataSource, qrm module_model.QueryResultMaker) (res map[string]interface{}, err error)
	ExecuteMutationQuery(datasource *module_model.DataSource, myself SqlMakerI, mrm module_model.MutationResultMaker) (res []map[string]interface{}, err error)
	ExecutePreparedQuery(query string, datasource *module_model.DataSource) (res map[string]interface{}, err error)
	ExecuteQueryForCsv(query string, datasource *module_model.DataSource, aliasName string) (res map[string]interface{}, err error)
	RollbackQuery() (err error)
	GetTableList(query string, datasource *module_model.DataSource, myself SqlMakerI) (err error)
	GetTableMetaDataSQL() string
	MakeCreateTableSQL(tableName string, tableObj map[string]module_model.TableColsMetaData) (string, error)
	MakeDropTableSQL(tableName string) (string, error)
	getDataTypeMapping(dataType string) string
	GetSqlResult() map[string]interface{}
	GetPreparedQueryPlaceholder(rowCount int, colCount int, single bool) string
	//CreateConn() error
}

func (sqr *SqlMaker) GetReturnAlias() string {
	return sqr.MutationReturn.ReturnDocAlias
}

func (sqr *SqlMaker) GetPreparedQueryPlaceholder(rowCount int, colCount int, single bool) string {
	return strings.Repeat(" ? ", colCount*rowCount)
}

func (sqr *SqlMaker) GetBaseSqlMaker() *SqlMaker {
	return sqr
}
func (sqr *SqlMaker) GetTableMetaDataSQL() string {
	return ""
}

func (sqr *SqlMaker) MakeCreateTableSQL(tableName string, tableObj map[string]module_model.TableColsMetaData) (string, error) {
	return "", nil
}
func (sqr *SqlMaker) MakeDropTableSQL(tableName string) (string, error) {
	return "", nil
}

func (sqr *SqlMaker) GetSqlResult() map[string]interface{} {
	return sqr.result
}

func (sqr *SqlMaker) CreateConn(dataSource *module_model.DataSource) error {
	return errors.New("CreateConn not implemented")
}

func (sqr *SqlMaker) CheckMe() {
	log.Print("I am SqlMaker")
}

/*
func (sqr *SqlMaker) processMutationDoc(d interface{}, myself SqlMakerI, datasource *module_model.DataSource, parentTableName string) (mr []MutationRecord, e error) {
	docs, err := d.([]interface{})
	if !err {
		_, er := d.(map[string]interface{}) // checking if docs is a single document without array
		if !er {
			return nil, errors.New("error while parsing value of 'docs'")
		}
		docs = append(docs, d)
	} else if sqr.QueryType == "update" {
		return nil, errors.New("value of 'docs' cannot be an array")
	}

	mr = make([]MutationRecord, len(docs))
	for i, doc := range docs {
		var cols []string
		var updateCols []string
		var colsPlaceholder []string
		var values []interface{}
		var childRecords []MutationRecord
		insertDoc, err := doc.(map[string]interface{})
		if !err {
			return nil, errors.New(fmt.Sprintf("error while parsing document at index ", i))
		}
		ii := 1
		mr[i] = MutationRecord{}
		for k, kv := range insertDoc {
			childTableName := strings.Replace(k, "___", ".", -1)
			a1, b1 := kv.([]interface{})          //check if value is array of nested object
			a2, b2 := kv.(map[string]interface{}) // check if value is single nested object (not an array)
			if b2 {
				a1 = append(a1, a2) // making single nested object into array of one element - to have common code to execute both conditions
				b1 = true
			}
			if b1 {
				tj, e := datasource.GetTableJoins(parentTableName, childTableName)
				//TODO ensure parent query has returning fields required for child join values
				if e != nil {
					return nil, e
				}
				childRecords, e = sqr.processMutationDoc(a1, myself, datasource, childTableName)
				if e != nil {
					log.Print(e.Error())
					return nil, e
				}
				if mr[i].childRecords == nil {
					mr[i].childRecords = make(map[string][]MutationRecord)
				}
				mr[i].childRecords[childTableName] = childRecords
				if mr[i].tableJoins == nil {
					mr[i].tableJoins = make(map[string]module_model.TableJoins)
				}
				mr[i].tableJoins[childTableName] = tj
			} else {
				cols = append(cols, k)
				updateCols = append(updateCols, fmt.Sprint(k, " = ", myself.GetPreparedQueryPlaceholder(ii)))
				colsPlaceholder = append(colsPlaceholder, myself.GetPreparedQueryPlaceholder(ii))
				values = append(values, kv)
				ii++
			}
		}
		mr[i].cols = strings.Join(cols, ",")
		mr[i].updatedCols = strings.Join(updateCols, ",")
		mr[i].colsPlaceholder = strings.Join(colsPlaceholder, ",")
		mr[i].values = values
	}
	return mr, nil
}

func (sqr *SqlMaker) ProcessMutationGraphQL(sel ast.Selection, vars map[string]interface{}, myself SqlMakerI, datasource *module_model.DataSource, singleTxn bool, openTxn bool, closeTxn bool, query string, cols string) (returnResult map[string]interface{}, err error) {
	//myself.CheckMe()
	log.Print("inside ProcessMutationGraphQL")
	sqr.SingleTxn = singleTxn
	sqr.openTxn = openTxn
	sqr.closeTxn = closeTxn
	field := sel.(*ast.Field)
	docsFound := false
	errMsg := "-"
	tempStr := strings.SplitN(field.Name.Value, "_", 2)
	sqr.QueryType = tempStr[0]
	sqr.MainTableName = strings.Replace(tempStr[1], "___", ".", -1)
	sqr.TxnFlag = true //default to true

	// resetting below variables as it is called in loop and
	// in case of SingleTxn, same sqr object is used for all records in loop
	sqr.MutationSelectQuery = ""
	sqr.MutationSelectCols = ""
	sqr.MutationRecords = nil
	for _, ff := range field.Arguments {
		switch ff.Name.Value {

		case "docs":
			docsFound = true
			varValue, e := ParseAstValue(ff.Value, vars)
			if e != nil {
				return nil, e
			}
			sqr.MutationRecords, err = sqr.processMutationDoc(varValue, myself, datasource, sqr.MainTableName)
			if err != nil {
				errMsg = err.Error()
				// return nil, err
				// no need to return here
			}
		case "query":
			//varValue, err := ParseAstValue(ff.Value, vars)
			//if err != nil {
			//	return nil, err
			//}

			sqr.MutationSelectQuery = query
			sqr.MutationSelectCols = cols
			docsFound = true
		case "txn":
			if !sqr.SingleTxn { //ignore txn flag for each query if singleTxn directive exists
				varValue, err := ParseAstValue(ff.Value, vars)
				if err != nil {
					return nil, err
				}
				v, e := varValue.(bool)
				if !e {
					return nil, errors.New("error while parsing value of 'txn'")
				}
				sqr.TxnFlag = v
			}
		case "where":
			v, e := ParseAstValue(ff.Value, vars)
			_ = v
			if e != nil {
				// TODO: return returnresult error
				return nil, e
			}
			///////////////////////////////////// TODO to open up wc, _ := sqr.processWhereClause(v, "", false)
			///////////////////////////////////// TODO sqr.WhereClause = fmt.Sprint(" where ", wc)
		default:
			//do nothing
		}
	}
	log.Print(sqr.MutationSelectQuery)
	if sqr.QueryType == "delete" {
		sqr.MutationRecords = make([]MutationRecord, 1) // dummy record added so that it enters for loop in ExecuteMutationQuery function
	} else if !docsFound {
		return nil, errors.New("missing 'docs' keyword - document to mutate not found") //TODO this error is not returned in graphql error
	}

	sqr.MutationReturn.ReturnError = false  //default to false
	sqr.MutationReturn.ReturnDoc = false    //default to false
	sqr.MutationReturn.ReturnFields = " * " //set default to * - below code will override it if user has asked for specific fields only
	for _, ss := range field.SelectionSet.Selections {
		field := ss.(*ast.Field)
		switch field.Name.Value {
		case "error":
			sqr.MutationReturn.ReturnError = true
			if field.Alias != nil {
				sqr.MutationReturn.ReturnErrorAlias = field.Alias.Value
			} else {
				sqr.MutationReturn.ReturnErrorAlias = field.Name.Value
			}
		case "returning":
			sqr.MutationReturn.ReturnDoc = true
			if field.Alias != nil {
				sqr.MutationReturn.ReturnDocAlias = field.Alias.Value
			} else {
				sqr.MutationReturn.ReturnDocAlias = field.Name.Value
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
				sqr.MutationReturn.ReturnFields = strings.Join(retFieldArray, ",")
			}
			//else {
			//	sqr.MutationReturn.ReturnFields = " * "
			//}
		default:
			//do nothing
		}
	}
	res, er := sqr.ExecuteMutationQuery(datasource, myself)

	if er != nil {
		log.Print(er.Error())
		errMsg = er.Error()
		//return nil, er
		// no need to return here - error is returned as part of result - if asked in the query.
	}
	returnResult = make(map[string]interface{})
	if sqr.MutationReturn.ReturnError {
		returnResult[sqr.MutationReturn.ReturnErrorAlias] = errMsg
	}
	if sqr.MutationReturn.ReturnDoc {
		returnResult[sqr.MutationReturn.ReturnDocAlias] = res
	}
	//TODO : query alias to be returned for mutation
	return returnResult, er
}
*/
/*
func (sqr *SqlMaker) ProcessGraphQL(sel ast.Selection, vars map[string]interface{}, myself SqlMakerI, datasource *module_model.DataSource, executeFlag bool) (res map[string]interface{}, query string, cols string, err error) {
	//myself.CheckMe()
	field := sel.(*ast.Field)

	//log.Print("field.GetKind() = " + field.GetKind())
	//log.Print("field.Name.Value = " + field.Name.Value)
	sqr.MainTableName = strings.Replace(field.Name.Value, "___", ".", -1)
	if field.Alias != nil {
		sqr.MainAliasName = field.Alias.Value
	} else {
		sqr.MainAliasName = sqr.MainTableName
	}
	//sqr.AllTableNames = append(sqr.AllTableNames, field.Name.Value)
	//sqr.AllTableNamesNew = append(sqr.AllTableNamesNew,[]tablesInQuery{})
	//sqr.graphqlResult.gqlr = make(map[string]interface{})
	sqr.MainTableDB = field.Directives[0].Name.Value
	______________________________________________________________________
	we will need below block for tenant ds alias
	   log.Print("field.Directives[0].Name.Value = " + field.Directives[0].Name.Value)
	   	log.Print("loop on field.Directives[0].Arguments starts")
	   	for _, vv := range field.Directives[0].Arguments {
	   		log.Print("vv.Name.Value = "+vv.Name.Value)
	   		log.Print("vv.Value.GetValue().(string)" + vv.Value.GetValue().(string))
	   	}
	   	log.Print("loop on field.Directives[0].Arguments ends")
	______________________________________________________________________

	//log.Print("len(field.Arguments) = " + string(len(field.Arguments)))
	for _, ff := range field.Arguments { //TODO to add join to main table without having to add
		switch ff.Name.Value {
		case "where":
			v, _ := ParseAstValue(ff.Value, vars)
			wc, _ := sqr.processWhereClause(v, "", false)
			//log.Print("final where clause = " + wc)
			sqr.WhereClause = fmt.Sprint(" where ", wc)
		case "sort":
			sqr.processSortClause(ff.Value, vars)
		case "distinct":
			if ff.Value.GetKind() != kinds.BooleanValue {
				return nil, "", "", errors.New("Non Boolean value received - distinct clause need integer value")
			}
			v, _ := ParseAstValue(ff.Value, vars)
			sqr.DistinctResults = v.(bool)
		case "limit": //TODO to handle if variable not found
			v, _ := ParseAstValue(ff.Value, vars)
			if reflect.TypeOf(v).Kind() == reflect.Float64 {
				v = int(v.(float64))
			}
			if reflect.TypeOf(v).Kind() != reflect.Int {
				return nil, "", "" , errors.New("Non Integer value received - limit clause need integer value")
			}
			sqr.Limit = v.(int)
		case "skip":
			if ff.Value.GetKind() != kinds.IntValue {
				return nil, "", "", errors.New("Non Integer value received - skip clause need integer value")
			}
			v, _ := ParseAstValue(ff.Value, vars)
			sqr.Skip = v.(int)
		default:
		}
	}
	//log.Print("loop on field.Arguments ends")
	//log.Print("len(field.SelectionSet.Selections) = "+string(len(field.SelectionSet.Selections)))
	if field.SelectionSet == nil {
		var tmpSelSet []ast.Selection
		sqr.ColumnList, sqr.DBColumns, sqr.GroupList, _ = sqr.processColumnList(tmpSelSet, sqr.MainTableName, vars, 0, 0, datasource)
		sqr.ColumnList = " * "
	} else {
		sqr.ColumnList, sqr.DBColumns, sqr.GroupList, _ = sqr.processColumnList(field.SelectionSet.Selections, sqr.MainTableName, vars, 0, 0, datasource)
	}
	query, errStr := sqr.MakeQuery()
	if errStr != "" {
		return nil, "", "", errors.New(errStr)
	}
	query = myself.AddLimitSkipClause(query, sqr.Limit, sqr.Skip, 1000)
	if executeFlag {
		res, err = myself.ExecuteQuery(query, datasource)
	}
	return res, query, sqr.DBColumns, err
}
*/

func (sqr *SqlMaker) ExecuteQueryForCsv(query string, datasource *module_model.DataSource, aliasName string) (res map[string]interface{}, err error) {
	log.Print("inside ExecuteQueryForCsv")
	//ctx, cancel := context.WithTimeout(context.Background(), 100000*time.Millisecond) //TODO: to get context as argument
	//defer cancel()
	rows, e := datasource.Con.Queryx(query)
	if e != nil {
		return nil, e
	}
	defer rows.Close()

	mapping := make(map[string]interface{})
	colsType, ee := rows.ColumnTypes()
	if ee != nil {
		return nil, ee
	}
	sqr.result = make(map[string]interface{})
	var innerResult [][]interface{}
	firstRow := true
	for rows.Next() {
		var innerResultRow []interface{}
		var innerResultLabel []interface{}
		ee = rows.MapScan(mapping)
		if ee != nil {
			return nil, ee
		}
		for _, colType := range colsType {
			if firstRow {
				colHeader := colType.Name()
				colHeaderArray := strings.Split(colType.Name(), "**")
				if len(colHeaderArray) > 1 {
					colHeader = colHeaderArray[1]
				}
				innerResultLabel = append(innerResultLabel, colHeader)
			}
			if mapping[colType.Name()] != nil {
				//log.Print(colType.Name())
				//log.Print(reflect.TypeOf(mapping[colType.Name()]).String(), " - ", colType.DatabaseTypeName())
				if reflect.TypeOf(mapping[colType.Name()]).String() == "[]uint8" && colType.DatabaseTypeName() == "NUMERIC" {
					f := 0.0
					f, err = strconv.ParseFloat(string(mapping[colType.Name()].([]byte)), 64)
					mapping[colType.Name()] = strconv.FormatFloat(f, 'f', -1, 64)
					if err != nil {
						log.Print(err)
						return nil, err
					}
				} else if reflect.TypeOf(mapping[colType.Name()]).String() == "float64" {
					f := 0.0
					f = mapping[colType.Name()].(float64)
					mapping[colType.Name()] = strconv.FormatFloat(f, 'E', -1, 64)
				} else if reflect.TypeOf(mapping[colType.Name()]).String() == "bool" {
					mapping[colType.Name()] = strconv.FormatBool(mapping[colType.Name()].(bool))
				} else if reflect.TypeOf(mapping[colType.Name()]).String() == "time.Time" {
					if colType.DatabaseTypeName() == "DATE" {
						mapping[colType.Name()] = mapping[colType.Name()].(time.Time).Format("02-Jan-2006")
					} else {
						mapping[colType.Name()] = mapping[colType.Name()].(time.Time).String()
					}
				} else if strings.HasPrefix(reflect.TypeOf(mapping[colType.Name()]).String(), "int") {
					//if reflect.TypeOf(mapping[colType.Name()]).String() == "int64" {
					n := mapping[colType.Name()].(int64)
					mapping[colType.Name()] = fmt.Sprintf("%d", n)
					//}
				} else if (colType.DatabaseTypeName() == "JSONB" || colType.DatabaseTypeName() == "JSON") || colType.DatabaseTypeName() == "BPCHAR" {
					mapping[colType.Name()] = string(mapping[colType.Name()].([]byte))
				}
			}

			if val, ok := mapping[colType.Name()].(string); ok {
				innerResultRow = append(innerResultRow, val)
			} else if mapping[colType.Name()] == nil {
				innerResultRow = append(innerResultRow, "")
			} else {
				err = errors.New(fmt.Sprint("value of ", colType.Name(), " is not a string"))
				return nil, err
			}
		}
		if firstRow {
			innerResult = append(innerResult, innerResultLabel)
			firstRow = false
		}
		innerResult = append(innerResult, innerResultRow)
	}
	if len(innerResult) == 0 {
		innerResult = append(innerResult, []interface{}{})
	}
	log.Print("aliasName = ", aliasName)
	sqr.result[aliasName] = innerResult
	return sqr.result, nil
}

func (sqr *SqlMaker) ExecutePreparedQuery(query string, datasource *module_model.DataSource) (res map[string]interface{}, err error) {
	log.Print("inside ExecutePreparedQuery")
	log.Print(query)
	//ctx, cancel := context.WithTimeout(context.Background(), 100000*time.Millisecond) //TODO: to get context as argument
	//defer cancel()
	rows, e := datasource.Con.Queryx(query)
	if e != nil {
		return nil, e
	}
	defer rows.Close()

	mapping := make(map[string]interface{})
	colsType, ee := rows.ColumnTypes()
	if ee != nil {
		return nil, ee
	}
	sqr.result = make(map[string]interface{})
	var innerResult []map[string]interface{}
	for rows.Next() {
		innerResultRow := make(map[string]interface{})
		ee = rows.MapScan(mapping)
		if ee != nil {
			return nil, ee
		}
		for _, colType := range colsType {
			//log.Println(colType.DatabaseTypeName())
			if colType.DatabaseTypeName() == "NUMERIC" && mapping[colType.Name()] != nil {
				//log.Print("reflect.TypeOf(mapping[colType.Name()]) ===", reflect.TypeOf(mapping[colType.Name()]))
				f := 0.0
				if reflect.TypeOf(mapping[colType.Name()]).String() == "[]uint8" {
					f, err = strconv.ParseFloat(string(mapping[colType.Name()].([]byte)), 64)
					mapping[colType.Name()] = f
				} else if reflect.TypeOf(mapping[colType.Name()]).String() == "float64" {
					f = mapping[colType.Name()].(float64)
					mapping[colType.Name()] = f
				}
				if err != nil {
					log.Print(err)
					return nil, err
				}
			} else if (colType.DatabaseTypeName() == "JSONB" || colType.DatabaseTypeName() == "JSON") && mapping[colType.Name()] != nil {
				bytesToUnmarshal := mapping[colType.Name()].([]byte)
				var v interface{}
				err = json.Unmarshal(bytesToUnmarshal, &v)
				if err != nil {
					return nil, err
				}
				mapping[colType.Name()] = &v
			}
			innerResultRow[colType.Name()] = mapping[colType.Name()]
			//innerResult[colType.Name()+"_DT"] = reflect.TypeOf(r).Kind().String()
		}
		innerResult = append(innerResult, innerResultRow)
	}
	if len(innerResult) == 0 {
		innerResult = append(innerResult, make(map[string]interface{}))
	}
	sqr.result["Results"] = innerResult

	return sqr.result, nil

}

func (sqr *SqlMaker) ExecuteMutationQuery(datasource *module_model.DataSource, myself SqlMakerI, mrm module_model.MutationResultMaker) (res []map[string]interface{}, err error) {
	//TODO single column with same values without distinct flag returns only one row - behaves like distinct
	log.Print(fmt.Sprint("ExecuteMutationQuery of SqlMaker called for ", mrm.QueryType))

	sqr.MainTableName = mrm.MainTableName
	sqr.MainAliasName = mrm.MainAliasName
	sqr.IsNested = mrm.IsNested
	sqr.MutationRecords = mrm.MutationRecords
	sqr.MutationReturn = mrm.MutationReturn
	sqr.SingleTxn = mrm.SingleTxn
	sqr.openTxn = mrm.OpenTxn
	sqr.closeTxn = mrm.CloseTxn
	sqr.TxnFlag = mrm.TxnFlag
	sqr.QueryType = mrm.QueryType
	sqr.DBQuery = mrm.DBQuery
	sqr.PreparedQuery = mrm.PreparedQuery
	//log.Println("sqr.MutationRecords printed below after conversion from mrm")
	//log.Println(sqr.MutationRecords)
	var errMsgs []string
	ctx, cancel := context.WithTimeout(context.Background(), 100000*time.Millisecond) //TODO: to get context as argument
	defer cancel()
	log.Print("Check if txn is open == ", sqr.tx)
	log.Print("sqr.openTxn = ", sqr.openTxn)
	log.Print("sqr.TxnFlag = ", sqr.TxnFlag)
	log.Print("sqr.SingleTxn = ", sqr.SingleTxn)
	if sqr.openTxn || (sqr.TxnFlag && !sqr.SingleTxn) {
		log.Print("****************************** datasource.Con.MustBegin() called in ExecuteMutationQuery ****************************** ")
		sqr.tx = datasource.Con.MustBegin() //begin txn only once for all queries OR begin txn outside for loop to insert all docs as single txn
	}
	log.Print("Check if txn is open == ", sqr.tx)
	log.Print("len(sqr.MutationRecords) = ", len(sqr.MutationRecords))
	if len(sqr.MutationRecords) > 0 || sqr.QueryType == "insertselect" || sqr.QueryType == "delete" || sqr.PreparedQuery {
		res, err = sqr.iterateDocsForMutation(ctx, sqr.MutationRecords, sqr.MainTableName, datasource, myself, false, -1)
		if err != nil {
			errMsgs = append(errMsgs, err.Error())
		}
	}
	if sqr.closeTxn || (sqr.TxnFlag && !sqr.SingleTxn) {
		log.Print(" ***************************** sqr.tx.Commit() called in ExecuteMutationQuery *******************")
		err = sqr.tx.Commit()
		if err != nil {
			errMsgs = append(errMsgs, fmt.Sprint("DB error :", err.Error()))
			sqr.tx.Rollback()
		}
	}
	if len(errMsgs) > 0 {
		if sqr.TxnFlag {
			res = make([]map[string]interface{}, 0)
		}
		return res, errors.New(strings.Join(errMsgs, " , "))
	}
	return res, nil
}

func (sqr *SqlMaker) RollbackQuery() (err error) {
	log.Print("RollbackQuery called")
	if sqr.tx != nil {
		err = sqr.tx.Rollback()
		if err != nil {
			log.Print("RollbackQuery failed = ", err.Error())
		}
	}
	return err
}

func (sqr *SqlMaker) iterateDocsForMutation(ctx context.Context, docs []module_model.MutationRecord, tableName string, datasource *module_model.DataSource, myself SqlMakerI, isNested bool, docNo int) (res []map[string]interface{}, err error) {
	log.Print("inside iterateDocsForMutation")
	var errMsgs []string
	var finalValues []interface{}
	query := ""
	if !sqr.IsNested {
		log.Print("inside if of !sqr.IsNested ")
		log.Print("sqr.DBQuery = ", sqr.DBQuery)
		log.Print("sqr.QueryType = ", sqr.QueryType)
		log.Print("sqr.PreparedQuery = ", sqr.PreparedQuery)
		if sqr.QueryType == "insertselect" || sqr.QueryType == "delete" || sqr.PreparedQuery {
			query = sqr.DBQuery
		} else if sqr.QueryType == "update" {
			finalValues = docs[0].Values
			query = sqr.MutationRecords[0].DBQuery
			for i, _ := range sqr.MutationRecords[0].UpdatedCols {
				query = strings.Replace(query, fmt.Sprint("$UpdateColPlaceholder", i), myself.GetPreparedQueryPlaceholder(1, i, true), 1)
			}
		} else {
			//TODO to handle if sqr.MutationRecords is nil - one of the reason it is passed as nil is when table or table join is not found
			//log.Println("docs printed below from ds line 573")
			//log.Println(docs)
			for _, d := range docs {
				finalValues = append(finalValues, d.NonNestedValues...)
			}
			query = strings.Replace(sqr.MutationRecords[0].DBQuery, "$ColsPlaceholder", myself.GetPreparedQueryPlaceholder(len(sqr.MutationRecords), len(sqr.MutationRecords[0].Values), false), 1)
		}
		//log.Println("finalValues = ", finalValues)
		//log.Print(query)
		res, err = sqr.executeMutationQueriesinDB(ctx, query, tableName, datasource, myself, isNested, docNo, 0, finalValues)
		if err != nil {
			errMsgs = append(errMsgs, err.Error())
			return res, errors.New(strings.Join(errMsgs, " , "))
		}
	} else {
		log.Print("inside else of !sqr.IsNested ")
		for i, v := range docs {
			query = strings.Replace(v.DBQuery, "$ColsPlaceholder", myself.GetPreparedQueryPlaceholder(1, len(v.Values), false), 1)
			//log.Print(query)
			//log.Println("v.Values = ", v.Values)
			resDocs, err := sqr.executeMutationQueriesinDB(ctx, query, tableName, datasource, myself, isNested, docNo, i, v.Values)
			if err != nil {
				errMsgs = append(errMsgs, err.Error())
				return res, errors.New(strings.Join(errMsgs, " , "))
			}
			if sqr.QueryType == "insert" && len(resDocs) > 0 {
				resDoc := resDocs[0]
				var childError bool
				for ck, cv := range v.ChildRecords {
					childError = false
					tj := v.TableJoins[ck]
					log.Print("tj ===== ", tj)
					var colsPlaceholder []int
					//var values []interface{}
					//for ii := 0; ii < len(tj.Table1Cols); ii++ {
					//	cols = append(cols, tj.Table1Cols[ii])
					//	colsPlaceholder = append(colsPlaceholder, 999+ii)
					//	values = append(values, resDoc[tj.Table1Cols[ii]])
					//}
					for icv, childRec := range cv {
						cvColsArray := strings.Split(childRec.Cols, ",")
						//log.Print(cvColsArray)
						//log.Print(tj.Table1Cols)
						for _, tjColVal := range tj.Table1Cols { //TODO test parent-child insertion extensively.
							colFound := false
							for colIdx, colVal := range cvColsArray {
								if tjColVal == colVal {
									colFound = true
									//colsPlaceholder = append(colsPlaceholder[:idx], colsPlaceholder[idx+1:]...)
									//log.Print(resDoc[tjColVal])
									//log.Print(childRec.Values)
									//log.Print(colIdx)
									childRec.Values[colIdx] = resDoc[tjColVal]
									break
								}
							}
							if !colFound {
								log.Print("inside !colFound")
								cvColsArray = append(cvColsArray, tjColVal)
								//childRec.Values = append(childRec.Values, resDoc[tjColVal])
								cv[icv].Values = append(cv[icv].Values, resDoc[tjColVal])
								colsPlaceholder = append(colsPlaceholder, len(cvColsArray))
							}
						}
						cv[icv].Cols = strings.Join(cvColsArray, ",")
						/* to check if below block is needed
						var colsPlaceholderStr []string

						for iiii := 0; iiii < len(colsPlaceholder); iiii++ {
							//colsPlaceholderStr = append(colsPlaceholderStr, myself.GetPreparedQueryPlaceholder(colsPlaceholder[iiii]-999+len(strings.Split(cv[iii].colsPlaceholder, ","))+1))
							colsPlaceholderStr = append(colsPlaceholderStr, myself.GetPreparedQueryPlaceholder(colsPlaceholder[iiii]))
						}
						childRec.colsPlaceholder = fmt.Sprint(childRec.colsPlaceholder, " , ", strings.Join(colsPlaceholderStr, ","))
						//cv[iii].values = append(cv[iii].values, values...)
						*/
					}
					//log.Print(cv)
					resDoc[ck], err = sqr.iterateDocsForMutation(ctx, cv, ck, datasource, myself, true, docNo)
					if err != nil {
						childError = true
						errMsgs = append(errMsgs, err.Error())
						return res, errors.New(strings.Join(errMsgs, " , "))
					}
				}
				if !childError {
					res = append(res, resDoc) //remember parent result only if there are no errors in child/nested records
				} else {
					res = append(res, make(map[string]interface{}))
				}
			} else {
				res = resDocs
			}
		}
	}
	if len(errMsgs) > 0 {
		return res, errors.New(strings.Join(errMsgs, " , "))
	}
	return res, nil
}

func (sqr *SqlMaker) executeMutationQueriesinDB(ctx context.Context, query string, tableName string, datasource *module_model.DataSource, myself SqlMakerI, isNested bool, docNo int, idx int, vals []interface{}) (res []map[string]interface{}, err error) {
	log.Print("inside executeMutationQueriesinDB")
	var errMsgs []string
	if !isNested {
		docNo = idx + 1 //to ensure error message always refer to parent document number
	}
	errFound := false
	if !sqr.TxnFlag && !isNested {
		log.Print("****************************** datasource.Con.MustBegin() called ****************************** ")
		sqr.tx = datasource.Con.MustBegin() // begin txn inside for loop to insert every doc as seperate txn
	}
	log.Println(query)
	stmt, err := sqr.tx.PreparexContext(ctx, query) // TODO: to fetch con after locking
	if err != nil {
		log.Print(err)
		errFound = true
		errMsgs = append(errMsgs, fmt.Sprint("DB error for Document No ", docNo, " : ", err.Error()))
		sqr.tx.Rollback()
		if sqr.TxnFlag || isNested {
			res = make([]map[string]interface{}, 0)
			return nil, errors.New(strings.Join(errMsgs, " , "))
		}
		//return nil, errors.New(strings.Join(errMsgs, " , "))
	}
	if !errFound {
		/*if sqr.QueryType == "insert" {
			resDoc := make(map[string]interface{})
			rw := stmt.QueryRowxContext(ctx, vals...).MapScan(resDoc)

			//rw, ee := stmt.QueryxContext(ctx, v.values...)
			if rw != nil {
				if rw.Error() != "" {
					errMsgs = append(errMsgs, fmt.Sprint("DB error for Document No ", docNo, " : ", rw.Error()))
					sqr.tx.Rollback()
					if sqr.TxnFlag || isNested {
						res = make([]map[string]interface{}, 0)
						return nil, errors.New(strings.Join(errMsgs, " , "))
					}
				}
			}

			for k, v := range resDoc {
				if v != nil {
					if reflect.TypeOf(v).String() == "[]uint8" {
						log.Print("string(v.([]byte) == ", string(v.([]byte)))
						f, err := strconv.ParseFloat(string(v.([]byte)), 64)
						if err != nil {
							log.Print(err.Error(), " : ", k, " : ", v)
							resDoc[k] = string(v.([]byte))
						} else {
							resDoc[k] = f
						}
					}
				}
			}
			res = append(res, resDoc)
		} else {

		*/
		//log.Println(vals)
		rw, ee := stmt.QueryxContext(ctx, vals...)
		if ee != nil {
			errMsgs = append(errMsgs, fmt.Sprint("DB error for Document No ", docNo, " : ", ee.Error()))
			log.Print(errMsgs)
			sqr.tx.Rollback()
			//if sqr.TxnFlag || isNested {
			return nil, errors.New(strings.Join(errMsgs, " , "))
			//}
		}
		for rw.Rows.Next() {
			resDoc := make(map[string]interface{})
			colsType, ee := rw.ColumnTypes()
			if ee != nil {
				return nil, ee
			}
			e := rw.MapScan(resDoc)
			if e != nil {
				return nil, e
			}

			for _, colType := range colsType {
				//log.Print("resDoc[colType.Name()] == ", resDoc[colType.Name()])
				//log.Println("colType.DatabaseTypeName() == ", colType.DatabaseTypeName())
				if colType.DatabaseTypeName() == "JSONB" || colType.DatabaseTypeName() == "JSON" {
					var tmpv interface{}
					json.Unmarshal(resDoc[colType.Name()].([]byte), &tmpv)
					//log.Println(tmpv)
					resDoc[colType.Name()] = tmpv
				} else if colType.DatabaseTypeName() == "NUMERIC" && resDoc[colType.Name()] != nil {
					f, err := strconv.ParseFloat(string(resDoc[colType.Name()].([]byte)), 64)
					if err != nil {
						log.Print(err)
						return nil, err
					}
					resDoc[colType.Name()] = f
				}
			}
			res = append(res, resDoc)
		}
		//log.Println(res)
		//}
	}
	log.Print("!sqr.TxnFlag && !isNested", sqr.TxnFlag, " ", !isNested)
	if !sqr.TxnFlag && !isNested {
		log.Print(" ***************************** sqr.tx.Commit() called *******************")
		err = sqr.tx.Commit()
		if err != nil {
			errMsgs = append(errMsgs, fmt.Sprint("DB error for Document No ", docNo, " : ", err.Error()))
			sqr.tx.Rollback()
		}
	}
	//log.Print(res)
	log.Print("Exiting executeMutationQueriesinDB")
	return res, nil
}

func (sqr *SqlMaker) ExecuteQuery(datasource *module_model.DataSource, qrm module_model.QueryResultMaker) (res map[string]interface{}, err error) {
	log.Print("executeQuery of SqlMaker called")

	sqr.queryLevel = qrm.QueryLevel
	sqr.querySubLevel = qrm.QuerySubLevel
	sqr.tables = qrm.Tables
	sqr.MainAliasName = qrm.MainAliasName
	sqr.MainTableName = qrm.MainTableName

	//rows, e := datasource.Con.Query(query)
	//log.Print(datasource.ConStatus)
	//log.Print(qrm.SQLQuery)
	rows, e := datasource.Con.Queryx(qrm.SQLQuery)
	if e != nil {
		return nil, e
	}
	//log.Print(rows.Columns())
	//cols,ee := rows.Columns()
	//log.Print(ee)
	defer rows.Close()
	mapping := make(map[string]interface{})
	colsType, ee := rows.ColumnTypes()
	if ee != nil {
		return nil, ee
	}
	rowNo := 0
	sqr.result = make(map[string]interface{})
	//sqr.result = map[string]interface{}{sqr.MainTableName:[]interface{}{}}
	for rows.Next() {
		rowNo = rowNo + 1
		//log.Print(rowNo)
		//resultRowHolder := make([]map[string]interface{}, sqr.queryLevel)
		resultRowHolderNew := make([][]map[string]interface{}, sqr.queryLevel+1)
		//sqr.tempA = make(map[int]int)
		e = rows.MapScan(mapping)
		if e != nil {
			return nil, e
		}

		var colLevelI, colSubLevelI int
		var er error
		for _, colType := range colsType {
			//log.Print("raw colType = ", colType)
			colLevelStrArray := strings.Split(colType.Name(), "**")
			colLevelStr := "0~~0"
			cn := colType.Name()
			if len(colLevelStrArray) > 1 {
				colLevelStr = strings.Replace(colLevelStrArray[0], "L", "", 1)
				cn = strings.Split(colType.Name(), "**")[1]
			}
			//log.Print("colLevelStr = ", colLevelStr)
			colLevel := strings.Split(colLevelStr, "~~")[0]
			colSubLevel := ""
			//log.Print("colType.Name() = ", colType.Name())
			//log.Print("cn ===", cn)
			//commneted on 7oct
			//if len(strings.Split(cn, ".")) > 1 {
			//	cn = strings.Split(cn, ".")[1]
			//}

			if len(strings.Split(colLevelStr, "~~")) > 1 {
				colSubLevel = strings.Split(colLevelStr, "~~")[1]
				colSubLevelI, er = strconv.Atoi(colSubLevel)
			}
			colLevelI, er = strconv.Atoi(colLevel) //TODO to handle select * (no column names)
			if er != nil {
				return nil, er
			}

			//colLevelI = colLevelI - 1
			//colSubLevelI = colSubLevelI - 1

			//if resultRowHolder[colLevelI] == nil {
			//	resultRowHolder[colLevelI] = make(map[string]interface{})
			//}
			if resultRowHolderNew[colLevelI] == nil {
				resultRowHolderNew[colLevelI] = []map[string]interface{}{0: make(map[string]interface{})}
			} else if len(resultRowHolderNew[colLevelI])-1 < colSubLevelI {
				resultRowHolderNew[colLevelI] = append(resultRowHolderNew[colLevelI], make(map[string]interface{}))
			}

			//log.Print("colType.DatabaseTypeName() ===", colType.DatabaseTypeName())
			//log.Print("mapping[colType.Name()] == ",mapping[colType.Name()])
			actualColType := ""
			if mapping[colType.Name()] != nil {
				actualColType = reflect.TypeOf(mapping[colType.Name()]).String()
			}
			_ = actualColType
			//log.Println(actualColType)
			//log.Println(colType.Name())
			//log.Println(mapping[colType.Name()])

			//log.Println(colType.DatabaseTypeName())
			if colType.DatabaseTypeName() == "NUMERIC" && mapping[colType.Name()] != nil {
				//log.Print("reflect.TypeOf(mapping[colType.Name()]) ===", reflect.TypeOf(mapping[colType.Name()]))
				f := 0.0
				if reflect.TypeOf(mapping[colType.Name()]).String() == "[]uint8" {
					f, err = strconv.ParseFloat(string(mapping[colType.Name()].([]byte)), 64)
					mapping[colType.Name()] = f
				} else if reflect.TypeOf(mapping[colType.Name()]).String() == "float64" {
					f = mapping[colType.Name()].(float64)
					mapping[colType.Name()] = f
				}
				if err != nil {
					log.Print(err)
					return nil, err
				}
			} else if (colType.DatabaseTypeName() == "JSONB" || colType.DatabaseTypeName() == "JSON") && mapping[colType.Name()] != nil {
				bytesToUnmarshal := mapping[colType.Name()].([]byte)
				var v interface{}
				err = json.Unmarshal(bytesToUnmarshal, &v)
				if err != nil {
					return nil, err
				}
				mapping[colType.Name()] = &v
			}
			resultRowHolderNew[colLevelI][colSubLevelI][cn] = mapping[colType.Name()]

			/*
				if actualColType == "[]uint8" && mapping[colType.Name()] != nil {
					f, err := strconv.ParseFloat(string(mapping[colType.Name()].([]byte)), 64)
					if err != nil {
						log.Print("error from strconv.ParseFloat")
						log.Print(err)
						return nil, err
					}
					mapping[colType.Name()] = f
				}
				resultRowHolderNew[colLevelI][colSubLevelI][cn] = mapping[colType.Name()]
			*/

			//log.Print(resultRowHolderNew)
		}

		r, rf, err := sqr.processRows(resultRowHolderNew, 0, rowNo, true, -1) //sqr.result[sqr.MainTableName].([]interface{})
		if err != nil {
			log.Print("error from processRows = ", err.Error())
			return nil, er
		}
		mtn := strings.Replace(sqr.MainTableName, ".", "___", 1)
		man := strings.Replace(sqr.MainAliasName, ".", "___", 1)
		//log.Print(rf)
		//log.Print(mtn)
		//log.Print(man)
		if !rf[mtn] {
			if sqr.result[man] == nil {
				sqr.result[man] = r[mtn]
			} else {
				sqr.result[man] = append(sqr.result[man].([]interface{}), r[mtn].([]interface{})...)
			}
		}

	}
	log.Print("rows returned = ", rowNo)
	//log.Print("888888888")
	//ff, _ := json.Marshal(sqr.result)
	//log.Print(string(ff))
	//log.Print("sqr.result is printed below")
	//log.Print(sqr.result)
	return sqr.result, nil
}

func (sqr *SqlMaker) processRows(vrh [][]map[string]interface{}, curLevel int, rowNo int, parentRecordFound bool, parentIndexNo int) (r map[string]interface{}, recf map[string]bool, err error) {
	//log.Print("curLevel = ", curLevel)
	recordFound := false
	recf = make(map[string]bool)
	//r = append(r,[]interface{}{})
	r = make(map[string]interface{})

	for i := 0; i <= sqr.querySubLevel[curLevel]; i++ {
		//log.Print("curLevel = ", curLevel, " curSubLevel = ", i)
		curLevelValue := make(map[string]interface{})
		vrh[curLevel][i]["parentIndexNo"] = parentIndexNo
		v, e := json.Marshal(vrh[curLevel][i])
		if e != nil {
			return nil, nil, e
		}

		delete(vrh[curLevel][i], "parentIndexNo")
		var curLevelRecord map[string]interface{}
		indexNo := 1

		if len(sqr.resultHolderNew) <= curLevel {
			sqr.resultHolderNew = append(sqr.resultHolderNew, []map[string]interface{}{0: make(map[string]interface{})})
			sqr.resultIndexHolderNew = append(sqr.resultIndexHolderNew, []map[int]int{map[int]int{parentIndexNo: 0}})
		}
		if len(sqr.resultHolderNew[curLevel]) <= i {
			sqr.resultHolderNew[curLevel] = append(sqr.resultHolderNew[curLevel], make(map[string]interface{}))
			sqr.resultIndexHolderNew[curLevel] = append(sqr.resultIndexHolderNew[curLevel], map[int]int{parentIndexNo: 0})
		}
		if sqr.resultHolderNew[curLevel][i][string(v)] != nil {
			curLevelRecord = sqr.resultHolderNew[curLevel][i][string(v)].(map[string]interface{})
			if curLevelValue[sqr.tables[curLevel][i].Name] == nil {
				curLevelValue[sqr.tables[curLevel][i].Name] = []interface{}{curLevelRecord}
			} else {
				curLevelValue[sqr.tables[curLevel][i].Name] = append(curLevelValue[sqr.tables[curLevel][i].Name].([]interface{}), curLevelRecord)
			}
			if !parentRecordFound {
				recordFound = false
				sqr.resultIndexHolderNew[curLevel][i][parentIndexNo] = 1
				indexNo = sqr.resultIndexHolderNew[curLevel][i][parentIndexNo]
			} else {
				recordFound = true
				indexNo = curLevelRecord["rowIndex"].(int)
			}
		} else {
			recordFound = false
			if !parentRecordFound {
				sqr.resultIndexHolderNew[curLevel][i][parentIndexNo] = 1
			} else {
				sqr.resultIndexHolderNew[curLevel][i][parentIndexNo] = sqr.resultIndexHolderNew[curLevel][i][parentIndexNo] + 1
			}
			sqr.resultHolderNew[curLevel][i][string(v)] = vrh[curLevel][i]
			curLevelRecord = sqr.resultHolderNew[curLevel][i][string(v)].(map[string]interface{})
			//TODO remove rowindex - atleast from result.
			curLevelRecord["rowIndex"] = sqr.resultIndexHolderNew[curLevel][i][parentIndexNo]
			if curLevelValue[sqr.tables[curLevel][i].Name] == nil {
				curLevelValue[sqr.tables[curLevel][i].Name] = []interface{}{curLevelRecord}
			} else {
				curLevelValue[sqr.tables[curLevel][i].Name] = append(curLevelValue[sqr.tables[curLevel][i].Name].([]interface{}), curLevelRecord)
			}
			indexNo = sqr.resultIndexHolderNew[curLevel][i][parentIndexNo]
		}
		recf[sqr.tables[curLevel][i].Name] = recordFound
		//log.Print(fmt.Sprint("rowNo = ", rowNo, " curLevel = ", curLevel, " recordFound = ", recordFound, " rowIndex = ", indexNo, " parentIndex = ", parentIndexNo))
		if curLevel+1 <= sqr.queryLevel && sqr.tables[curLevel][i].Nested {
			cr, rf, ee := sqr.processRows(vrh, curLevel+1, rowNo, recordFound, indexNo)
			if ee != nil {
				return nil, nil, ee
			}
			//log.Print(fmt.Sprint("rowNo = ", rowNo, " curLevel = ", curLevel, " curSubLevel = ", i))
			idx := 0
			for k, v := range cr {
				if !rf[k] {
					if curLevelRecord[k] == nil {
						curLevelRecord[k] = v
					} else {
						curLevelRecord[k] = append(curLevelRecord[k].([]interface{}), v.([]interface{})...)
					}
				}
				idx = idx + 1
			}
		}
		//log.Print("curLevelValue printed below")
		//log.Print(curLevelValue)
		r[sqr.tables[curLevel][i].Name] = curLevelValue[sqr.tables[curLevel][i].Name]
	}
	//log.Print(r)
	//log.Print(recf)
	return r, recf, nil
}

/*
func (sqr *SqlMaker) processColumnList(sel []ast.Selection, tableName string, vars map[string]interface{}, level int, sublevel int, datasource *module_model.DataSource) (columnList string, cList string, groupList string, err string) {
	//log.Print(fmt.Sprint("level = ", level, " sublevel = ", sublevel))
	//log.Print("tableName === ", tableName)

	if sqr.queryLevel < level {
		sqr.queryLevel = level
	}
	if len(sqr.querySubLevel) <= level {
		sqr.querySubLevel = append(sqr.querySubLevel, 1)
	}
	sqr.querySubLevel[level] = sublevel
	mySublevel := 0
	tempArray := make([]string, len(sel))
	var tempArrayG []string
	var tempArrayC []string
	tiq := tablesInQuery{strings.Replace(tableName, ".", "___", 1), false}

	if len(sqr.AllTableNamesNew) == level {
		sqr.AllTableNamesNew = append(sqr.AllTableNamesNew, []tablesInQuery{})
	}
	if len(sqr.AllTableNamesNew[level]) == sublevel {
		sqr.AllTableNamesNew[level] = append(sqr.AllTableNamesNew[level], tiq)
	}

	for i, va := range sel {
		joinFound := false
		colProcessed := false
		field := va.(*ast.Field)
		temp1 := strings.Split(field.Name.Value, "___")
		var temp2 []string
		var colSchemaName, colTableName, colName string
		if len(temp1) > 1 {
			colSchemaName = temp1[0]
			temp2 = strings.Split(temp1[1], "__")
			if len(temp2) > 1 {
				if colSchemaName != "" {
					colTableName = fmt.Sprint(colSchemaName, ".", temp2[0])
				} else {
					colTableName = temp2[0]
				}
				colName = temp2[1]
			} else {
				colName = temp2[0]
			}
		} else {
			temp2 = strings.Split(field.Name.Value, "__")
			if len(temp2) > 1 {
				colTableName = temp2[0]
				colName = temp2[1]
			} else {
				colName = temp2[0]
			}
		}
		if field.SelectionSet != nil {
			if colSchemaName != "" {
				colTableName = fmt.Sprint(colSchemaName, ".", colName)
			} else {
				colTableName = colName
			}
			colName = ""
		}
		var val string
		if colTableName != "" {
			val = fmt.Sprint(colTableName, ".", colName)
		} else {
			val = colName
		}
		//orgVal := fmt.Sprint("L",level,"**",field.Name.Value)
		alias := ""
		cName := ""
		if field.Alias == nil {
			alias = fmt.Sprint(" \"L", level, "~~", sublevel, "**", field.Name.Value, "\" ") // TODO to add aggregate in column name"_", d.Name.Value,
			cName = field.Name.Value
			//alias1 = fmt.Sprint(" ",alias)
		} else {
			alias = fmt.Sprint(" \"L", level, "~~", sublevel, "**", field.Alias.Value, "\" ")
			//alias1 = fmt.Sprint(" ",field.Alias.Value)
			cName = field.Alias.Value
		}
		//alias1 := ""
		tn := ""
		if len(strings.Split(tableName, ".")) > 1 {
			tn = strings.Split(tableName, ".")[1]
		} else {
			tn = tableName
		}
		if !strings.Contains(val, ".") {
			val = fmt.Sprint(tn, ".", val)
		}
		for _, a := range field.Arguments { //TODO where clause for inner tables
			switch a.Name.Value {
			case "join":
				joinFound = true
				sqr.processJoins(a.Value, nil, colTableName, vars)
			case "calc":
				v, err := ParseAstValue(a.Value, vars)
				log.Print(err)                         //TODO to exit if error
				val = fmt.Sprint("'", v.(string), "'") //TODO to handle float value as variable value
			default:
				// do nothing
			}
		}
		if field.SelectionSet != nil {
			colName = ""
			var tg string
			var tc string
			tiq.nested = true
			sqr.AllTableNamesNew[level][sublevel] = tiq
			tempArray[i], tc, tg, err = sqr.processColumnList(field.SelectionSet.Selections, colTableName, vars, level+1, mySublevel, datasource)
			colProcessed = true
			mySublevel = mySublevel + 1
			if tg != "" {
				tempArrayG = append(tempArrayG, tg)
			}
			tempArrayC= append(tempArrayC,tc)
			if err != "" {
				return "", "", "", err
			}

		} else if len(field.Directives) > 0 {
			d := field.Directives[0]

			switch d.Name.Value {
			case "sum", "count", "avg", "max", "min":
				tempArray[i] = fmt.Sprint(d.Name.Value, "(", val, ") ", alias)
				sqr.HasAggregate = true
				colProcessed = true
			case "distinctcount":
				tempArray[i] = fmt.Sprint("count(distinct ", val, ") ", alias)
				sqr.HasAggregate = true
				colProcessed = true
			default:
				// do nothing
			}
		}
		if !colProcessed {
			tempArray[i] = fmt.Sprint(val, alias)
			tempArrayG = append(tempArrayG, val)
			tempArrayC = append(tempArrayC, cName)
		}
		if !joinFound && colTableName != "" && colTableName != sqr.MainTableName {
			log.Print("fetch joins for tables ", tableName, " and ", colTableName)
			if sqr.TableNames == nil {
				sqr.TableNames = make(map[string]string)
			}
			if _, ok := sqr.TableNames[colTableName]; !ok { //TODO to check simlar check of duplicate table in joins in join clause passed explicitly in query
				sqr.TableNames[colTableName] = ""
				tj, e := datasource.GetTableJoins(tableName, colTableName)
				log.Print(e)
				if e != nil {
					return "", "","", e.Error()
				}
				sqr.processJoins(nil, tj.GetOnClause(), colTableName, vars)
				joinFound = true
			}
		}
	}
	//log.Print("loop on field.SelectionSet.Selections ends")
	return strings.Join(tempArray, " , "), strings.Join(tempArrayC, " , "), strings.Join(tempArrayG, " , "), ""
}
*/

// parseAstValue returns an interface that can be casted to string
func ParseAstValue(value ast.Value, vars map[string]interface{}) (interface{}, error) {
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
		return replaceVariableValue(t.Name.Value, vars)

	default:
		return nil, errors.New("Invalid data type `" + value.GetKind() + "` for value " + string(value.GetLoc().Source.Body)[value.GetLoc().Start:value.GetLoc().End])
	}
}

func replaceVariableValue(varName string, vars map[string]interface{}) (res interface{}, err error) {
	if vars[varName] == nil {
		return nil, errors.New(fmt.Sprint("Variable value not found for '", varName, "'"))
	}
	switch reflect.TypeOf(vars[varName]).Kind() {
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
		switch reflect.TypeOf(v).Kind() {
		case reflect.Map:
			m[k], err = processMapVariable(v.(map[string]interface{}), vars)
		case reflect.Slice:
			// do nothing - return as is
		default:
			if strings.HasPrefix(v.(string), "$") { // TODO handle non string values in DOC for update
				m[k], err = replaceVariableValue(strings.Replace(v.(string), "$", "", 1), vars)
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

func (sqr *SqlMaker) MakeQuery() (dbQuery string, err string) {
	var str string
	strDistinct := ""
	gs := ""
	if sqr.HasAggregate && sqr.GroupList != "" {
		gs = fmt.Sprint(" group by ", sqr.GroupList, " ")
	}
	if sqr.DistinctResults {
		strDistinct = " distinct "
	}
	str = fmt.Sprint("select ", strDistinct, sqr.ColumnList, " from ", sqr.MainTableName, " ", sqr.JoinClause, " ", sqr.WhereClause, " ", gs, sqr.SortClause)
	return str, ""
}

func (sqr *SqlMaker) AddLimitSkipClause(query string, limit int, skip int, globalLimit int) (newQuery string) {
	if limit > 0 {
		newQuery = fmt.Sprint(newQuery, " limit ", limit)
	} else {
		newQuery = fmt.Sprint(newQuery, " limit ", globalLimit)
	}
	if skip > 0 {
		newQuery = fmt.Sprint(newQuery, " offset ", skip)
	}
	return newQuery
}

func (sqr *SqlMaker) GetTableList(query string, datasource *module_model.DataSource, myself SqlMakerI) (err error) {
	//log.Print(query)
	//log.Print(datasource.DbConfig.DefaultSchema)
	tableList := make(map[string]map[string]module_model.TableColsMetaData)
	//log.Println(query)
	rows, e := datasource.Con.Queryx(query)
	if e != nil {
		log.Print(e)
		return e
	}
	defer rows.Close()
	for rows.Next() {
		innerResultRow := module_model.TableColsMetaData{}
		e = rows.StructScan(&innerResultRow)
		if e != nil {
			log.Print(e)
		}
		innerResultRow.OwnDataType = myself.getDataTypeMapping(innerResultRow.DataType)
		tableKey := fmt.Sprint(innerResultRow.TblSchema, ".", innerResultRow.TblName)
		if tableList[tableKey] == nil {
			tableList[tableKey] = make(map[string]module_model.TableColsMetaData)
		}
		tableList[tableKey][innerResultRow.ColName] = innerResultRow
	}
	datasource.OtherTables = tableList
	return nil

}
