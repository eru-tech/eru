package orchestrator

import (
	"context"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	eru_models "github.com/eru-tech/eru/eru-models"
)

func TestDescribeCodeParamAbsentOrEmpty(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]interface{}
	}{
		{"nil params", nil},
		{"no code key", map[string]interface{}{"context": "rows"}},
		{"blank code", map[string]interface{}{"code": "   "}},
		{"empty object", map[string]interface{}{"code": "{}"}},
		{"null", map[string]interface{}{"code": "null"}},
	}
	for _, tc := range cases {
		if cc := describeCodeParam(tc.params); cc.Present {
			t.Errorf("%s : expected code to be treated as absent, got %+v", tc.name, cc)
		}
	}
}

func TestDescribeCodeParamClassifiesArtifacts(t *testing.T) {
	cases := []struct {
		name     string
		code     interface{}
		wantKind string
		wantKeys []string
	}{
		{"eru page", `{"id":"p1","name":"page","components":[{"id":"c1"}]}`, "eru_page_json", []string{"components", "id", "name"}},
		{"func group", `{"func_group_name":"g","func_steps":{}}`, "func_group_json", []string{"func_group_name", "func_steps"}},
		{"sql", "  SELECT a, b FROM t WHERE x = 1", "sql", nil},
		{"go template", `{{stringify (dict "content" .Vars.Body.content)}}`, "go_template", nil},
		{"json object", map[string]interface{}{"sql": "select 1"}, "json_object", []string{"sql"}},
		{"json array", `[{"a":1}]`, "json_array", nil},
		{"text", "just some notes", "text", nil},
	}
	for _, tc := range cases {
		cc := describeCodeParam(map[string]interface{}{"code": tc.code})
		if !cc.Present {
			t.Fatalf("%s : expected code to be present", tc.name)
		}
		if cc.Kind != tc.wantKind {
			t.Errorf("%s : expected kind %q, got %q", tc.name, tc.wantKind, cc.Kind)
		}
		if strings.Join(cc.TopKeys, ",") != strings.Join(tc.wantKeys, ",") {
			t.Errorf("%s : expected top keys %v, got %v", tc.name, tc.wantKeys, cc.TopKeys)
		}
	}
}

func TestDescribeCodeParamTruncatesPreview(t *testing.T) {
	code := "SELECT " + strings.Repeat("a", codePreviewLimit*2)
	cc := describeCodeParam(map[string]interface{}{"code": code})
	if !cc.Truncated {
		t.Fatal("expected preview to be truncated")
	}
	if len(cc.Preview) != codePreviewLimit {
		t.Errorf("expected preview of %d chars, got %d", codePreviewLimit, len(cc.Preview))
	}
	if cc.Size != len(code) {
		t.Errorf("expected size %d, got %d", len(code), cc.Size)
	}
}

func TestPromptSectionOnlyWhenPresent(t *testing.T) {
	if section := (codeContext{}).promptSection(nil); section != "" {
		t.Fatalf("expected no prompt section when code is absent, got %q", section)
	}
	cc := describeCodeParam(map[string]interface{}{"code": `{"id":"p1","components":[]}`})
	section := cc.promptSection(nil)
	for _, want := range []string{"RULE #2c", "eru_page_json", ".Vars.Body.params.code", "Do NOT pass it to steps"} {
		if !strings.Contains(section, want) {
			t.Errorf("prompt section missing %q :\n%s", want, section)
		}
	}
}

func TestPromptSectionHintsCandidateAgents(t *testing.T) {
	discovered := []agents.DiscoveredAgent{
		{AgentName: "generate_sql", OutputSchema: eru_models.JSONSchema{Properties: map[string]eru_models.JSONSchema{"sql": {Type: "string"}}}},
		{AgentName: "summarizer", OutputSchema: eru_models.JSONSchema{Properties: map[string]eru_models.JSONSchema{"summary": {Type: "string"}}}},
	}
	cc := describeCodeParam(map[string]interface{}{"code": "select 1 from t"})
	section := cc.promptSection(discovered)
	if !strings.Contains(section, "generate_sql") {
		t.Errorf("expected generate_sql to be hinted :\n%s", section)
	}
	if strings.Contains(section, "summarizer") {
		t.Errorf("summarizer should not be hinted for a sql artifact :\n%s", section)
	}
}

func TestValidateCodeRoutingRejectsCodeRefWithoutCode(t *testing.T) {
	plan := planWithTransformRequest(t, `{{stringify (dict "content" .Vars.Body.content "params" (dict "code" .Vars.Body.params.code))}}`)
	issues := validatePlan(context.Background(), plan, nil, nil, codeContext{})
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d : %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Err, "carried no code artifact") {
		t.Errorf("unexpected issue : %+v", issues[0])
	}
}

func TestValidateCodeRoutingAcceptsCodeRefWhenCodePresent(t *testing.T) {
	cc := describeCodeParam(map[string]interface{}{"code": `{"id":"p1","components":[{"id":"c1"}]}`})
	plan := planWithTransformRequest(t, `{{stringify (dict "content" .Vars.Body.content "params" (dict "code" .Vars.Body.params.code))}}`)
	if issues := validatePlan(context.Background(), plan, nil, nil, cc); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestValidateCodeRoutingRejectsPastedArtifact(t *testing.T) {
	code := `{"id":"page-1","name":"outstanding","components":[{"id":"table-1","type":"table","properties":{"columns":["financier","amount"]}}]}`
	cc := describeCodeParam(map[string]interface{}{"code": code})
	pasted := `{{stringify (dict "content" .Vars.Body.content "params" (dict "code" ` + "`" + code + "`" + `))}}`
	issues := validatePlan(context.Background(), planWithTransformRequest(t, pasted), nil, nil, cc)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d : %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Err, "pasted into this template") {
		t.Errorf("unexpected issue : %+v", issues[0])
	}
	if issues[0].Template != "" {
		t.Errorf("expected the pasted template to be omitted from the issue")
	}
}

func TestCodeRoutedSteps(t *testing.T) {
	plan := map[string]interface{}{
		"func_group_name": "g",
		"func_steps": map[string]interface{}{
			"generate_sql": map[string]interface{}{
				"agent_name":        "generate_sql",
				"transform_request": `{{stringify (dict "content" .Vars.Body.content "params" (dict "code" .Vars.Body.params.code))}}`,
				"func_steps": map[string]interface{}{
					"summarizer": map[string]interface{}{
						"agent_name":        "summarizer",
						"transform_request": `{{stringify (dict "content" (index .ResVars.generate_sql.Body.actions 0).action.sql)}}`,
					},
				},
			},
		},
	}
	routed := codeRoutedSteps(context.Background(), plan)
	if len(routed) != 1 || routed[0] != "generate_sql" {
		t.Fatalf("expected only generate_sql to receive the artifact, got %v", routed)
	}
}
