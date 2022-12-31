package security_rule

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	"log"
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

func (sr SecurityRule) Stringify(vars map[string]interface{}, ignoreIfNotFound bool) (str string, err error) {
	log.Print(vars)
	if len(sr.CustomRule.AND) > 0 {
		str, err = processRuleClause(sr.CustomRule.AND, "and", vars, ignoreIfNotFound)
		return
	}
	if len(sr.CustomRule.OR) > 0 {
		str, err = processRuleClause(sr.CustomRule.OR, "or", vars, ignoreIfNotFound)
		return
	}
	return
}
func processRuleClause(rules []CustomRuleDetails, conditionType string, vars map[string]interface{}, ignoreIfNotFound bool) (ruleOutput string, err error) {
	var strArray []string
	str := ""
	for _, v := range rules {
		if len(v.AND) > 0 {
			str, err = processRuleClause(v.AND, "and", vars, ignoreIfNotFound)
		} else if len(v.OR) > 0 {
			str, err = processRuleClause(v.OR, "or", vars, ignoreIfNotFound)
		} else {
			str, err = stringifyRule(v, conditionType, vars, ignoreIfNotFound)
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

func stringifyRule(cd CustomRuleDetails, conditionType string, vars map[string]interface{}, ignoreIfNotFound bool) (str string, err error) {
	op := ""
	valPrefix := ""
	valSuffix := ""
	if cd.DataType == "string" {
		valPrefix = "'"
		valSuffix = "'"
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

	var1Bytes, err := processTemplate("customrule", cd.Variable1, vars, "string")
	if err == nil {
		cd.Variable1 = fmt.Sprint(valPrefix, string(var1Bytes), valSuffix)
	} else if ignoreIfNotFound && err.Error() != "no variable prefix found" {
		return "", nil
	}
	var2Bytes, err := processTemplate("customrule", cd.Variable2, vars, "string")
	if err == nil {
		cd.Variable2 = fmt.Sprint(valPrefix, string(var2Bytes), valSuffix)
	} else if ignoreIfNotFound && err.Error() != "no variable prefix found" {
		return "", nil
	}
	log.Print(fmt.Sprint(cd.Variable1, op, cd.Variable2))
	return fmt.Sprint(cd.Variable1, op, cd.Variable2), nil
}

func processTemplate(templateName string, templateString string, vars map[string]interface{}, outputType string) (output []byte, err error) {

	ruleValue := strings.SplitN(templateString, ".", 2)
	if ruleValue[0] == "token" {
		templateStr := fmt.Sprint("{{ .", ruleValue[1], " }}")
		goTmpl := gotemplate.GoTemplate{templateName, templateStr}
		outputObj, err := goTmpl.Execute(vars["token"], outputType)
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
	} else {
		err = errors.New("no variable prefix found")
	}
	//todo - to add if prefix is not token
	return
}
