package ql

import (
	"context"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"github.com/eru-tech/eru/eru-utils"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type SQLObjectQ struct {
	ProjectId       string
	FinalVariables  map[string]interface{}
	MainTableName   string
	MainAliasName   string
	MainTableDB     string
	WhereClause     interface{}
	SortClause      interface{}
	JoinClause      []*OrderedMap //map[string]interface{}
	DistinctResults bool
	HasAggregate    bool
	Limit           int
	Skip            int
	Columns         SQLCols
	tables          [][]module_model.Tables
	tableNames      map[string]string
	queryLevel      int
	querySubLevel   []int
	DBQuery         string
	OverwriteDoc    map[string]map[string]interface{} `json:"-"`
	SecurityClause  map[string]string                 `json:"-"`
	WithQuery       string                            `json:"-"`
}

type SQLCols struct {
	ColWithAlias []string
	ColNames     []string
	GroupClause  []string
}

func (sqlObj *SQLObjectQ) ProcessGraphQL(ctx context.Context, sel ast.Selection, datasource *module_model.DataSource, sqlMaker ds.SqlMakerI, vars map[string]interface{}, s module_store.ModuleStoreI, withColAlias bool) (err error) {
	logs.WithContext(ctx).Debug("ProcessGraphQL - Start")
	field := sel.(*ast.Field)
	sqlObj.MainTableName = strings.Replace(field.Name.Value, "___", ".", -1) //replacing schema___tablename with schema.tablename
	if field.Alias != nil {
		sqlObj.MainAliasName = field.Alias.Value
	} else {
		sqlObj.MainAliasName = sqlObj.MainTableName
	}
	sqlObj.MainTableDB = field.Directives[0].Name.Value

	/* we will need below block for tenant ds alias
	for _, vv := range field.Directives[0].Arguments {
	}
	*/

	for _, ff := range field.Arguments { //TODO to add join to main table without having to add

		v, e := ParseAstValue(ctx, ff.Value, vars)
		if e != nil {
			logs.WithContext(ctx).Error(e.Error())
		}
		switch ff.Name.Value {
		case "where":
			sqlObj.WhereClause = v
		case "sort":
			sqlObj.SortClause = v
		case "distinct":
			if ff.Value.GetKind() != kinds.BooleanValue {
				err = errors.New("Non Boolean value received - distinct clause need boolean value")
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			sqlObj.DistinctResults = v.(bool)
		case "limit": //TODO to handle if variable not found
			if reflect.TypeOf(v).Kind() == reflect.Float64 {
				v = int(v.(float64))
			}
			if reflect.TypeOf(v).Kind() != reflect.Int {
				err = errors.New("Non Integer value received - limit clause need integer value")
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			sqlObj.Limit = v.(int)
		case "skip":
			if reflect.TypeOf(v).Kind() == reflect.Float64 {
				v = int(v.(float64))
			}
			if reflect.TypeOf(v).Kind() != reflect.Int {
				err = errors.New("Non Integer value received - skip clause need integer value")
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			//v, e := ParseAstValue(ff.Value, vars)
			sqlObj.Skip = v.(int)
		default:
		}
	}
	sqlCols := SQLCols{}
	if field.SelectionSet == nil {
		var tmpSelSet []ast.Selection
		sqlCols, _ = sqlObj.processColumnList(ctx, tmpSelSet, sqlObj.MainTableName, vars, 0, 0, datasource, s)
		sqlCols.ColWithAlias = append(sqlCols.ColWithAlias, " * ")
	} else {
		sqlCols, _ = sqlObj.processColumnList(ctx, field.SelectionSet.Selections, sqlObj.MainTableName, vars, 0, 0, datasource, s)
	}
	sqlObj.Columns = sqlCols
	err = sqlObj.MakeQuery(ctx, sqlMaker, withColAlias)
	logs.WithContext(ctx).Info(fmt.Sprint("query  : ", sqlObj.DBQuery))
	return err
}

func (sqlObj *SQLObjectQ) processColumnList(ctx context.Context, sel []ast.Selection, tableName string, vars map[string]interface{}, level int, sublevel int, datasource *module_model.DataSource, s module_store.ModuleStoreI) (sqlCols SQLCols, err string) {
	logs.WithContext(ctx).Debug("processColumnList - Start")
	if sqlObj.queryLevel < level {
		sqlObj.queryLevel = level
	}
	if len(sqlObj.querySubLevel) <= level {
		sqlObj.querySubLevel = append(sqlObj.querySubLevel, 1)
	}
	sqlObj.querySubLevel[level] = sublevel
	mySublevel := 0
	//sqlCols.ColWithAlias = make([]string, len(sel))

	//tempArray := make([]string, len(sel))
	//var tempArrayG []string
	//var tempArrayC []string

	tiq := module_model.Tables{strings.Replace(tableName, ".", "___", 1), false, ""}

	if len(sqlObj.tables) == level {
		sqlObj.tables = append(sqlObj.tables, []module_model.Tables{})
	}
	if len(sqlObj.tables[level]) == sublevel {
		sqlObj.tables[level] = append(sqlObj.tables[level], tiq)
	}

	for _, va := range sel {
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
				v, err := ParseAstValue(ctx, a.Value, vars)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
				//TODO to exit if error
				//sqlObj.processJoins(a.Value, nil, colTableName, vars)

				//if sqlObj.JoinClause == nil {
				//	sqlObj.JoinClause = make(map[string]interface{})
				//}
				mapObj := make(map[string]interface{})
				mapObj[colTableName] = v
				om := OrderedMap{Level: level, SubLevel: sublevel, Rank: len(sqlObj.JoinClause) + 1, Obj: mapObj}
				sqlObj.JoinClause = append(sqlObj.JoinClause, &om)

			case "calc":
				v, err := ParseAstValue(ctx, a.Value, vars)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
				}
				//TODO to exit if error
				//val = fmt.Sprint("'", v.(string), "'") //TODO to handle float value as variable value
				actualType := reflect.TypeOf(v).String()
				if actualType == "string" {
					//val = fmt.Sprint("'", v.(string), "'") //TODO commented this as formulas stopped working
					val = v.(string)
				} else {
					val = v.(string) //TODO calc numeric value thorws error here
				}
			default:
				// do nothing
			}
		}
		if field.SelectionSet != nil {
			colName = ""
			cName = ""
			//var tg string
			//var tc string
			tiq.Nested = true
			sqlObj.tables[level][sublevel] = tiq
			sqlChildCols := SQLCols{}
			sqlChildCols, err = sqlObj.processColumnList(ctx, field.SelectionSet.Selections, colTableName, vars, level+1, mySublevel, datasource, s)
			sqlCols.ColNames = append(sqlCols.ColNames, sqlChildCols.ColNames...)
			sqlCols.ColWithAlias = append(sqlCols.ColWithAlias, sqlChildCols.ColWithAlias...)
			sqlCols.GroupClause = append(sqlCols.GroupClause, sqlChildCols.GroupClause...)
			colProcessed = true
			mySublevel = mySublevel + 1
			//if tg != "" {
			//tempArrayG = append(tempArrayG, tg)
			//	sqlCols.GroupClause = append(sqlCols.GroupClause, tg)
			//}
			//tempArrayC = append(tempArrayC, tc)
			//sqlCols.ColNames = append(sqlCols.ColNames, tc)
			if err != "" {
				return SQLCols{}, err
			}
		} else if len(field.Directives) > 0 {
			d := field.Directives[0] // do not support multiple directives for fields - thus picking up first one - rest if provided will be ignored
			switch d.Name.Value {
			case "sum", "count", "avg", "max", "min":
				//tempArray[i] = fmt.Sprint(d.Name.Value, "(", val, ") ", alias)
				sqlCols.ColWithAlias = append(sqlCols.ColWithAlias, fmt.Sprint(d.Name.Value, "(", val, ") ", alias))
				sqlCols.ColNames = append(sqlCols.ColNames, cName)
				sqlObj.HasAggregate = true
				colProcessed = true
			case "distinctcount":
				//tempArray[i] = fmt.Sprint("count(distinct ", val, ") ", alias)
				sqlCols.ColWithAlias = append(sqlCols.ColWithAlias, fmt.Sprint("count(distinct ", val, ") ", alias))
				sqlCols.ColNames = append(sqlCols.ColNames, cName)
				sqlObj.HasAggregate = true
				colProcessed = true
			default:
				// do nothing
			}
		}
		if !colProcessed {
			//tempArray[i] = fmt.Sprint(val, alias)
			//tempArrayG = append(tempArrayG, val)
			//tempArrayC = append(tempArrayC, cName)
			sqlCols.ColWithAlias = append(sqlCols.ColWithAlias, fmt.Sprint(val, alias))
			sqlCols.GroupClause = append(sqlCols.GroupClause, val)
			sqlCols.ColNames = append(sqlCols.ColNames, cName)
		}
		if !joinFound && colTableName != "" && colTableName != sqlObj.MainTableName {
			logs.WithContext(ctx).Info(fmt.Sprint("fetch joins for tables ", tableName, " and ", colTableName))
			if sqlObj.tableNames == nil {
				sqlObj.tableNames = make(map[string]string)
			}
			if _, ok := sqlObj.tableNames[colTableName]; !ok { //TODO to check simlar check of duplicate table in joins in join clause passed explicitly in query
				sqlObj.tableNames[colTableName] = ""
				tj, e := datasource.GetTableJoins(ctx, tableName, colTableName, sqlObj.tableNames)
				if e != nil {
					logs.WithContext(ctx).Error(e.Error())
					return SQLCols{}, e.Error()
				}
				if sqlObj.SecurityClause == nil {
					sqlObj.SecurityClause = make(map[string]string)
				}
				sqlObj.SecurityClause[colTableName], e = getTableSecurityRule(ctx, sqlObj.ProjectId, datasource.DbAlias, colTableName, s, "query", sqlObj.FinalVariables)
				if e != nil {
					logs.WithContext(ctx).Error(e.Error())
					return SQLCols{}, e.Error()
				}
				//if sqlObj.JoinClause == nil {
				//	sqlObj.JoinClause = make(map[string]interface{})
				//}
				onClause, er := processMapVariable(ctx, tj.GetOnClause(ctx), vars)
				if er != nil {
					logs.WithContext(ctx).Error(er.Error())
				}
				mapObj := make(map[string]interface{})
				mapObj[colTableName] = onClause
				om := OrderedMap{Level: level, SubLevel: sublevel, Rank: len(sqlObj.JoinClause) + 1, Obj: mapObj}
				sqlObj.JoinClause = append(sqlObj.JoinClause, &om)
				joinFound = true
			}
		}
	}
	return sqlCols, err
}

func processWhereClause(ctx context.Context, val interface{}, parentKey string, mainTableName string, isJoinClause bool) (whereClause string, err string) { //, gqr *graphQLRead
	logs.WithContext(ctx).Debug("processWhereClause - Start")

	if val != nil {
		if strings.HasPrefix(parentKey, "CONST_") {
			parentKey = fmt.Sprint("'", strings.Replace(parentKey, "CONST_", "", 1), "'")
		} else if strings.HasPrefix(parentKey, "FIELD_") {
			parentKey = fmt.Sprint(strings.Replace(parentKey, "FIELD_", "", 1))
		} else if !(strings.Contains(parentKey, ".")) {
			parentKey = fmt.Sprint(mainTableName, ".", parentKey)
		}
		switch reflect.TypeOf(val).Kind() {
		case reflect.Map:
			var tempArray []string
			//tempArray := make([]string, len(reflect.ValueOf(val).MapKeys()))
			for _, v := range reflect.ValueOf(val).MapKeys() {
				newVal := reflect.ValueOf(val).MapIndex(v).Interface()
				if newVal != nil {
					var valPrefix, valSuffix = "", ""
					if reflect.TypeOf(newVal).Kind().String() == "string" {
						if !strings.Contains(newVal.(string), ".") {
							valPrefix = "'"
							valSuffix = "'"
						}
					}
					if v.String() == "$or" || v.String() == "or" {
						if reflect.TypeOf(newVal).Kind().String() != "slice" {
							errStr := "Error : or clause has single element"
							logs.WithContext(ctx).Error(errStr)
							return "", errStr
						}
						s := reflect.ValueOf(newVal)
						innerTempArray := make([]string, s.Len())
						for ii := 0; ii < s.Len(); ii++ {
							innerTempArray[ii], err = processWhereClause(ctx, s.Index(ii).Interface(), v.String(), mainTableName, isJoinClause)
							if err != "" {
								return "", err
							}
						}
						tempArray = append(tempArray, fmt.Sprint("( ", strings.Join(innerTempArray, " or "), " )"))
					} else {
						op := ""
						switch v.String() {
						case "$btw":
							op = " between "
						case "$gte":
							op = " >= "
						case "$lte":
							op = " <= "
						case "$gt":
							op = " > "
						case "$lt":
							op = " < "
						case "$eq":
							op = " = "
						case "$ne":
							op = " <> "
						case "$in":
							op = " in "
						case "$nin":
							op = " not in "
						case "$like":
							op = " like "
						default:
							op = ""
						}
						switch v.String() {
						case "$gte", "$lte", "$gt", "$lt", "$eq", "$ne":
							valTmp := reflect.ValueOf(newVal).String()
							if strings.HasPrefix(valTmp, "FIELD_") {
								valTmp = strings.Replace(valTmp, "FIELD_", "", -1)
								valPrefix = ""
								valSuffix = ""
							}
							tempArray = append(tempArray, fmt.Sprint(parentKey, op, valPrefix, valTmp, valSuffix))
						case "$like":
							tempArray = append(tempArray, fmt.Sprint(parentKey, op, valPrefix, "%", reflect.ValueOf(newVal), "%", valSuffix))
						case "$btw":
							btwClause, ok := reflect.ValueOf(newVal).Interface().(map[string]interface{})
							if !ok {
								logs.WithContext(ctx).Warn("between clause is not a map")
							}
							preFix := "'"
							//checking only from value to determine with values recevied are int/float to avoid adding single quote in sql
							_, Interr := strconv.Atoi(btwClause["from"].(string))
							if Interr == nil {
								preFix = ""
							}
							if _, flErr := strconv.ParseFloat(btwClause["from"].(string), 64); flErr == nil {
								preFix = ""
							}
							btwClauseStr := fmt.Sprint(preFix, btwClause["from"], preFix, " and ", preFix, btwClause["to"], preFix)
							tempArray = append(tempArray, fmt.Sprint(parentKey, op, btwClauseStr))
						case "$null":
							nullValue := fmt.Sprint(reflect.ValueOf(newVal))
							if nullValue == "true" {
								tempArray = append(tempArray, fmt.Sprint(parentKey, " IS NULL "))
							} else {
								tempArray = append(tempArray, fmt.Sprint(parentKey, " IS NOT NULL "))
							}
						case "$in", "$nin": //TODO to pass json variable aaray and check if the replaced array is passed as single string or string of values to sql
							switch reflect.TypeOf(newVal).Kind() {
							case reflect.String:
								s := reflect.ValueOf(newVal)
								if strings.HasPrefix(s.String(), "$") {

								}
							case reflect.Slice:
								s := reflect.ValueOf(newVal)
								temp := make([]string, s.Len())
								for i := 0; i < s.Len(); i++ {
									ss := s.Index(i).Interface()
									if reflect.TypeOf(ss).Kind().String() == "string" {
										temp[i] = fmt.Sprint("'", ss, "'")
									} else {
										temp[i] = fmt.Sprint(ss)
									}
								}
								tempArray = append(tempArray, fmt.Sprint(parentKey, op, "(", strings.Join(temp, " , "), ")"))
							default:
								logs.WithContext(ctx).Warn(fmt.Sprint("skipping $in and $nin clause as it needs array as a value but recevied ", newVal))
							}
						default:
							str := ""
							str, err = processWhereClause(ctx, newVal, eru_utils.ReplaceUnderscoresWithDots(v.String()), mainTableName, isJoinClause)
							if str == "" {
								logs.WithContext(ctx).Warn(fmt.Sprint("skipping whereclause for ", newVal, " as there is no value provided by user  : ", str))
							} else {
								tempArray = append(tempArray, str)
							}
							if err != "" {
								logs.WithContext(ctx).Error(err)
								return "", err
							}
						}
					}
				}
			}
			if len(tempArray) > 0 {
				return fmt.Sprint("( ", strings.Join(tempArray, " and "), " )"), ""
			} else {
				return "", ""
			}

		case reflect.String:
			var newVal, valPrefix, valSuffix = "", "", ""
			//TODO due to below statement - 2022-07-27T18:30:00.000Z date in filter is failing if passed in this format
			//parse for date
			if !strings.Contains(reflect.ValueOf(val).String(), ".") || !isJoinClause {
				valPrefix = "'"
				valSuffix = "'"
			}
			newVal = reflect.ValueOf(val).String()
			if strings.HasPrefix(newVal, "FIELD_") {
				valPrefix = ""
				valSuffix = ""
				newVal = strings.Replace(newVal, "FIELD_", "", -1)
			}
			return fmt.Sprint(parentKey, " = ", valPrefix, newVal, valSuffix), ""
		case reflect.Int, reflect.Float32, reflect.Float64:
			return fmt.Sprint(parentKey, " = ", reflect.ValueOf(val)), ""
		default:
			return "", ""
		}
	}
	return "", ""
}

func (sqlObj *SQLObjectQ) processSortClause(ctx context.Context, val interface{}) (sortClause string) {
	logs.WithContext(ctx).Debug("processSortClause - Start")
	if val != nil {
		isDesc := ""
		_ = isDesc
		//v, e := ParseAstValue(val, vars)
		switch reflect.TypeOf(val).Kind() {
		case reflect.Slice:
			s := reflect.ValueOf(val)
			temp := make([]string, s.Len())
			for i := 0; i < s.Len(); i++ {
				isDesc = ""
				si, ok := s.Index(i).Interface().(int)
				if ok {
					if si < 0 {
						isDesc = " desc"
						si = si * -1
					}
					temp[i] = fmt.Sprint(si, " ", isDesc)
				} else {
					ss := fmt.Sprintf("%s", s.Index(i))
					if strings.HasPrefix(ss, "-") {
						isDesc = " desc"
						ss = strings.Replace(ss, "-", "", 1)
					}
					if strings.Contains(ss, ".") {
						temp[i] = ss + isDesc
					} else {
						temp[i] = fmt.Sprintf("%s%s%s%s", sqlObj.MainTableName, ".", ss, isDesc)
					}
				}
			}
			return fmt.Sprint(" order by ", strings.Join(temp, " , "))
		case reflect.String:
			s := fmt.Sprintf("%s", reflect.ValueOf(val))
			if strings.HasPrefix(s, "-") {
				isDesc = " desc"
				s = strings.Replace(s, "-", "", 1)
			}
			if strings.Contains(eru_utils.ReplaceUnderscoresWithDots(s), ".") {
				return fmt.Sprint(" order by ", eru_utils.ReplaceUnderscoresWithDots(s), isDesc)
			} else {
				return fmt.Sprint(" order by ", sqlObj.MainTableName, ".", s, isDesc)
			}
		case reflect.Int:
			s := reflect.ValueOf(val).Int()
			if s < 0 {
				isDesc = " desc"
				s = s * -1
			}
			return fmt.Sprint(" order by ", s, isDesc)
		case reflect.Float64:
			s := reflect.ValueOf(val).Float()
			if s < 0 {
				isDesc = " desc"
				s = s * -1
			}
			return fmt.Sprint(" order by ", s, isDesc)
		default:
		}
	}
	return ""
}
func (sqlObj *SQLObjectQ) processJoins(ctx context.Context, val []*OrderedMap) (strJoinClause string) {
	logs.WithContext(ctx).Debug("processJoins - Start")

	sort.Sort(MapSorter(val))

	for _, obj := range val {
		for tableName, v := range obj.Obj {
			joinType := "LEFT" //default join value TODO schema joins has an option to define join type
			onClause := ""
			switch reflect.TypeOf(v).Kind() {
			case reflect.Map:
				for _, vv := range reflect.ValueOf(v).MapKeys() { //TODO remove reflect usage
					if vv.String() == "joinType" {
						jt, err := reflect.ValueOf(v).MapIndex(vv).Interface().(string)
						if !err {
							logs.WithContext(ctx).Warn("joinType value is not a string")
						}
						switch jt {
						case "LEFT", "RIGHT", "INNER":
							joinType = jt
						default:
							logs.WithContext(ctx).Warn("valid values for joinType are LEFT RIGHT and INNER ")
						}
					} else if vv.String() == "on" {
						oc, _ := processWhereClause(ctx, reflect.ValueOf(v).MapIndex(vv).Interface(), "", sqlObj.MainTableName, true)
						onClause = oc
					}
				}
				strJoinClause = fmt.Sprint(strJoinClause, " ", fmt.Sprint(joinType, " JOIN ", tableName, " on ", onClause))
			default:
				//do nothing
			}
		}
	}
	return strJoinClause
}

func (sqlObj *SQLObjectQ) MakeQuery(ctx context.Context, sqlMaker ds.SqlMakerI, withColAlias bool) (err error) {
	logs.WithContext(ctx).Debug("MakeQuery - Start")
	strDistinct := ""
	strGroupClause := ""
	strColums := ""
	if withColAlias {
		strColums = strings.Join(sqlObj.Columns.ColWithAlias, " , ")
	} else {
		strColums = strings.Join(sqlObj.Columns.ColNames, " , ")
	}
	strJoinClause := sqlObj.processJoins(ctx, sqlObj.JoinClause)
	strWhereClause, e := processWhereClause(ctx, sqlObj.WhereClause, "", sqlObj.MainTableName, false)
	if e != "" {
		err = errors.New(e)
	}

	strAnd := ""
	strSecurityClause := ""
	for _, v := range sqlObj.SecurityClause {
		if v != "" {
			strSecurityClause = fmt.Sprint(strSecurityClause, strAnd, v)
			strAnd = " and "
		}
	}
	if strSecurityClause != "" {
		if strWhereClause != "" {
			strWhereClause = fmt.Sprint(strWhereClause, " and ", strSecurityClause)
		} else {
			strWhereClause = strSecurityClause
		}
	}
	if strWhereClause != "" {
		strWhereClause = fmt.Sprint(" where ", strWhereClause)
	}

	strSortClause := sqlObj.processSortClause(ctx, sqlObj.SortClause)
	if sqlObj.HasAggregate && len(sqlObj.Columns.GroupClause) > 0 {
		strGroupClause = fmt.Sprint(" group by ", strings.Join(sqlObj.Columns.GroupClause, " , "))
	}
	if sqlObj.DistinctResults {
		strDistinct = " distinct "
	}

	fromTable := sqlObj.MainTableName
	withClause := ""
	if sqlObj.WithQuery != "" {
		fromTable = fmt.Sprint("( ", sqlObj.WithQuery, " ) ", sqlObj.MainTableName)
	}
	sqlObj.DBQuery = fmt.Sprint(withClause, "select ", strDistinct, strColums, " from ", fromTable, " ", strJoinClause, " ", strWhereClause, " ", strGroupClause, strSortClause)

	sqlObj.DBQuery = sqlMaker.AddLimitSkipClause(ctx, sqlObj.DBQuery, sqlObj.Limit, sqlObj.Skip, 1000)
	return err
}
