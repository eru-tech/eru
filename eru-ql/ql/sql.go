package ql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"github.com/eru-tech/eru/eru-read-write/eru_writes"
	"sort"
	"strings"
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
	}
	queryObjs = append(queryObjs, queryObj)
	return res, queryObjs, err
}
