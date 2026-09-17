package orchestrator

import (
	"context"
	"strings"
	"testing"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	utility "github.com/eru-tech/eru/eru-ai/tools/utility"
)

// Nothing backs the lookups, so nothing is offered and the prompt says nothing
// about them.
func TestNoResearchIsOfferedWithoutAnEruqlTool(t *testing.T) {
	oa := &OrchestratorAgent{}
	research := oa.researchTools(context.Background())
	if len(research) != 0 {
		t.Fatalf("expected no research tools, got %v", research)
	}
	if researchGuidance(research) != "" {
		t.Error("a planner should not be told about a lookup it cannot make")
	}
}

func TestResearchGuidanceAppearsOnlyWithTheLookup(t *testing.T) {
	withTool := map[string]tools.Tooling{utility.EntityMetadataToolName: &utility.EntityMetadataTool{}}
	guidance := researchGuidance(withTool)
	if !strings.Contains(guidance, utility.EntityMetadataToolName) {
		t.Error("guidance should name the lookup")
	}
	for _, want := range []string{"BEFORE you produce the plan", "Do not do the work here", "looks up for itself"} {
		if !strings.Contains(guidance, want) {
			t.Errorf("guidance is missing %q", want)
		}
	}
}

// A tool the planner is not offered is still refused, so research cannot become
// a way to run arbitrary tools during planning.
func TestAnUnofferedToolIsNotHandled(t *testing.T) {
	research := map[string]tools.Tooling{utility.EntityMetadataToolName: &utility.EntityMetadataTool{}}
	if _, _, handled := executeResearchTool(context.Background(), research, "save_query", "p", "t", nil); handled {
		t.Error("a tool outside the research set must not be executed during planning")
	}
}

func TestAnOfferedToolIsHandled(t *testing.T) {
	research := map[string]tools.Tooling{utility.EntityMetadataToolName: &utility.EntityMetadataTool{}}
	// No delegate is wired, so it errors - but it must be HANDLED, which is what
	// tells the loop this is a lookup rather than an unknown tool.
	_, err, handled := executeResearchTool(context.Background(), research, utility.EntityMetadataToolName, "p", "t", nil)
	if !handled {
		t.Fatal("the metadata lookup should be handled during planning")
	}
	if err == nil {
		t.Error("with no delegate the lookup should report why it cannot run")
	}
}
