package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	agents "github.com/eru-tech/eru/eru-ai/agents"
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
	return validatePlan(ctx, plan, nil, nil, codeContext{})
}

// validatePlan checks a generated FuncGroup before any step runs: every go
// template is parsed with the function map used at execution time, every
// .ResVars/.ReqVars reference is resolved against the plan's real step keys, and
// step keys, agent names and tool actions are checked against the naming rules
// and the agents/tools the orchestrator is actually allowed to use.
func validatePlan(ctx context.Context, plan map[string]interface{}, allowedAgents []agents.DiscoveredAgent, allowedTools []agents.DiscoveredTool, cc codeContext) []planIssue {
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return []planIssue{{Field: "func_group", Err: err.Error()}}
	}
	var funcGroup functions.FuncGroup
	if err := json.Unmarshal(planJSON, &funcGroup); err != nil {
		return []planIssue{{Field: "func_group", Err: fmt.Sprint("plan is not a valid FuncGroup : ", err.Error())}}
	}
	if len(funcGroup.FuncSteps) == 0 {
		return []planIssue{{Field: "func_steps", Err: "plan has no func_steps - the FuncGroup must contain at least one agent or tool step at the root of func_steps"}}
	}

	issues := validateStepTemplates(ctx, "", funcGroup.FuncSteps)
	issues = append(issues, validateStepReferences(ctx, funcGroup.FuncSteps)...)
	issues = append(issues, validateStepIdentity(funcGroup.FuncSteps, allowedAgents, allowedTools)...)
	issues = append(issues, validateCodeRouting(ctx, funcGroup.FuncSteps, cc)...)
	return issues
}

// validateCodeRouting checks how the plan handled the caller's existing
// structured output (params.code): a step may not read an artifact that was
// never sent, and the artifact must be passed by reference rather than pasted
// into a template, which bloats the plan and breaks on quoting.
func validateCodeRouting(ctx context.Context, steps map[string]*functions.FuncStep, cc codeContext) []planIssue {
	fingerprint := cc.fingerprint()
	var issues []planIssue
	walkSteps(steps, "", func(stepPath string, stepKey string, step *functions.FuncStep) {
		for _, templateField := range stepTemplateFields(step) {
			if strings.TrimSpace(templateField.Template) == "" {
				continue
			}
			if !cc.Present && templateReadsCodeParam(ctx, stepPath, templateField) {
				issues = append(issues, planIssue{
					StepPath: stepPath,
					Field:    templateField.Name,
					Template: templateField.Template,
					Err:      "reads .Vars.Body.params.code but the request carried no code artifact - remove the code key from this step's params",
				})
			}
			if fingerprint != "" && strings.Contains(normaliseForFingerprint(templateField.Template), fingerprint) {
				issues = append(issues, planIssue{
					StepPath: stepPath,
					Field:    templateField.Name,
					Err:      "the existing artifact from params.code is pasted into this template - pass it by reference instead, as (dict \"code\" .Vars.Body.params.code), and remove the pasted copy",
				})
			}
		}
	})
	return issues
}

// templateReadsCodeParam reports whether a template feeds the caller's
// params.code artifact into its step.
func templateReadsCodeParam(ctx context.Context, stepPath string, templateField stepTemplateField) bool {
	goTmpl := gotemplate.GoTemplate{Name: fmt.Sprint(stepPath, ".", templateField.Name), Template: templateField.Template}
	refs, err := goTmpl.FieldReferences(ctx)
	if err != nil {
		return strings.Contains(templateField.Template, fmt.Sprint("params.", codeParamKey))
	}
	for _, ref := range refs {
		if len(ref) >= 4 && ref[0] == "Vars" && ref[1] == "Body" && ref[2] == "params" && ref[3] == codeParamKey {
			return true
		}
	}
	return false
}

