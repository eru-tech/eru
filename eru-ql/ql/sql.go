package ql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"github.com/eru-tech/eru/eru-read-write/eru_writes"
)

type SQLData struct {
	QLData
	DBAlias string `json:"db_alias"`
	Cols    string `json:"cols"`
}

func (sqd *SQLData) SetQLData(ctx context.Context, mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool, tokenObj map[string]interface{}, isPublic bool, outputType string) {
	logs.WithContext(ctx).Debug("SetQLData - Start")
	sqd.SetQLDataCommon(ctx, mq, vars, executeFlag, tokenObj, isPublic, outputType)
	//sqd.Query=mq.Query
	//sqd.Variables=mq.Vars
	sqd.DBAlias = mq.DBAlias
	sqd.Cols = mq.Cols
	//sqd.SetFinalVars(vars)
}
func (sqd *SQLData) Execute(ctx context.Context, projectId string, datasources map[string]*module_model.DataSource, s module_store.ModuleStoreI, outputType string) (res []map[string]interface{}, queryObjs []QueryObject, err error) {
	logs.WithContext(ctx).Debug("Execute of Sql - Start")
	datasource := datasources[sqd.DBAlias]
	if datasource == nil {
		return nil, nil, errors.New(fmt.Sprint("dbAlias ", sqd.DBAlias, " not found"))
	}
	var result map[string]interface{}
	sr := ds.GetSqlMaker(datasource.DbName)

	var keyList []string
	for key, _ := range sqd.FinalVariables {
		keyList = append(keyList, key)
	}
	sort.Strings(keyList)
	for _, k := range keyList {
		v := sqd.FinalVariables[k]
		var str string
		switch tp := v.(type) {
		case []interface{}:
			isMap := false
			if iArray, ok := v.([]interface{}); ok {
				for i, strText := range iArray {
					isMap = false
					sep := ""
					if i > 0 {
						sep = " , "
					}
					if txt, txtOk := strText.(string); txtOk {
						str = fmt.Sprint(str, sep, "'", txt, "'")
					} else if _, mapVOk := strText.(map[string]interface{}); mapVOk {
						isMap = true
						break
					}
				}
				if isMap {
					mapJ, mapJErr := json.Marshal(iArray)
					if mapJErr != nil {
						logs.WithContext(ctx).Error(mapJErr.Error())
						return nil, nil, mapJErr
					}
					str = string(mapJ)
				}
			}
			sqd.FinalVariables[k] = str
			sqd.Query = strings.Replace(sqd.Query, fmt.Sprint("$", k), str, -1)
			break
		default:
			// do noting
			_ = tp
			break
		}
	}

	for _, k := range keyList {
		v := sqd.FinalVariables[k]
		//ignoring processing token variable
		if k != module_model.RULEPREFIX_TOKEN {

			err = sr.VerifyForBlockedWords(ctx, k, v, sr)
			if err != nil {
				return
			}

			var str string
			switch tp := v.(type) {
			case float64:
				str = fmt.Sprint(v.(float64))
				break
			case string:
				str = v.(string)
				vBytes, err := processTemplate(ctx, "variable", str, sqd.FinalVariables, "string", "", nil)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, nil, err
				}
				if string(vBytes) != "" {
					str = string(vBytes)
				}
				break
			case map[string]interface{}:
				strBytes, strBytesErr := json.Marshal(v)
				err = strBytesErr
				str = string(strBytes)
				break
			default:
				logs.WithContext(ctx).Warn(fmt.Sprint("Unhandled type for : ", tp))
				// do noting
				break
			}
			//logs.WithContext(ctx).Info(fmt.Sprint(k, " = ", str))
			sqd.FinalVariables[k] = str
			sqd.Query = strings.Replace(sqd.Query, fmt.Sprint("$", k), str, -1)
		}
	}

	queryObj := QueryObject{}
	queryObj.Query = sqd.Query
	queryObj.Cols = sqd.Cols
	if sqd.ExecuteFlag {
		if sqd.OutputType == eru_writes.OutputTypeCsv || sqd.OutputType == eru_writes.OutputTypeExcel {
			result, err = sr.ExecuteQueryForCsv(ctx, sqd.Query, datasource, "Results")
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			res = append(res, result)
		} else {
			result, err = sr.ExecutePreparedQuery(ctx, sqd.Query, datasource)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
			}
			res = append(res, result)
		}
	} else if sqd.OutputType == "ast" {
		fmt.Println(sqd.Query)
		tableNames := sr.ExtractTableNames(ctx, sqd.Query)
		sort.Sort(module_model.MapSorterTable(tableNames.Tables))

		for _, table := range tableNames.Tables {
			if !(strings.HasSuffix(table.Obj.TableName, "___ALL")) {
				if !(strings.Contains(table.Obj.TableName, ".")) {
					table.Obj.TableName = fmt.Sprint(sr.DefaultSchemaName(), table.Obj.TableName)
				}
				sRulesStr, srJoins, srErr := getTableSecurityRule(ctx, projectId, sqd.DBAlias, table.Obj.TableName, s, "query", sqd.FinalVariables)
				if srErr == nil {
					q := fmt.Sprint("select  x.* from ", table.Obj.TableName, " x ")
					for _, srJoin := range srJoins {
						tj, e := datasource.GetTableJoins(ctx, table.Obj.TableName, srJoin, make(map[string]string))
						if e != nil {
							logs.WithContext(ctx).Error(e.Error())
							return nil, nil, e
						}
						onClause, er := processMapVariable(ctx, tj.GetOnClause(ctx), sqd.FinalVariables)
						if er != nil {
							logs.WithContext(ctx).Error(er.Error())
							return
						}
						oc, _ := processWhereClause(ctx, onClause, "", table.Obj.TableName, true, false)
						q = fmt.Sprint(q, " left join ", srJoin, " on ", oc)
					}
					q = fmt.Sprint(q, " where ", sRulesStr)
					sqd.Query = strings.Replace(sqd.Query, fmt.Sprint(table.Obj.TableKeyPrefix, table.Obj.TableKey, table.Obj.TableKeySuffix), fmt.Sprint(table.Obj.TableKeyPrefix, " (", q, ") ", table.Obj.AliasName, " ", table.Obj.TableKeySuffix), -1)
				}
			} else {
				sqd.Query = strings.Replace(sqd.Query, table.Obj.TableName, strings.Replace(table.Obj.TableName, "___ALL", "", -1), -1)
			}
		}
		res = append(res, map[string]interface{}{"sql": sqd.Query, "ast": tableNames.Tables})
	}
	queryObjs = append(queryObjs, queryObj)
	return res, queryObjs, err
}
