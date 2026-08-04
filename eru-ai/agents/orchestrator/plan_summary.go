package orchestrator

import (
	"context"
	"encoding/json"
	"sort"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
	functions "github.com/eru-tech/eru/eru-functions/functions"
)

type planStepSummary struct {
	Step        string            `json:"step"`
	AgentName   string            `json:"agent_name,omitempty"`
	ToolName    string            `json:"tool_name,omitempty"`
	ToolAction  string            `json:"tool_action,omitempty"`
	WaitFor     string            `json:"wait_for,omitempty"`
	Async       bool              `json:"async,omitempty"`
	Loop        bool              `json:"loop,omitempty"`
	Conditional bool              `json:"conditional,omitempty"`
	Steps       []planStepSummary `json:"steps,omitempty"`
}

type planSummary struct {
	FuncCategoryName string            `json:"func_category_name,omitempty"`
	FuncGroupName    string            `json:"func_group_name,omitempty"`
	StepCount        int               `json:"step_count"`
	Steps            []planStepSummary `json:"steps"`
}

// summarizePlan reduces a generated FuncGroup to the step graph the client needs
// for progress display: what runs, in what order, and whether siblings run in
// parallel. Templates, tenant ids and conditions are deliberately left out —
// they are internal orchestration detail. Built in code, no model call.
func summarizePlan(plan map[string]interface{}) (planSummary, error) {
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return planSummary{}, err
	}
	var funcGroup functions.FuncGroup
	if err := json.Unmarshal(planJSON, &funcGroup); err != nil {
		return planSummary{}, err
	}
	steps := summarizeSteps(funcGroup.FuncSteps)
	return planSummary{
		FuncCategoryName: funcGroup.FuncCategoryName,
		FuncGroupName:    funcGroup.FuncGroupName,
		StepCount:        countSteps(steps),
		Steps:            steps,
	}, nil
}

func summarizeSteps(steps map[string]*functions.FuncStep) []planStepSummary {
	stepKeys := make([]string, 0, len(steps))
	for stepKey := range steps {
		stepKeys = append(stepKeys, stepKey)
	}
	sort.Strings(stepKeys)
	summaries := make([]planStepSummary, 0, len(stepKeys))
	for _, stepKey := range stepKeys {
		step := steps[stepKey]
		if step == nil {
			continue
		}
		summaries = append(summaries, planStepSummary{
			Step:        stepKey,
			AgentName:   step.AgentName,
			ToolName:    step.ToolName,
			ToolAction:  step.ToolAction,
			WaitFor:     step.WaitFor,
			Async:       step.Async,
			Loop:        step.LoopVariable != "",
			Conditional: step.Condition != "",
			Steps:       summarizeSteps(step.FuncSteps),
		})
	}
	return summaries
}

func countSteps(steps []planStepSummary) int {
	count := 0
	for _, step := range steps {
		count = count + 1 + countSteps(step.Steps)
	}
	return count
}

// clientTraces returns the traces to hand back to the caller. The
// structured_output tool input carries the whole FuncGroup (and, for sub-agents,
// their full structured answer, which is already delivered as an action), so it
// is dropped unless raw output was requested. The traces saved to the
// conversation keep everything.
func clientTraces(ctx context.Context, traces []models.StepTrace) []models.StepTrace {
	if len(traces) == 0 || agents.RawOutputEnabled(ctx) {
		return traces
	}
	sanitized := make([]models.StepTrace, len(traces))
	copy(sanitized, traces)
	for i := range sanitized {
		if sanitized[i].ToolName == models.TerminalToolStructuredOutput {
			sanitized[i].ToolInput = nil
		}
	}
	return sanitized
}
