package orchestrator

import (
	"context"
	"strings"
	"testing"
)

// The orchestrator prompt shipped with "{{GUIDELINES_PLACEHOLDER}}" and
// "{{EXAMPLES_PLACEHOLDER}}" that nothing ever filled in, and the model
// eventually answered a question with one of them, verbatim.
func TestThePlanningPromptCarriesNoPlaceholders(t *testing.T) {
	oa := &OrchestratorAgent{}
	prompt := oa.planningSystemPrompt(codeContext{}, false, "p", "t")
	whole := prompt.Static + prompt.Dynamic
	if found := placeholderPattern.FindAllString(whole, 5); len(found) > 0 {
		t.Errorf("the prompt still names holes nobody fills: %s", strings.Join(found, ", "))
	}
}

func TestPlaceholderPatternMatchesTheMarkersWeUse(t *testing.T) {
	for _, marker := range []string{"{{GUIDELINES_PLACEHOLDER}}", "{{COMPONENT_LIBRARY}}", "{{NESTED_PAGES}}"} {
		if !placeholderPattern.MatchString(marker) {
			t.Errorf("%s should be recognised as a placeholder", marker)
		}
	}
	// Go template actions and ordinary prose are not placeholders.
	for _, notMarker := range []string{"{{stringify .Vars.Body.content}}", "a {{ b }} c", "plain text"} {
		if placeholderPattern.MatchString(notMarker) {
			t.Errorf("%q should not be taken for a placeholder", notMarker)
		}
	}
}

func TestReportingAPlaceholderDoesNotPanic(t *testing.T) {
	reportUnsubstitutedPlaceholders(context.Background(), "test prompt", "hello {{SOMETHING}} world")
	reportUnsubstitutedPlaceholders(context.Background(), "test prompt", "nothing to see")
}
