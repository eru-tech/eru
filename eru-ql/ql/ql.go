package ql

import (
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"github.com/eru-tech/eru/eru-security-rule/security_rule"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	"log"
	"strings"
)

type QLData struct {
	Query          string                     `json:"query"`
	Variables      map[string]interface{}     `json:"variables"`
	FinalVariables map[string]interface{}     `json:"-"`
	ExecuteFlag    bool                       `json:"-"`
	SecurityRule   security_rule.SecurityRule `json:"security_rule"`
}

type QueryObject struct {
	Query string
	Cols  string
	Type  string
}

type QL interface {
	Execute(projectId string, datasources map[string]*module_model.DataSource, s module_store.ModuleStoreI) (res []map[string]interface{}, queryObjs []QueryObject, err error)
	SetQLData(mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool, tokenObj map[string]interface{})
	ProcessTransformRule(tr module_model.TransformRule) (outputObj map[string]interface{}, err error)
}

func (qld *QLData) SetQLDataCommon(mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool, tokenObj map[string]interface{}) (err error) {
	if mq.Vars == nil {
		mq.Vars = make(map[string]interface{})
	}
	mq.Vars[module_model.RULEPREFIX_TOKEN] = tokenObj
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
func (qld *QLData) ProcessTransformRule(tr module_model.TransformRule) (outputObj map[string]interface{}, err error) {
	if tr.RuleType == module_model.RULETYPE_NONE {
		outputObj = make(map[string]interface{})
		return
	}
	if tr.RuleType == module_model.RULETYPE_ALWAYS {
		outputObj = make(map[string]interface{})
		log.Print(tr.Rules)
		if len(tr.Rules) > 0 {
			log.Print("inside len(tr.Rules)>0")
			for k, v := range tr.Rules[0].ForceColumnValues { //todo to remove array and make it single object
				log.Print(qld.Variables["token"])
				log.Print(v)
				outputBytes, err := processTemplate("xxx", v, qld.Variables, "string")
				if err != nil {
					return nil, err
				}
				outputObj[k] = string(outputBytes)
			}
		}
	}
	log.Print(outputObj)
	return
}

func processTemplate(templateName string, templateString string, vars map[string]interface{}, outputType string) (output []byte, err error) {
	log.Println("inside processTemplate")
	ruleValue := strings.SplitN(templateString, ".", 2)
	if ruleValue[0] == module_model.RULEPREFIX_TOKEN {
		templateStr := fmt.Sprint("{{ .", ruleValue[1], " }}")
		goTmpl := gotemplate.GoTemplate{templateName, templateStr}
		outputObj, err := goTmpl.Execute(vars[module_model.RULEPREFIX_TOKEN], outputType)
		if err != nil {
			log.Println(err)
			return nil, err
		} else if outputType == "string" {
			return []byte(outputObj.(string)), nil
		} else {
			output, err = json.Marshal(outputObj)
			if err != nil {
				log.Println(err)
				return nil, err
			}
		}
	}
	//todo - to add if prefix is not token
	return
}
