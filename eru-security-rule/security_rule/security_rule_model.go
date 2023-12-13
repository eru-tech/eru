package security_rule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	"strings"
)

type CustomRule struct {
	AND []CustomRuleDetails `json:",omitempty"`
	OR  []CustomRuleDetails `json:",omitempty"`
}

type CustomRuleDetails struct {
	DataType  string              `json:",omitempty"`
	Variable1 string              `json:",omitempty"`
	Variable2 string              `json:",omitempty"`
	Operator  string              `json:",omitempty"`
	ErrorMsg  string              `json:",omitempty"`
	AND       []CustomRuleDetails `json:",omitempty"`
	OR        []CustomRuleDetails `json:",omitempty"`
}

type SecurityRule struct {
	RuleType   string
	CustomRule CustomRule
}

func (sr SecurityRule) Stringify(ctx context.Context, vars map[string]interface{}, ignoreIfNotFound bool) (str string, err error) {
	logs.WithContext(ctx).Debug("Stringify - Start")
	if len(sr.CustomRule.AND) > 0 {
		str, err = processRuleClause(ctx, sr.CustomRule.AND, "and", vars, ignoreIfNotFound)
		return
	}
	if len(sr.CustomRule.OR) > 0 {
		str, err = processRuleClause(ctx, sr.CustomRule.OR, "or", vars, ignoreIfNotFound)
		return
	}
	return
}
func processRuleClause(ctx context.Context, rules []CustomRuleDetails, conditionType string, vars map[string]interface{}, ignoreIfNotFound bool) (ruleOutput string, err error) {
	logs.WithContext(ctx).Debug("processRuleClause - Start")
	var strArray []string
	str := ""
	for _, v := range rules {
		if len(v.AND) > 0 {
			str, err = processRuleClause(ctx, v.AND, "and", vars, ignoreIfNotFound)
		} else if len(v.OR) > 0 {
			str, err = processRuleClause(ctx, v.OR, "or", vars, ignoreIfNotFound)
		} else {
			str, err = stringifyRule(ctx, v, conditionType, vars, ignoreIfNotFound)
		}
		if str != "" {
			strArray = append(strArray, str)
		}
	}
	if len(strArray) > 0 {
		conditionType = fmt.Sprint(" ", conditionType, " ")
		ruleOutput = fmt.Sprint("( ", strings.Join(strArray, conditionType), " )")
	}
	return
}

func stringifyRule(ctx context.Context, cd CustomRuleDetails, conditionType string, vars map[string]interface{}, ignoreIfNotFound bool) (str string, err error) {
	logs.WithContext(ctx).Debug("stringifyRule - Start")
	op := ""
	valPrefix := ""
	valSuffix := ""
	if cd.DataType == "string" {
		valPrefix = "'"
		valSuffix = "'"
	}
	if cd.Operator == "like" {
		valPrefix = "'%"
		valSuffix = "%'"
	}
	switch cd.Operator {
	case "btw":
		op = " between "
		break
	case "gte":
		op = " >= "
		break
	case "lte":
		op = " <= "
		break
	case "gt":
		op = " > "
		break
	case "lt":
		op = " < "
		break
	case "eq":
		op = " = "
		break
	case "ne":
		op = " <> "
		break
	case "in":
		op = " in "
		break
	case "nin":
		op = " not in "
		break
	case "like":
		op = " like "
		break
	case "eq_null":
		op = " is null "
		break
	case "neq_null":
		op = " is not null "
		break
	default:
		//do nothing
		break
	}
	var1Bytes, err := processTemplate(ctx, "customrule", cd.Variable1, vars, "string")
	if err == nil {
		cd.Variable1 = fmt.Sprint(valPrefix, string(var1Bytes), valSuffix)
	} else if ignoreIfNotFound && err.Error() != "no variable prefix found" {
		return "", nil
	}
	var2Bytes, err := processTemplate(ctx, "customrule", cd.Variable2, vars, "string")
	if err == nil {
		cd.Variable2 = fmt.Sprint(valPrefix, string(var2Bytes), valSuffix)
	} else if ignoreIfNotFound && err.Error() != "no variable prefix found" {
		return "", nil
	}
	return fmt.Sprint(cd.Variable1, op, cd.Variable2), nil
}

func processTemplate(ctx context.Context, templateName string, templateString string, vars map[string]interface{}, outputType string) (output []byte, err error) {
	logs.WithContext(ctx).Debug("processTemplate - Start")
	ruleValue := strings.SplitN(templateString, ".", 2)
	if ruleValue[0] == "token" {
		templateStr := fmt.Sprint("{{ .", ruleValue[1], " }}")
		goTmpl := gotemplate.GoTemplate{templateName, templateStr}
		outputObj, err := goTmpl.Execute(ctx, vars["token"], outputType)
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
	} else {
		err = errors.New("no variable prefix found")
		//logs.WithContext(ctx).Error(err.Error())
	}
	//todo - to add if prefix is not token
	return
}
