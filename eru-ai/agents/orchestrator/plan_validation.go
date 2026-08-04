package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	functions "github.com/eru-tech/eru/eru-functions/functions"
	gotemplate "github.com/eru-tech/eru/eru-templates/gotemplate"
)

type planIssue struct {
	StepPath string
	Field    string
	Template string
	Err      string
}

func (pi planIssue) String() string {
	if pi.StepPath == "" {
		return fmt.Sprintf("%s: %s", pi.Field, pi.Err)
	}
	if pi.Template == "" {
		return fmt.Sprintf("step %s -> %s: %s", pi.StepPath, pi.Field, pi.Err)
	}
	return fmt.Sprintf("step %s -> %s: %s\n    template: %s", pi.StepPath, pi.Field, pi.Err, pi.Template)
}

func formatPlanIssues(issues []planIssue) string {
	var sb strings.Builder
	for i, issue := range issues {
		sb.WriteString(fmt.Sprint(i+1, ". ", issue.String(), "\n"))
	}
	return sb.String()
}

// validatePlanTemplates parses every go template in a generated FuncGroup with the
// same function map used at execution time, so malformed templates are caught
// before any step runs. It reports structural problems too, since a plan that
// cannot be decoded into a FuncGroup can never execute.
func validatePlanTemplates(ctx context.Context, plan map[string]interface{}) []planIssue {
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return []planIssue{{Field: "func_group", Err: err.Error()}}
	}
	var funcGroup functions.FuncGroup
	if err := json.Unmarshal(planJSON, &funcGroup); err != nil {
		return []planIssue{{Field: "func_group", Err: fmt.Sprint("plan is not a valid FuncGroup : ", err.Error())}}
	}
	return validateStepTemplates(ctx, "", funcGroup.FuncSteps)
}

func validateStepTemplates(ctx context.Context, parentPath string, steps map[string]*functions.FuncStep) []planIssue {
	var issues []planIssue
	stepKeys := make([]string, 0, len(steps))
	for stepKey := range steps {
		stepKeys = append(stepKeys, stepKey)
	}
	sort.Strings(stepKeys)
	for _, stepKey := range stepKeys {
		step := steps[stepKey]
		if step == nil {
			continue
		}
		stepPath := stepKey
		if parentPath != "" {
			stepPath = fmt.Sprint(parentPath, ".", stepKey)
		}
		templateFields := []struct {
			Name     string
			Template string
		}{
			{"transform_request", step.TransformRequest},
			{"transform_response", step.TransformResponse},
			{"condition", step.Condition},
			{"condition_fail_message", step.ConditionFailMessage},
			{"loop_variable", step.LoopVariable},
			{"async_message", step.AsyncMessage},
		}
		for _, templateField := range templateFields {
			if issue, ok := validateStepTemplate(ctx, stepPath, templateField.Name, templateField.Template); !ok {
				issues = append(issues, issue)
			}
		}
		headerFields := []struct {
			Name    string
			Headers []functions.Headers
		}{
			{"request_headers", step.RequestHeaders},
			{"query_params", step.QueryParams},
			{"form_data", step.FormData},
			{"response_headers", step.ResponseHeaders},
		}
		for _, headerField := range headerFields {
			for i, header := range headerField.Headers {
				if !header.IsTemplate {
					continue
				}
				fieldName := fmt.Sprint(headerField.Name, "[", i, "].value")
				if issue, ok := validateStepTemplate(ctx, stepPath, fieldName, header.Value); !ok {
					issues = append(issues, issue)
				}
			}
		}
		issues = append(issues, validateStepTemplates(ctx, stepPath, step.FuncSteps)...)
	}
	return issues
}

func validateStepTemplate(ctx context.Context, stepPath string, fieldName string, templateString string) (planIssue, bool) {
	if strings.TrimSpace(templateString) == "" {
		return planIssue{}, true
	}
	goTmpl := gotemplate.GoTemplate{Name: fmt.Sprint(stepPath, ".", fieldName), Template: templateString}
	if err := goTmpl.Validate(ctx); err != nil {
		return planIssue{StepPath: stepPath, Field: fieldName, Template: templateString, Err: err.Error()}, false
	}
	return planIssue{}, true
}
