package ql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-ql/ds"
	"github.com/eru-tech/eru/eru-ql/module_model"
	"github.com/eru-tech/eru/eru-ql/module_store"
	"github.com/eru-tech/eru/eru-security-rule/security_rule"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	eru_utils "github.com/eru-tech/eru/eru-utils"
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

func processSecurityRule(ctx context.Context, sr security_rule.SecurityRule, vars map[string]interface{}, mainTableName string, ctjMap map[string]string) (outputStr string, templates []string, ptables []string, err error) {
	logs.WithContext(ctx).Debug("processSecurityRule - Start")
	if sr.RuleType == module_model.RULETYPE_NONE {
		err = errors.New("Security Rule Set to NONE")
		return
	}
	if sr.RuleType == module_model.RULETYPE_ALWAYS {
		//do nothing
		return
	} else if sr.RuleType == module_model.RULETYPE_CUSTOM {
		outputStr, templates, ptables, err = sr.Stringify(ctx, vars, false, mainTableName, ctjMap)

	}
	return
}

// func processTemplate(ctx context.Context, templateName string, templateString string, vars map[string]interface{}, outputType string, key string, d interface{}) (output []byte, err error) {
// 	logs.WithContext(ctx).Debug("processTemplate - Start")
// 	ruleValue := strings.SplitN(templateString, ".", 2)
// 	templateStr := ""
// 	if len(ruleValue) > 1 {
// 		templateStr = ruleValue[1]
// 	} else {
// 		templateStr = ruleValue[0]
// 	}
// 	if ruleValue[0] == module_model.RULEPREFIX_TOKEN {
// 		templateStr = ruleValue[1]
// 		if !(strings.HasPrefix(ruleValue[1], "{{")) {
// 			templateStr = fmt.Sprint("{{ .", ruleValue[1], " }}")
// 		}
// 		templateStr = eru_utils.ReplaceVariables(ctx, templateStr, vars)
// 		return executeTemplate(ctx, templateName, templateStr, vars[module_model.RULEPREFIX_TOKEN], outputType)
// 	} else if ruleValue[0] == module_model.RULEPREFIX_DOCS {
// 		var docs []interface{}
// 		isArray := false

// 		docs, isArray = d.([]interface{})
// 		if !isArray {
// 			dd, er := d.(map[string]interface{}) // checking if docs is a single document without array
// 			if !er {
// 				return nil, errors.New("error while parsing value of 'docs'")
// 			}
// 			docs = append(docs, dd)
// 		}

// 		for i, doc := range docs {
// 			dd, er := doc.(map[string]interface{}) // checking if docs is a single document without array
// 			if !er {
// 				return nil, errors.New("error while parsing value of 'docs'")
// 			}
// 			outputBytes, ptErr := executeTemplate(ctx, templateName, templateStr, dd, outputType)
// 			if err != nil {
// 				err = ptErr
// 				logs.WithContext(ctx).Error(err.Error())
// 				return
// 			}
// 			dd[key] = string(outputBytes)
// 			docs[i] = dd
// 			outputBytes = nil
// 		}
// 	} else if ruleValue[0] == module_model.RULEPREFIX_NONE {
// 		return executeTemplate(ctx, templateName, templateStr, vars, outputType)
// 	} else {
// 		return executeTemplate(ctx, templateName, templateStr, vars, outputType)
// 	}
// 	return
// }