// codeRoutedSteps lists the steps that receive the params.code artifact, so the
// routing decision the planner made is visible in the logs.
func codeRoutedSteps(ctx context.Context, plan map[string]interface{}) []string {
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return nil
	}
	var funcGroup functions.FuncGroup
	if err := json.Unmarshal(planJSON, &funcGroup); err != nil {
		return nil
	}
	var routed []string
	walkSteps(funcGroup.FuncSteps, "", func(stepPath string, stepKey string, step *functions.FuncStep) {
		for _, templateField := range stepTemplateFields(step) {
			if strings.TrimSpace(templateField.Template) == "" {
				continue
			}
			if templateReadsCodeParam(ctx, stepPath, templateField) {
				routed = append(routed, stepPath)
				return
			}
		}
	})
	return routed
}

// validateStepReferences rejects a template that reads .ResVars/.ReqVars of a
// step that does not exist in the plan. A missing key renders as nil rather than
// failing, so without this check the step silently receives null data.
func validateStepReferences(ctx context.Context, steps map[string]*functions.FuncStep) []planIssue {
	stepKeys := make(map[string]bool)
	collectStepKeys(steps, stepKeys)

	var issues []planIssue
	walkSteps(steps, "", func(stepPath string, stepKey string, step *functions.FuncStep) {
		for _, templateField := range stepTemplateFields(step) {
			if strings.TrimSpace(templateField.Template) == "" {
				continue
			}
			goTmpl := gotemplate.GoTemplate{Name: fmt.Sprint(stepPath, ".", templateField.Name), Template: templateField.Template}
			refs, err := goTmpl.FieldReferences(ctx)
			if err != nil {
				continue
			}
			reported := make(map[string]bool)
			for _, ref := range refs {
				if len(ref) < 2 {
					continue
				}
				if ref[0] != "ResVars" && ref[0] != "ReqVars" {
					continue
				}
				if stepKeys[ref[1]] || reported[ref[1]] {
					continue
				}
				reported[ref[1]] = true
				issues = append(issues, planIssue{
					StepPath: stepPath,
					Field:    templateField.Name,
					Template: templateField.Template,
					Err: fmt.Sprint(".", ref[0], ".", ref[1], " does not exist - there is no step named \"", ref[1],
						"\" in this plan. Valid step keys are: ", strings.Join(sortedKeys(stepKeys), ", "),
						". Reference the step by its exact func_steps key"),
				})
			}
		}
	})
	return issues
}

// validateStepIdentity enforces the step-key naming rule and that every step
// uses an agent or tool the orchestrator is allowed to delegate to.
func validateStepIdentity(steps map[string]*functions.FuncStep, allowedAgents []agents.DiscoveredAgent, allowedTools []agents.DiscoveredTool) []planIssue {
	agentNames := make(map[string]bool)
	for _, discovered := range allowedAgents {
		agentNames[discovered.AgentName] = true
	}
	toolActions := make(map[string][]string)
	for _, discovered := range allowedTools {
		toolActions[discovered.ToolName] = append(toolActions[discovered.ToolName], discovered.ActionName)
	}

	var issues []planIssue
	walkSteps(steps, "", func(stepPath string, stepKey string, step *functions.FuncStep) {
		switch {
		case step.AgentName != "":
			if step.ToolName != "" {
				issues = append(issues, planIssue{StepPath: stepPath, Field: "agent_name", Err: fmt.Sprint("step sets both agent_name (", step.AgentName, ") and tool_name (", step.ToolName, ") - a step is either an agent step or a tool step")})
			}
			if !stepKeyMatches(stepKey, step.AgentName) {
				issues = append(issues, planIssue{StepPath: stepPath, Field: "agent_name", Err: fmt.Sprint("func_step key \"", stepKey, "\" must equal agent_name \"", step.AgentName, "\" (a numeric suffix is allowed for duplicates, e.g. \"", step.AgentName, "2\")")})
			}
			if len(agentNames) > 0 && !agentNames[step.AgentName] {
				issues = append(issues, planIssue{StepPath: stepPath, Field: "agent_name", Err: fmt.Sprint("agent \"", step.AgentName, "\" is not available - use one of: ", strings.Join(sortedKeys(agentNames), ", "))})
			}
		case step.ToolName != "":
			if step.ToolAction == "" {
				issues = append(issues, planIssue{StepPath: stepPath, Field: "tool_action", Err: fmt.Sprint("tool step \"", stepKey, "\" is missing tool_action")})
			}
			if !stepKeyMatches(stepKey, step.ToolName) && !stepKeyMatches(stepKey, fmt.Sprint(step.ToolName, "_", step.ToolAction)) {
				issues = append(issues, planIssue{StepPath: stepPath, Field: "tool_name", Err: fmt.Sprint("func_step key \"", stepKey, "\" must equal \"", step.ToolName, "\" or \"", step.ToolName, "_", step.ToolAction, "\"")})
			}
			if len(toolActions) > 0 {
				actions, found := toolActions[step.ToolName]
				if !found {
					issues = append(issues, planIssue{StepPath: stepPath, Field: "tool_name", Err: fmt.Sprint("tool \"", step.ToolName, "\" is not available - use one of: ", strings.Join(sortedKeys(toolActionNames(toolActions)), ", "))})
				} else if step.ToolAction != "" && !containsString(actions, step.ToolAction) {
					issues = append(issues, planIssue{StepPath: stepPath, Field: "tool_action", Err: fmt.Sprint("action \"", step.ToolAction, "\" is not allowed on tool \"", step.ToolName, "\" - allowed actions: ", strings.Join(actions, ", "))})
				}
			}
		default:
			issues = append(issues, planIssue{StepPath: stepPath, Field: "step", Err: "step has neither agent_name nor tool_name - the orchestrator may only delegate to agents and tools"})
		}
	})
	return issues
}

