package ql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"github.com/eru-tech/eru/eru-security-rule/security_rule"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	"strings"
)

type QLData struct {
	Query          string                     `json:"query"`
	Variables      map[string]interface{}     `json:"variables"`
	FinalVariables map[string]interface{}     `json:"-"`
	ExecuteFlag    bool                       `json:"-"`
	SecurityRule   security_rule.SecurityRule `json:"security_rule"`
	IsPublic       bool                       `json:"is_public"`
	OutputType     string                     `json:"output_type"`
}

type QueryObject struct {
	Query string
	Cols  string
	Type  string
}

type QL interface {
	Execute(ctx context.Context, projectId string, datasources map[string]*module_model.DataSource, s module_store.ModuleStoreI, outputType string) (res []map[string]interface{}, queryObjs []QueryObject, err error)
	SetQLData(ctx context.Context, mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool, tokenObj map[string]interface{}, isPublic bool, outputType string)
	ProcessTransformRule(ctx context.Context, tr module_model.TransformRule, docs interface{}) (outputObj map[string]interface{}, err error)
}

func (qld *QLData) SetQLDataCommon(ctx context.Context, mq module_model.MyQuery, vars map[string]interface{}, executeFlag bool, tokenObj map[string]interface{}, isPublic bool, outputType string) (err error) {
	logs.WithContext(ctx).Debug("SetQLDataCommon - Start")
	if mq.Vars == nil {
		mq.Vars = make(map[string]interface{})
	}
	//mq.Vars[module_model.RULEPREFIX_TOKEN] = tokenObj
	qld.Query = mq.Query
	qld.Variables = mq.Vars
	qld.ExecuteFlag = executeFlag
	qld.IsPublic = isPublic
	qld.OutputType = outputType
	err = qld.SetFinalVars(ctx, vars)
	if tokenObj != nil {
		qld.FinalVariables[module_model.RULEPREFIX_TOKEN] = tokenObj
	}
	return err
}
func (qld *QLData) SetFinalVars(ctx context.Context, vars map[string]interface{}) (err error) {
	logs.WithContext(ctx).Debug("SetFinalVars - Start")
	tmpVars, _ := json.Marshal(qld.Variables)
	finalVars := make(map[string]interface{})
	err = json.Unmarshal(tmpVars, &finalVars) // Marshall UnMarshall used to copy without referencing of map
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
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
func (qld *QLData) ProcessTransformRule(ctx context.Context, tr module_model.TransformRule, docs interface{}) (outputObj map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ProcessTransformRule - Start")
	if tr.RuleType == module_model.RULETYPE_NONE {
		outputObj = make(map[string]interface{})
		return
	}
	if tr.RuleType == module_model.RULETYPE_ALWAYS {
		outputObj = make(map[string]interface{})
		if len(tr.Rules) > 0 {
			for k, v := range tr.Rules[0].ForceColumnValues {
				//todo to remove array and make it single object
				outputBytes, err := processTemplate(ctx, "xxx", v, qld.FinalVariables, "string", k, docs)
				if err != nil {
					return nil, err
				}
				if outputBytes != nil {
					outputObj[k] = string(outputBytes)
					//skipping to write to overwriteobj if there is no output
				}
				logs.WithContext(ctx).Info(fmt.Sprint(outputObj[k]))
			}
		}
	}
	return
}

func processSecurityRule(ctx context.Context, sr security_rule.SecurityRule, vars map[string]interface{}) (outputStr string, err error) {
	logs.WithContext(ctx).Debug("processSecurityRule - Start")
	if sr.RuleType == module_model.RULETYPE_NONE {
		err = errors.New("Security Rule Set to NONE")
		return
	}
	if sr.RuleType == module_model.RULETYPE_ALWAYS {
		//do nothing
		return
	} else if sr.RuleType == module_model.RULETYPE_CUSTOM {
		outputStr, err = sr.Stringify(ctx, vars, false)

	}
	return
}

func processTemplate(ctx context.Context, templateName string, templateString string, vars map[string]interface{}, outputType string, key string, d interface{}) (output []byte, err error) {
	logs.WithContext(ctx).Debug("processTemplate - Start")
	ruleValue := strings.SplitN(templateString, ".", 2)
	templateStr := ""
	if len(ruleValue) > 1 {
		templateStr = ruleValue[1]
	} else {
		templateStr = ruleValue[0]
	}
	if ruleValue[0] == module_model.RULEPREFIX_TOKEN {
		return executeTemplate(ctx, templateName, templateStr, vars[module_model.RULEPREFIX_TOKEN], outputType)
	} else if ruleValue[0] == module_model.RULEPREFIX_DOCS {
		var docs []interface{}
		isArray := false

		docs, isArray = d.([]interface{})
		if !isArray {
			dd, er := d.(map[string]interface{}) // checking if docs is a single document without array
			if !er {
				return nil, errors.New("error while parsing value of 'docs'")
			}
			docs = append(docs, dd)
		}

		for i, doc := range docs {
			dd, er := doc.(map[string]interface{}) // checking if docs is a single document without array
			if !er {
				return nil, errors.New("error while parsing value of 'docs'")
			}
			outputBytes, ptErr := executeTemplate(ctx, templateName, templateStr, dd, outputType)
			if err != nil {
				err = ptErr
				logs.WithContext(ctx).Error(err.Error())
				return
			}
			dd[key] = string(outputBytes)
			docs[i] = dd
			outputBytes = nil
		}
	} else if ruleValue[0] == module_model.RULEPREFIX_NONE {
		return executeTemplate(ctx, templateName, templateStr, vars, outputType)
	}
	return
}

func executeTemplate(ctx context.Context, templateName string, templateString string, vars interface{}, outputType string) (output []byte, err error) {
	logs.WithContext(ctx).Debug("executeTemplate - Start")
	goTmpl := gotemplate.GoTemplate{templateName, templateString}
	outputObj, err := goTmpl.Execute(ctx, vars, outputType)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	} else if outputType == "string" {
		return []byte(outputObj.(string)), nil
	} else {
		output, err = json.Marshal(outputObj)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
	}
	return
}
