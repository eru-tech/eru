package reasoning_agents

import (
	"context"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
	models "github.com/eru-tech/eru/eru-ai/models"
)

// streamCtx is a request that is being streamed and has a component scanner, the
// way EruStudioAgent.Execute sets one up.
func streamCtx() (context.Context, *[]agents.StreamEvent) {
	captured := &[]agents.StreamEvent{}
	ctx := agents.WithStreamCallback(context.Background(), func(event agents.StreamEvent) {
		*captured = append(*captured, event)
	})
	ctx = studio.WithComponentScanner(ctx, studio.NewComponentScanner())
	return ctx, captured
}

func delta(chunk string) models.ModelStreamEvent {
	return models.ModelStreamEvent{
		Type:      models.StreamToolInputDelta,
		ToolName:  models.TerminalToolStructuredOutput,
		Content:   chunk,
		Iteration: 1,
	}
}

func TestEnrichStreamEmitsComponentsAsTheModelWritesThem(t *testing.T) {
	agent := &EruStudioAgent{}
	ctx, _ := streamCtx()

	chunks := []string{
		`{"mode":"patch","patch":{"upsert":[`,
		`{"id":"root","type":"flex_container","children_ids":["hello"]}`,
		`,{"id":"hello","type":"text",`,
		`"properties":{"base":{"label":"Hi"}}}`,
		`]}}`,
	}

	emitted := []agents.StreamEvent{}
	for _, chunk := range chunks {
		emitted = append(emitted, agent.EnrichStream(ctx, delta(chunk))...)
	}

	if len(emitted) != 2 {
		t.Fatalf("expected two components, got %d: %+v", len(emitted), emitted)
	}
	for _, event := range emitted {
		if event.Event != agents.StreamEventPageComponent {
			t.Errorf("event = %q, want %q", event.Event, agents.StreamEventPageComponent)
		}
		if event.Iteration != 1 {
			t.Errorf("iteration = %d", event.Iteration)
		}
	}

	first, ok := emitted[0].Data.(studio.ScannedComponent)
	if !ok {
		t.Fatalf("event data is %T, want a ScannedComponent", emitted[0].Data)
	}
	if first.Id != "root" || first.Sequence != 1 {
		t.Errorf("first component = %+v", first)
	}
	if len(first.ChildrenIds) != 1 || first.ChildrenIds[0] != "hello" {
		t.Errorf("the adjacency a renderer needs was lost: %+v", first)
	}

	second := emitted[1].Data.(studio.ScannedComponent)
	if second.Id != "hello" || second.Type != "text" || second.Sequence != 2 {
		t.Errorf("second component = %+v", second)
	}
	// The component arrived from a chunk boundary in the middle of its JSON,
	// which is the case that matters: it still parsed whole.
	properties, _ := second.Component["properties"].(map[string]interface{})
	if properties == nil {
		t.Errorf("the component was emitted without its properties: %+v", second.Component)
	}
}

func TestEnrichStreamIgnoresEverythingElse(t *testing.T) {
	agent := &EruStudioAgent{}
	ctx, _ := streamCtx()

	cases := []models.ModelStreamEvent{
		{Type: models.StreamThinking, Content: `{"id":"x","type":"text"}`},
		{Type: models.StreamTextDelta, Content: `{"id":"x","type":"text"}`},
		{Type: models.StreamToolUse, ToolName: models.TerminalToolStructuredOutput},
		// A delta from another tool is another tool's arguments, not the page.
		{Type: models.StreamToolInputDelta, ToolName: "get_component_spec", Content: `{"types":["grid"]}`},
	}
	for _, event := range cases {
		if got := agent.EnrichStream(ctx, event); len(got) > 0 {
			t.Errorf("%s produced %d event(s)", event.Type, len(got))
		}
	}
}

func TestEnrichStreamIsInertWithoutAScanner(t *testing.T) {
	// A non-streamed request has no scanner, and enrichment must cost nothing.
	agent := &EruStudioAgent{}
	if got := agent.EnrichStream(context.Background(), delta(`{"patch":{"upsert":[{"id":"a","type":"text"}]}}`)); got != nil {
		t.Errorf("a request with no scanner produced %+v", got)
	}
}

func TestEnrichStreamAlsoWorksForAFullPage(t *testing.T) {
	// Progressive rendering is not only for patches: a page built from scratch is
	// the longest wait, and its root components stream the same way.
	agent := &EruStudioAgent{}
	ctx, _ := streamCtx()

	emitted := agent.EnrichStream(ctx, delta(`{"mode":"full","page":{"id":"p","components":[{"id":"root","type":"card"}`))
	emitted = append(emitted, agent.EnrichStream(ctx, delta(`,{"id":"second","type":"text"}]}}`))...)

	if len(emitted) != 2 {
		t.Fatalf("expected two components, got %d", len(emitted))
	}
	if source := emitted[0].Data.(studio.ScannedComponent).Source; source != "components" {
		t.Errorf("source = %q, want \"components\"", source)
	}
}
