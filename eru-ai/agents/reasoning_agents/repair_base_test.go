package reasoning_agents

import (
	"context"
	"testing"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
)

// chartPage is one line_chart whose query_result_path is the wrong shape - the
// fault the repair turn exists to correct.
func repairChartPage(resultPath string) map[string]interface{} {
	return map[string]interface{}{
		"id": "p1", "name": "dash", "styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id": "trend", "type": "line_chart", "styles": map[string]interface{}{},
				"properties": map[string]interface{}{
					"base": map[string]interface{}{
						"query":             "db_disb",
						"query_result_path": resultPath,
					},
				},
			},
		},
	}
}

func hasResultPathIssue(issues []catalog.Issue) bool {
	for _, issue := range issues {
		if issue.Code == catalog.CodeQueryResultPathWrong {
			return true
		}
	}
	return false
}

// A repair that hands back the same fault must still be rejected. The filter
// that forgives pre-existing faults is there for the user's page, and during a
// repair the base is the agent's own rejected answer.
func TestARepairDoesNotForgiveTheFaultItWasCalledToFix(t *testing.T) {
	rejected := repairChartPage("result.0.Results")

	state := studio.NewRepairState()
	state.Begin(rejected, nil)
	ctx := studio.WithRepairState(context.Background(), state)
	ctx = studio.WithOutputMode(ctx, studio.ModeAuto)
	ctx = studio.WithLedger(ctx, studio.NewLedger())

	answer := map[string]interface{}{"mode": studio.ModeFull, "page": repairChartPage("result.0.Results")}
	_, issues := eruStudioPageIssues(ctx, answer)
	if !hasResultPathIssue(issues) {
		t.Fatal("a repair that reproduces the rejected fault must still be reported")
	}
}

// The forgiveness itself must survive: a fault the USER's own page already had
// is not the agent's to fix, and reporting it turns an edit into a rewrite.
func TestAFaultInTheUsersOwnPageIsStillForgiven(t *testing.T) {
	usersPage := repairChartPage("result.0.Results")

	ctx := studio.WithBasePage(context.Background(), usersPage)
	ctx = studio.WithOutputMode(ctx, studio.ModeAuto)
	ctx = studio.WithLedger(ctx, studio.NewLedger())

	answer := map[string]interface{}{"mode": studio.ModeFull, "page": repairChartPage("result.0.Results")}
	_, issues := eruStudioPageIssues(ctx, answer)
	if hasResultPathIssue(issues) {
		t.Fatal("a fault the user's page already carried must not be raised against the agent")
	}
}
