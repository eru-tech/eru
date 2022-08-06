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
	log.Print("inside Stringify")
	if len(sr.CustomRule.AND) > 0 {
		str, err = processRuleClause(sr.CustomRule.AND, "and", vars, ignoreIfNotFound)
		log.Print(err)
		log.Print(str)
		return
	}
	if len(sr.CustomRule.OR) > 0 {
		str, err = processRuleClause(sr.CustomRule.OR, "or", vars, ignoreIfNotFound)
		log.Print(err)
		log.Println(str)
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
			log.Print(str)
			log.Print(err)
		} else if len(v.OR) > 0 {
			str, err = processRuleClause(v.OR, "or", vars, ignoreIfNotFound)
			log.Print(str)
			log.Print(err)
		} else {
			str, err = stringifyRule(v, conditionType, vars, ignoreIfNotFound)
			log.Print(str)
			log.Print(err)
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
	log.Print("ignoreIfNotFound = ", ignoreIfNotFound)
	op := ""
	valPrefix := ""
	valSuffix := ""
	if cd.DataType == "string" {
		valPrefix = "'"
		valSuffix = "'"
	}
	switch cd.Operator {
	case "eq":
		op = " = "
		break
	case "neq":
		op = " <> "
		break
	default:
		//do nothing
		break
	}

	log.Print("cd.Variable1 = ", cd.Variable1)
	var1Bytes, err := processTemplate("customrule", cd.Variable1, vars, "string")
	log.Print("error fron var1 = ", err)
	if err == nil {
		cd.Variable1 = fmt.Sprint(valPrefix, string(var1Bytes), valSuffix)
	} else if ignoreIfNotFound && err.Error() != "no variable prefix found" {
		return "", nil
	}
	log.Print(cd.Variable1)
	log.Print("cd.Variable2 = ", cd.Variable2)
	var2Bytes, err := processTemplate("customrule", cd.Variable2, vars, "string")
	log.Print("error fron var2 = ", err)
	log.Print(string(var2Bytes))
	log.Print(cd.Variable2)
	if err == nil {
		cd.Variable2 = fmt.Sprint(valPrefix, string(var2Bytes), valSuffix)
	} else if ignoreIfNotFound && err.Error() != "no variable prefix found" {
		return "", nil
	}
	log.Print(cd.Variable2)

	return fmt.Sprint(cd.Variable1, op, cd.Variable2), nil
}

func processTemplate(templateName string, templateString string, vars map[string]interface{}, outputType string) (output []byte, err error) {
	log.Println("inside processTemplate")

	ruleValue := strings.SplitN(templateString, ".", 2)
	log.Print(ruleValue)
	if ruleValue[0] == "token" {
		log.Print("inside token check")
		templateStr := fmt.Sprint("{{ .", ruleValue[1], " }}")
		goTmpl := gotemplate.GoTemplate{templateName, templateStr}
		outputObj, err := goTmpl.Execute(vars["token"], outputType)
		log.Print(outputObj)
		if err != nil {
			log.Println(err)
			return nil, err
		} else if outputType == "string" {
			log.Print(outputObj.(string))
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
