package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	models "github.com/eru-tech/eru/eru-ai/models"
)

func testPlan() map[string]interface{} {
	return map[string]interface{}{
		"func_category_name": "scf",
		"func_group_name":    "scf_outstanding_pie",
		"func_steps": map[string]interface{}{
			"processo_generate_sql": map[string]interface{}{
				"agent_name":        "processo_generate_sql",
				"tenant_id":         "processo",
				"transform_request": `{{stringify (dict "content" .Vars.Body.content)}}`,
				"func_steps": map[string]interface{}{
					"eruql_processo_execute_sql": map[string]interface{}{
						"tool_name":         "eruql_processo",
						"tool_action":       "execute_sql",
						"tenant_id":         "processo",
						"condition":         `{{if .Vars.Body.content}}true{{else}}false{{end}}`,
						"transform_request": `{{stringify (dict "params" (dict "query" "select 1"))}}`,
						"func_steps": map[string]interface{}{
							"eru_widget": map[string]interface{}{
								"agent_name":        "eru_widget",
								"tenant_id":         "processo",
								"wait_for":          "processo_generate_sql",
								"loop_variable":     `{{stringify .Vars.Body.rows}}`,
								"transform_request": `{{stringify (dict "content" "x")}}`,
							},
						},
					},
				},
			},
		},
	}
}

func TestSummarizePlanKeepsGraphDropsTemplates(t *testing.T) {
	summary, err := summarizePlan(testPlan())
	if err != nil {
		t.Fatal(err)
	}
	if summary.FuncGroupName != "scf_outstanding_pie" || summary.StepCount != 3 {
		t.Fatalf("unexpected summary head : %+v", summary)
	}
	if len(summary.Steps) != 1 || summary.Steps[0].Step != "processo_generate_sql" || summary.Steps[0].AgentName != "processo_generate_sql" {
		t.Fatalf("unexpected root step : %+v", summary.Steps)
	}
	toolStep := summary.Steps[0].Steps[0]
	if toolStep.ToolName != "eruql_processo" || toolStep.ToolAction != "execute_sql" || !toolStep.Conditional {
		t.Fatalf("unexpected tool step : %+v", toolStep)
	}
	widgetStep := toolStep.Steps[0]
	if widgetStep.AgentName != "eru_widget" || widgetStep.WaitFor != "processo_generate_sql" || !widgetStep.Loop {
		t.Fatalf("unexpected widget step : %+v", widgetStep)
	}

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"stringify", "transform_request", "tenant_id", "select 1"} {
		if strings.Contains(string(summaryJSON), leaked) {
			t.Errorf("summary leaked %q : %s", leaked, string(summaryJSON))
		}
	}
}

func TestClientTracesDropsStructuredOutputInput(t *testing.T) {
	traces := []models.StepTrace{
		{Iteration: 1, Thinking: "planning the steps"},
		{Iteration: 1, ToolName: "structured_output", ToolInput: map[string]interface{}{"func_group_name": "g"}},
		{Iteration: 2, ToolName: "ask_user", ToolInput: map[string]interface{}{"prompt": "which financier?"}},
	}

	sanitized := clientTraces(context.Background(), traces)
	if sanitized[1].ToolInput != nil {
		t.Errorf("expected structured_output tool_input dropped, got %v", sanitized[1].ToolInput)
	}
	if sanitized[1].ToolName != "structured_output" {
		t.Errorf("expected tool_name kept, got %q", sanitized[1].ToolName)
	}
	if sanitized[2].ToolInput == nil {
		t.Error("ask_user tool_input must be kept")
	}
	if sanitized[0].Thinking != "planning the steps" {
		t.Error("thinking must be kept")
	}
	if traces[1].ToolInput == nil {
		t.Error("original traces must not be mutated (conversation keeps the full plan)")
	}

	raw := clientTraces(agents.WithRawOutput(context.Background(), true), traces)
	if raw[1].ToolInput == nil {
		t.Error("raw output must keep structured_output tool_input")
	}
}
