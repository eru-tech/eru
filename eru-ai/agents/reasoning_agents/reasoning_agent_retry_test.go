package reasoning_agents

import (
	"fmt"
	"strings"
	"testing"
)

// A schema rejection says nothing about whether the work behind the report
// happened. Without that spelled out, a model reads "send it again" as proof
// the side effects already landed and reports work it never did.
func TestValidationRetryPromptDisclaimsSideEffects(t *testing.T) {
	prompt := fmt.Sprintf(agentValidationRetryPrompt, "missing required top-level key(s) summary")

	for _, phrase := range []string{
		"SHAPE of your report",
		"Do NOT assume any tool call has already happened",
		"before reporting",
	} {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("the retry prompt no longer tells the model the rejection is about shape alone: missing %q", phrase)
		}
	}
}
