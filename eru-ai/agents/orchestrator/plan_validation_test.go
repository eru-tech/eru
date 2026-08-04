package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func planWithTransformRequest(t *testing.T, transformRequest string) map[string]interface{} {
	t.Helper()
	plan := map[string]interface{}{
		"func_category_name": "scf",
		"func_group_name":    "scf_pie",
		"func_steps": map[string]interface{}{
			"eru_widget": map[string]interface{}{
				"agent_name":        "eru_widget",
				"tenant_id":         "processo",
				"transform_request": transformRequest,
			},
		},
	}
	if _, err := json.Marshal(plan); err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestValidatePlanTemplatesCatchesExtraBrace(t *testing.T) {
	broken := `{{stringify (dict "content" (printf "Display the following data as a pie chart showing total outstanding amount for each financier as of today:\n%s" (stringify .ResVars.eruql_processo_execute_sql.Body))))}}}`
	issues := validatePlanTemplates(context.Background(), planWithTransformRequest(t, broken))
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d : %v", len(issues), issues)
	}
	if issues[0].StepPath != "eru_widget" || issues[0].Field != "transform_request" {
		t.Errorf("unexpected issue location : %+v", issues[0])
	}
	if !strings.Contains(formatPlanIssues(issues), "transform_request") {
		t.Errorf("formatted issues missing field name : %s", formatPlanIssues(issues))
	}
}

func TestValidatePlanTemplatesAcceptsValidTemplate(t *testing.T) {
	valid := `{{stringify (dict "content" (printf "rows:\n%s" (stringify .ResVars.eruql_processo_execute_sql.Body)))}}`
	if issues := validatePlanTemplates(context.Background(), planWithTransformRequest(t, valid)); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestValidatePlanTemplatesNestedStepsAndUnknownFunc(t *testing.T) {
	plan := map[string]interface{}{
		"func_group_name": "g",
		"func_steps": map[string]interface{}{
			"generate_sql": map[string]interface{}{
				"agent_name":        "generate_sql",
				"transform_request": `{{stringify (dict "content" .Vars.Body.content)}}`,
				"func_steps": map[string]interface{}{
					"execute_sql": map[string]interface{}{
						"tool_name":         "eruql_processo",
						"tool_action":       "execute_sql",
						"transform_request": `{{jsonify (dict "params" (dict "query" "select 1"))}}`,
						"condition":         `{{if eq .Vars.Body.status "active"}}true{{else}}false{{end`,
					},
				},
			},
		},
	}
	issues := validatePlanTemplates(context.Background(), plan)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d : %v", len(issues), issues)
	}
	for _, issue := range issues {
		if issue.StepPath != "generate_sql.execute_sql" {
			t.Errorf("expected nested step path, got %q", issue.StepPath)
		}
	}
}

func TestValidatePlanTemplatesRejectsNonFuncGroup(t *testing.T) {
	plan := map[string]interface{}{"func_steps": "not-an-object"}
	issues := validatePlanTemplates(context.Background(), plan)
	if len(issues) != 1 || issues[0].Field != "func_group" {
		t.Fatalf("expected a func_group issue, got %v", issues)
	}
}
