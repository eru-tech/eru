package eru

import (
	"context"
	"strings"
	"testing"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func init() { logs.LogInit("test", "processo-phase0a-test") }

func TestProcessoExposesThePhase0aActions(t *testing.T) {
	names := processoActionNames()
	for _, want := range []string{ProcessoRemoveEntity, ProcessoGetEntityFieldData} {
		if !names[want] {
			t.Errorf("processo does not expose %s", want)
		}
		// The tool's own actions are never tenant scoped.
		if names[want+"_default"] || names[want+"_project"] {
			t.Errorf("%s was tenant scoped, which changes its name", want)
		}
	}
}

// remove_entity destroys data, so its description has to say so where the model
// reads it rather than only in a prompt somewhere else.
func TestRemoveEntityWarnsThatItCannotBeUndone(t *testing.T) {
	for _, a := range processoToolActions {
		if a.ActionName != ProcessoRemoveEntity {
			continue
		}
		combined := strings.ToLower(a.Description + " " + a.SystemPrompt)
		if !strings.Contains(combined, "cannot be undone") {
			t.Errorf("remove_entity should warn it cannot be undone, got %q", combined)
		}
		return
	}
	t.Fatal("remove_entity action not found")
}

func TestGetEntityFieldDataValidatesItsInputs(t *testing.T) {
	tool := &ProcessoTool{}
	if _, _, _, err := tool.GetEntityFieldData(context.Background(), "processo", "t", map[string]interface{}{
		"org_id": "o", "process_id": "p", "process_name": "n", "entity_name": "deals",
	}); err == nil {
		t.Error("field_name is mandatory")
	}
}
