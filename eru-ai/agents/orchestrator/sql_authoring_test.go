package orchestrator

import (
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	functions "github.com/eru-tech/eru/eru-functions/functions"
	eru_models "github.com/eru-tech/eru/eru-models"
)

func sqlCapableAgents() []agents.DiscoveredAgent {
	return []agents.DiscoveredAgent{
		{AgentName: "generate_sql", OutputSchema: eru_models.JSONSchema{
			Type:       "object",
			Properties: map[string]eru_models.JSONSchema{"sql": {Type: "string"}},
		}},
		{AgentName: "eru_studio"},
	}
}

// The run this guards against: the planner queried "eru.entity_fields", a table
// that does not exist, retried it until the run gave up, and the user was told
// only that something went wrong.
func TestValidateSqlAuthoringRejectsSqlTypedIntoThePlan(t *testing.T) {
	steps := map[string]*functions.FuncStep{
		"fetch_fields": {
			ToolName: "eruql_processo",
			TransformRequest: `{{stringify (dict "params" (dict "query" "SELECT ef.field_name FROM eru.entity_fields ef WHERE ef.entity_name ilike 'financier'"))}}`,
		},
	}
	issues := validateSqlAuthoring(steps, sqlCapableAgents())
	if len(issues) != 1 {
		t.Fatalf("expected the invented SQL to be reported, got %d issue(s)", len(issues))
	}
	if !strings.Contains(issues[0].Err, "generate_sql") {
		t.Errorf("the issue should name the agent that can write the SQL: %s", issues[0].Err)
	}
}

// SQL produced by an earlier step and passed on by reference is the supported
// shape and must not be flagged.
func TestValidateSqlAuthoringAllowsSqlPassedByReference(t *testing.T) {
	steps := map[string]*functions.FuncStep{
		"run_sql": {
			ToolName:         "eruql_processo",
			TransformRequest: `{{stringify (dict "params" (dict "query" (index .ResVars.generate_sql.Body.actions 0).action.sql))}}`,
		},
	}
	if issues := validateSqlAuthoring(steps, sqlCapableAgents()); len(issues) > 0 {
		t.Errorf("SQL taken from a previous step was reported: %v", issues)
	}
}

// With nothing able to write SQL from the schema, a literal is the only option
// there is, so complaining about it would be noise.
func TestValidateSqlAuthoringStaysQuietWithoutASqlAgent(t *testing.T) {
	steps := map[string]*functions.FuncStep{
		"fetch": {ToolName: "eruql_processo", TransformRequest: `{{stringify (dict "params" (dict "query" "SELECT a FROM b"))}}`},
	}
	if issues := validateSqlAuthoring(steps, []agents.DiscoveredAgent{{AgentName: "eru_studio"}}); len(issues) > 0 {
		t.Errorf("expected silence when no agent can author SQL, got %v", issues)
	}
}

// Prose that merely contains the word "select" is not a SQL statement.
func TestContainsLiteralSQLIgnoresProse(t *testing.T) {
	if containsLiteralSQL(`{{stringify (dict "content" "select a nice modern layout for the user")}}`) {
		t.Error("prose containing \"select\" was treated as SQL")
	}
	if !containsLiteralSQL(`... "UPDATE financiers SET name = 'x'" ...`) {
		t.Error("an UPDATE statement was not detected")
	}
}
