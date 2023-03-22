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
	"github.com/eru-tech/eru/eru-writes/eru_writes"
	"strings"
)

type SQLData struct {
	QLData
	DBAlias string `json:"dbalias"`
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
	for k, v := range sqd.FinalVariables {
		var str string
		switch tp := v.(type) {
		case float64:
			str = fmt.Sprint(v.(float64))
			break
		case string:
			str = v.(string)
			break
		case map[string]interface{}:
			strBytes, strBytesErr := json.Marshal(v)
			err = strBytesErr
			str = string(strBytes)
			break
		case []interface{}:
			if iArray, ok := v.([]interface{}); ok {
				for i, strText := range iArray {
					sep := ""
					if i > 0 {
						sep = " , "
					}
					str = fmt.Sprint(str, sep, "'", strText, "'")
				}
			}
			break
		default:
			logs.WithContext(ctx).Warn(fmt.Sprint("Unhandled type for : ", tp))
			// do noting
			break
		}
		logs.WithContext(ctx).Debug(fmt.Sprint(k, " = ", str))
		sqd.Query = strings.Replace(sqd.Query, fmt.Sprint("$", k), str, -1)

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
