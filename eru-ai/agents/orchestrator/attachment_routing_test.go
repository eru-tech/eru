package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	functions "github.com/eru-tech/eru/eru-functions/functions"
)

func funcGroupFrom(t *testing.T, raw string) functions.FuncGroup {
	t.Helper()
	var fg functions.FuncGroup
	if err := json.Unmarshal([]byte(raw), &fg); err != nil {
		t.Fatalf("bad plan fixture: %v", err)
	}
	return fg
}

const planWithoutFiles = `{"func_steps":{"build":{"agent_name":"eru_studio",
  "transform_request":"{{stringify (dict \"content\" .Vars.OrgBody.content)}}"}}}`

const planWithFiles = `{"func_steps":{"build":{"agent_name":"eru_studio",
  "transform_request":"{{stringify (dict \"content\" .Vars.OrgBody.content \"files\" .Vars.OrgBody.files)}}"}}}`

func TestAnAttachmentNoStepReceivesIsAPlanDefect(t *testing.T) {
	issues := validateAttachmentRouting(funcGroupFrom(t, planWithoutFiles), codeContext{
		Attachments: []string{"mockup.png"},
	})
	if len(issues) != 1 {
		t.Fatalf("a dropped attachment was not reported: %+v", issues)
	}
	for _, want := range []string{"mockup.png", "user.files", "top-level key"} {
		if !strings.Contains(issues[0].Err, want) {
			t.Fatalf("the plan issue does not mention %q:\n%s", want, issues[0].Err)
		}
	}
}

func TestAPlanThatForwardsTheAttachmentIsFine(t *testing.T) {
	issues := validateAttachmentRouting(funcGroupFrom(t, planWithFiles), codeContext{
		Attachments: []string{"mockup.png"},
	})
	if len(issues) != 0 {
		t.Fatalf("a plan that forwards the attachment was faulted: %+v", issues)
	}
}

func TestNothingIsRequiredWhenNothingWasAttached(t *testing.T) {
	// The overwhelming majority of requests carry no file, and this rule must be
	// invisible to them.
	issues := validateAttachmentRouting(funcGroupFrom(t, planWithoutFiles), codeContext{})
	if len(issues) != 0 {
		t.Fatalf("faulted a plan for not forwarding attachments that do not exist: %+v", issues)
	}
}

func TestOneOfSeveralStepsIsEnough(t *testing.T) {
	plan := funcGroupFrom(t, `{"func_steps":{
	  "lookup":{"agent_name":"processo_sql","transform_request":"{{stringify (dict \"content\" \"find the entity\")}}"},
	  "build":{"agent_name":"eru_studio","transform_request":"{{stringify (dict \"content\" .Vars.OrgBody.content \"files\" .Vars.OrgBody.files)}}"}}}`)
	if issues := validateAttachmentRouting(plan, codeContext{Attachments: []string{"a.png"}}); len(issues) != 0 {
		t.Fatalf("every step was expected to carry the file, when one is the point: %+v", issues)
	}
}

func TestTheMessageNamesEveryAttachment(t *testing.T) {
	issues := validateAttachmentRouting(funcGroupFrom(t, planWithoutFiles), codeContext{
		Attachments: []string{"front.png", "back.png"},
	})
	if len(issues) != 1 {
		t.Fatalf("expected one issue, got %d", len(issues))
	}
	for _, want := range []string{"front.png", "back.png", "them"} {
		if !strings.Contains(issues[0].Err, want) {
			t.Fatalf("issue does not mention %q:\n%s", want, issues[0].Err)
		}
	}
}
