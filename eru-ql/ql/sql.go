package ql

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"log"
	"strings"
)

type SQLData struct {
	QLData
	DBAlias string `json:"dbalias"`
	Cols    string `json:"cols"`
}

func (sqd *SQLData) SetQLData(mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool) {
	log.Print("inside SetQLData of SQLData")
	sqd.SetQLDataCommon(mq, vars, executeFlag)
	//sqd.Query=mq.Query
	//sqd.Variables=mq.Vars
	sqd.DBAlias = mq.DBAlias
	sqd.Cols = mq.Cols
	//sqd.SetFinalVars(vars)
}
func (sqd *SQLData) Execute(projectId string, datasources map[string]*module_model.DataSource, s module_store.ModuleStoreI) (res []map[string]interface{}, queryObjs []QueryObject, err error) {
	log.Print("inside ExecuteSQL of SQLData")
	datasource := datasources[sqd.DBAlias]
	if datasource == nil {
		return nil, nil, errors.New(fmt.Sprint("dbAlias ", sqd.DBAlias, " not found"))
	}
	var result map[string]interface{}
	sr := ds.GetSqlMaker(datasource.DbName)
	log.Print("sqd.FinalVariables  = ", sqd.FinalVariables)
	for k, v := range sqd.FinalVariables {
		var str string
		switch tp := v.(type) {
		case float64:
			str = fmt.Sprint(v.(float64))
		case string:
			str = v.(string)
		default:
			log.Print(tp)
			// do noting
		}
		log.Print(k, " = ", str)
		sqd.Query = strings.Replace(sqd.Query, fmt.Sprint("$", k), str, 10)
	}
	queryObj := QueryObject{}
	queryObj.Query = sqd.Query
	queryObj.Cols = sqd.Cols
	log.Print("sqd.ExecuteFlag = ", sqd.ExecuteFlag)
	if sqd.ExecuteFlag {
		result, err = sr.ExecutePreparedQuery(sqd.Query, datasource)
		log.Print(err)
		res = append(res, result)
	}
	queryObjs = append(queryObjs, queryObj)
	return res, queryObjs, err
}