// stepKeyMatches allows the documented numeric suffix for duplicate steps at the
// same level ("classifier", "classifier2").
func stepKeyMatches(stepKey string, name string) bool {
	if name == "" {
		return false
	}
	if stepKey == name {
		return true
	}
	if !strings.HasPrefix(stepKey, name) {
		return false
	}
	suffix := strings.TrimPrefix(stepKey, name)
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return false
		}
	}
	return suffix != ""
}

type stepTemplateField struct {
	Name     string
	Template string
}

func stepTemplateFields(step *functions.FuncStep) []stepTemplateField {
	fields := []stepTemplateField{
		{"transform_request", step.TransformRequest},
		{"transform_response", step.TransformResponse},
		{"condition", step.Condition},
		{"condition_fail_message", step.ConditionFailMessage},
		{"loop_variable", step.LoopVariable},
		{"async_message", step.AsyncMessage},
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
			fields = append(fields, stepTemplateField{fmt.Sprint(headerField.Name, "[", i, "].value"), header.Value})
		}
	}
	return fields
}

func collectStepKeys(steps map[string]*functions.FuncStep, keys map[string]bool) {
	for stepKey, step := range steps {
		keys[stepKey] = true
		if step != nil {
			collectStepKeys(step.FuncSteps, keys)
		}
	}
}

func walkSteps(steps map[string]*functions.FuncStep, parentPath string, visit func(stepPath string, stepKey string, step *functions.FuncStep)) {
	for _, stepKey := range sortedStepKeys(steps) {
		step := steps[stepKey]
		if step == nil {
			continue
		}
		stepPath := stepKey
		if parentPath != "" {
			stepPath = fmt.Sprint(parentPath, ".", stepKey)
		}
		visit(stepPath, stepKey, step)
		walkSteps(step.FuncSteps, stepPath, visit)
	}
}

func sortedStepKeys(steps map[string]*functions.FuncStep) []string {
	stepKeys := make([]string, 0, len(steps))
	for stepKey := range steps {
		stepKeys = append(stepKeys, stepKey)
	}
	sort.Strings(stepKeys)
	return stepKeys
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func toolActionNames(toolActions map[string][]string) map[string]bool {
	names := make(map[string]bool)
	for toolName := range toolActions {
		names[toolName] = true
	}
	return names
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func validateStepTemplates(ctx context.Context, parentPath string, steps map[string]*functions.FuncStep) []planIssue {
	var issues []planIssue
	walkSteps(steps, parentPath, func(stepPath string, stepKey string, step *functions.FuncStep) {
		for _, templateField := range stepTemplateFields(step) {
			if issue, ok := validateStepTemplate(ctx, stepPath, templateField.Name, templateField.Template); !ok {
				issues = append(issues, issue)
			}
		}
	})
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
