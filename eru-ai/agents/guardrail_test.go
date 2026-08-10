package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestGuardrailSectionEmptyWhenNotConfigured(t *testing.T) {
	for _, gp := range []string{"", "   \n\t "} {
		agent := &Agent{GuardrailPrompt: gp}
		if got := agent.GuardrailSection(); got != "" {
			t.Errorf("expected empty guardrail section for %q, got %q", gp, got)
		}
	}
}

func TestGuardrailSectionFramesConfiguredPrompt(t *testing.T) {
	agent := &Agent{GuardrailPrompt: "  Only answer questions about loan products.  "}
	section := agent.GuardrailSection()

	checks := []string{
		"AGENT GUARDRAILS / BOUNDARIES — NON-NEGOTIABLE",
		"Only answer questions about loan products.",
		"--- GUARDRAILS (AGENT BOUNDARIES) ---",
		"--- END GUARDRAILS ---",
		"outside this agent's scope",
	}
	for _, c := range checks {
		if !strings.Contains(section, c) {
			t.Errorf("guardrail section missing %q", c)
		}
	}
	if strings.Contains(section, "  Only answer") {
		t.Error("guardrail text was not trimmed")
	}
}

func TestGuardrailPromptUnmarshalled(t *testing.T) {
	body := json.RawMessage(`{"agent_type":"REFLEX","agent_name":"a1","guardrail_prompt":"stay on topic"}`)
	agent := &Agent{}
	if err := agent.MakeFromJson(context.Background(), &body); err != nil {
		t.Fatalf("MakeFromJson failed: %v", err)
	}
	if agent.GuardrailPrompt != "stay on topic" {
		t.Errorf("guardrail_prompt not unmarshalled, got %q", agent.GuardrailPrompt)
	}

	got, err := agent.GetAttribute(context.Background(), "guardrail_prompt")
	if err != nil {
		t.Fatalf("GetAttribute failed: %v", err)
	}
	if got != "stay on topic" {
		t.Errorf("unexpected attribute value: %v", got)
	}
}
