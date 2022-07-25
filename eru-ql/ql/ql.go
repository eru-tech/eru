package ql

import (
	"encoding/json"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"log"
)

type QLData struct {
	Query          string                    `json:"query"`
	Variables      map[string]interface{}    `json:"variables"`
	FinalVariables map[string]interface{}    `json:"-"`
	ExecuteFlag    bool                      `json:"-"`
	SecurityRule   module_model.SecurityRule `json:"security_rule"`
}

type QueryObject struct {
	Query string
	Cols  string
	Type  string
}

type QL interface {
	Execute(projectId string, datasources map[string]*module_model.DataSource, s module_store.ModuleStoreI) (res []map[string]interface{}, queryObjs []QueryObject, err error)
	SetQLData(mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool)
}

func (qld *QLData) SetQLDataCommon(mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool) (err error) {
	qld.Query = mq.Query
	qld.Variables = mq.Vars
	qld.ExecuteFlag = executeFlag
	err = qld.SetFinalVars(vars)
	return err
}
func (qld *QLData) SetFinalVars(vars map[string]interface{}) (err error) {
	tmpVars, _ := json.Marshal(qld.Variables)
	finalVars := make(map[string]interface{})
	err = json.Unmarshal(tmpVars, &finalVars) // Marshall UnMarshall used to copy without referencing of map
	if err != nil {
		log.Print(err)
		return err
	}
	//commented below copying and replaced with above Marshall/UnMarshall to copy without referencing of map
	//for k, v := range myQuery.Vars {
	//	finalVars[k] = v
	//}
	if finalVars == nil {
		finalVars = make(map[string]interface{})
	}
	for k, v := range vars {
		finalVars[k] = v
	}
	qld.FinalVariables = finalVars
	return nil
}
