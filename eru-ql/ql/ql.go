package ql

import (
	"encoding/json"
	"errors"
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
	IsPublic       bool                       `json:"is_public"`
}

type QueryObject struct {
	Query string
	Cols  string
	Type  string
}

type QL interface {
	Execute(projectId string, datasources map[string]*module_model.DataSource, s module_store.ModuleStoreI) (res []map[string]interface{}, queryObjs []QueryObject, err error)
	SetQLData(mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool, tokenObj map[string]interface{}, isPublic bool)
	ProcessTransformRule(tr module_model.TransformRule) (outputObj map[string]interface{}, err error)
}

func (qld *QLData) SetQLDataCommon(mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool, tokenObj map[string]interface{}, isPublic bool) (err error) {
	if mq.Vars == nil {
		mq.Vars = make(map[string]interface{})
	}
	mq.Vars[module_model.RULEPREFIX_TOKEN] = tokenObj
	qld.Query = mq.Query
	qld.Variables = mq.Vars
	qld.ExecuteFlag = executeFlag
	qld.IsPublic = isPublic
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
		if len(tr.Rules) > 0 {
			for k, v := range tr.Rules[0].ForceColumnValues { //todo to remove array and make it single object
				outputBytes, err := processTemplate("xxx", v, qld.FinalVariables, "string", k)
				if err != nil {
					return nil, err
				}
				if outputBytes != nil {
					outputObj[k] = string(outputBytes)
					//skipping to write to overwriteobj if there is no output
				}
			}
		}
	}
	log.Print("ProcessTransformRule output = ", outputObj)
	return
}

func processSecurityRule(sr security_rule.SecurityRule, vars map[string]interface{}) (outputStr string, err error) {
	if sr.RuleType == module_model.RULETYPE_NONE {
		err = errors.New("Security Rule Set to NONE")
		return
	}
	if sr.RuleType == module_model.RULETYPE_ALWAYS {
		//do nothing
		return
	} else if sr.RuleType == module_model.RULETYPE_CUSTOM {
		outputStr, err = sr.Stringify(vars, false)

	}
	return
}

func processTemplate(templateName string, templateString string, vars map[string]interface{}, outputType string, key string) (output []byte, err error) {
	//log.Println("inside processTemplate with template = ", templateString)
	ruleValue := strings.SplitN(templateString, ".", 2)
	//log.Print(ruleValue)
	//log.Print("len(ruleValue) = ", len(ruleValue))
	templateStr := ""
	if len(ruleValue) > 1 {
		templateStr = ruleValue[1]
	} else {
		templateStr = ruleValue[0]
	}
	if ruleValue[0] == module_model.RULEPREFIX_TOKEN {
		return executeTemplate(templateName, templateStr, vars[module_model.RULEPREFIX_TOKEN], outputType)
	} else if ruleValue[0] == module_model.RULEPREFIX_DOCS {
		var docs []interface{}
		isArray := false
		if d, ok := vars["docs"]; ok {
			docs, isArray = d.([]interface{})
			if !isArray {
				dd, er := d.(map[string]interface{}) // checking if docs is a single document without array
				if !er {
					return nil, errors.New("error while parsing value of 'docs'")
				}
				docs = append(docs, dd)
			}
		} else {
			err = errors.New("docs keyword not found while transforming the doc")
		}
		for i, doc := range docs {
			dd, er := doc.(map[string]interface{}) // checking if docs is a single document without array
			if !er {
				return nil, errors.New("error while parsing value of 'docs'")
			}
			outputBytes, ptErr := executeTemplate(templateName, templateStr, dd, outputType)
			if err != nil {
				err = ptErr
				log.Print(err)
				return
			}
			dd[key] = string(outputBytes)
			docs[i] = dd
			outputBytes = nil
		}
	} else if ruleValue[0] == module_model.RULEPREFIX_NONE {
		log.Print(vars)
		return executeTemplate(templateName, templateStr, vars, outputType)
	}
	return
}

func executeTemplate(templateName string, templateString string, vars interface{}, outputType string) (output []byte, err error) {
	goTmpl := gotemplate.GoTemplate{templateName, templateString}
	outputObj, err := goTmpl.Execute(vars, outputType)
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
	return
}