func processTemplate(ctx context.Context, templateName string, templateString string, vars map[string]interface{}, outputType string, key string, d interface{}) (output []byte, err error) {
	logs.WithContext(ctx).Debug("processTemplate - Start")
	ruleValue := strings.SplitN(templateString, ".", 2)
	templateStr := ""
	if ruleValue[0] == module_model.RULEPREFIX_TOKEN {
		templateStr = ruleValue[1]
		if !(strings.HasPrefix(ruleValue[1], "{{")) {
			templateStr = fmt.Sprint("{{ .", ruleValue[1], " }}")
		}
		templateStr = eru_utils.ReplaceVariables(ctx, templateStr, vars)
		return executeTemplate(ctx, templateName, templateStr, vars[module_model.RULEPREFIX_TOKEN], outputType)
	} else if ruleValue[0] == module_model.RULEPREFIX_DOCS {
		templateStr = ruleValue[1]
		var docs []interface{}
		isArray := false

		docs, isArray = d.([]interface{})
		if !isArray {
			dd, er := d.(map[string]interface{}) // checking if docs is a single document without array
			if !er {
				return nil, logs.Err(ctx, errors.New("error while parsing value of 'docs'"), "")
			}
			docs = append(docs, dd)
		}

		for i, doc := range docs {
			dd, er := doc.(map[string]interface{}) // checking if docs is a single document without array
			if !er {
				return nil, logs.Err(ctx, errors.New("error while parsing value of 'docs'"), "")
			}
			outputBytes, ptErr := executeTemplate(ctx, templateName, templateStr, dd, outputType)
			if err != nil {
				err = logs.Err(ctx, ptErr, "")
				return
			}
			dd[key] = string(outputBytes)
			docs[i] = dd
			outputBytes = nil
		}
	} else if ruleValue[0] == module_model.RULEPREFIX_NONE {
		if len(ruleValue) > 1 {
			templateStr = ruleValue[1]
		} else {
			templateStr = templateString
		}
		return executeTemplate(ctx, templateName, templateStr, vars, outputType)
	} else if strings.Contains(templateString, module_model.RULEINFIX_NONE) {
		templateStr = strings.Replace(templateString, module_model.RULEINFIX_NONE, "", -1)
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
func (qld *QLData) secureSQL(ctx context.Context, query string, projectId string, datasource *module_model.DataSource, s module_store.ModuleStoreI, sr ds.SqlMakerI) (secureQuery string) {
	tableNames := sr.ExtractTableNames(ctx, query)
	for _, table := range tableNames.Tables {
		if !(strings.HasSuffix(table.TableName, "___ALL")) {
			if !(strings.Contains(table.TableName, ".")) {
				table.TableName = fmt.Sprint(sr.DefaultSchemaName(), table.TableName)
			}
			sRulesStr, srJoins, srErr := getTableSecurityRule(ctx, projectId, datasource.DbAlias, table.TableName, s, "query", qld.FinalVariables, table.TableName)
			if srErr != nil {
				logs.WithContext(ctx).Info(srErr.Error())
				if !strings.HasPrefix(srErr.Error(), "TableSecurityRule not defined for "+table.TableName) {
					return
				} else {
					srErr = nil
				}
			}
			if sRulesStr != "" {
				q := fmt.Sprint("select  ", table.TableName, ".* from ", table.TableName)
				for _, srJoin := range srJoins {
					tj, e := datasource.GetTableJoins(ctx, table.TableName, srJoin, make(map[string]string))
					if e != nil {
						logs.WithContext(ctx).Error(e.Error())
						return
					}
					onClause, er := processMapVariable(ctx, tj.GetOnClause(ctx), qld.FinalVariables)
					if er != nil {
						logs.WithContext(ctx).Error(er.Error())
						return
					}
					oc, _ := processWhereClause(ctx, onClause, "", table.TableName, true, false)
					q = fmt.Sprint(q, " left join ", srJoin, " on ", oc)
				}
				q = fmt.Sprint(q, " where ", sRulesStr)
				query = strings.Replace(query, fmt.Sprint(table.TableKeyPrefix, table.TableKey, table.TableKeySuffix), fmt.Sprint(table.TableKeyPrefix, " (", q, ") ", table.AliasName, " ", table.TableKeySuffix), -1)

				makeJsonArrayFnStrKeyWord, err := sr.GetMakeJsonArrayFnStr()
				if err != nil {
					makeJsonArrayFnStrKeyWord = ""
				}
				query = strings.Replace(query, module_model.MAKE_JSON_ARRAY_FN_STR, makeJsonArrayFnStrKeyWord, -1)

				makeJsonArrayFnKeyWord, err := sr.GetMakeJsonArrayFn()
				if err != nil {
					makeJsonArrayFnKeyWord = ""
				}
				query = strings.Replace(query, module_model.MAKE_JSON_ARRAY_FN, makeJsonArrayFnKeyWord, -1)
			}
		} else {
			query = strings.Replace(query, table.TableName, strings.Replace(table.TableName, "___ALL", "", -1), -1)
		}
	}
	secureQuery = query
	return
}
