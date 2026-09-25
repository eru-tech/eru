package agents

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Agent.UnmarshalJSON is the only door a configuration comes through, and it is
// an explicit allow-list. A field missing from that list is not a compile error
// and not a test failure - it is silently dropped. The struct field exists, the
// loop reads it, every Go test that builds an Agent directly passes, and no
// config file can ever set it.
//
// Five fields spent a day in exactly that state: validation_rules, evidence,
// claims, budget and max_delegation_depth were added, unit-tested, and
// documented as "reachable from configuration" while being unreachable. It was
// found by writing one real agent config, not by any test.
//
// This is that test.

// TestEveryConfigFieldSurvivesTheDoor builds a JSON object with every
// json-tagged field on Agent set to a non-zero value, sends it through
// UnmarshalJSON, and fails on anything that comes back empty.
func TestEveryConfigFieldSurvivesTheDoor(t *testing.T) {
	// Fields that legitimately do not arrive as JSON: they are resolved from
	// other config (a model by name, a store by type) or are runtime-only.
	resolvedElsewhere := map[string]bool{
		"-":                   true, // json:"-"
		"chat_memory":         true, // decoded separately, further down UnmarshalJSON
		"conversation_config": true, // decoded separately
		"function":            true, // covered, but a zero FuncGroup is legitimate
	}

	config := map[string]interface{}{
		"agent_type":           "REASONING",
		"agent_name":           "door_test",
		"is_system":            true,
		"description":          "d",
		"system_prompt":        "p",
		"guardrail_prompt":     "g",
		"model":                "m",
		"retry_count":          3,
		"memory_namespace":     "ns",
		"max_delegation_depth": 4,
		"agent_tools":          []map[string]interface{}{{"tool_name": "t", "action_name": "a"}},
		"output_schema":        map[string]interface{}{"type": "object"},
		"budget":               map[string]interface{}{"max_attempts": 7},
		"validation_rules":     []map[string]interface{}{{"rules": []map[string]interface{}{{"property": "x", "code": "c", "message": "m"}}}},
		"evidence":             []map[string]interface{}{{"name": "n", "action": "a", "from_arg": "k"}},
		"claims":               []map[string]interface{}{{"claims": "c", "action": "a"}},
	}

	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var agent Agent
	if err := json.Unmarshal(raw, &agent); err != nil {
		t.Fatal(err)
	}

	value := reflect.ValueOf(agent)
	structType := value.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if tag == "" || resolvedElsewhere[tag] {
			continue
		}
		if _, supplied := config[tag]; !supplied {
			t.Errorf("Agent has a json field %q that this test does not supply - add it to the config above, "+
				"then make sure UnmarshalJSON reads it", tag)
			continue
		}
		if value.Field(i).IsZero() {
			t.Errorf("%q was set in the config and arrived EMPTY. Agent.UnmarshalJSON is an allow-list: add "+
				"%q to its TempAgent struct and assign it, or no config file will ever be able to set it.", tag, tag)
		}
	}
}

// The five that were actually lost, named, so a regression reads as itself
// rather than as a generic failure.
func TestTheFieldsThatWereSilentlyDropped(t *testing.T) {
	raw := []byte(`{
      "agent_type": "REASONING", "agent_name": "a",
      "budget": {"max_attempts": 5, "max_total_tokens": 1000, "max_duration_seconds": 60},
      "max_delegation_depth": 2,
      "validation_rules": [{"subjects": "inv", "rules": [{"property": "inv_no", "code": "c", "message": "m"}]}],
      "evidence": [{"name": "threads_read", "action": "read_conversation", "from_arg": "conversation_id"}],
      "claims": [{"claims": "created", "action": "save_field", "arg_key": "name"}]
    }`)
	var agent Agent
	if err := json.Unmarshal(raw, &agent); err != nil {
		t.Fatal(err)
	}
	if agent.Budget.MaxAttempts != 5 || agent.Budget.MaxTotalTokens != 1000 || agent.Budget.MaxDurationSeconds != 60 {
		t.Errorf("budget lost: %+v", agent.Budget)
	}
	if agent.MaxDelegationDepth != 2 {
		t.Errorf("max_delegation_depth lost: %d", agent.MaxDelegationDepth)
	}
	if len(agent.ValidationRules) != 1 || len(agent.ValidationRules[0].Rules) != 1 {
		t.Errorf("validation_rules lost: %+v", agent.ValidationRules)
	}
	if agent.ValidationRules[0].Subjects != "inv" {
		t.Errorf("the subjects path was lost: %+v", agent.ValidationRules[0])
	}
	if len(agent.Evidence) != 1 || agent.Evidence[0].FromArg != "conversation_id" {
		t.Errorf("evidence lost: %+v", agent.Evidence)
	}
	if len(agent.Claims) != 1 || agent.Claims[0].ArgKey != "name" {
		t.Errorf("claims lost: %+v", agent.Claims)
	}
}
