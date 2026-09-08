package agents

import (
	"context"
	"testing"
)

func TestParseAndFormatAgentChain(t *testing.T) {
	chain := ParseAgentChain(" root / middle /leaf ")
	if len(chain) != 3 || chain[0] != "root" || chain[2] != "leaf" {
		t.Fatalf("unexpected chain : %v", chain)
	}
	if got := FormatAgentChain(chain); got != "root/middle/leaf" {
		t.Errorf("unexpected formatted chain : %s", got)
	}
	if got := ParseAgentChain(""); got != nil {
		t.Errorf("expected nil for an empty header, got %v", got)
	}
}

func TestChainWithDoesNotAliasCaller(t *testing.T) {
	base := make([]string, 1, 4)
	base[0] = "root"
	first := ChainWith(base, "a")
	second := ChainWith(base, "b")
	if FormatAgentChain(first) != "root/a" || FormatAgentChain(second) != "root/b" {
		t.Fatalf("branches aliased a shared backing array : %v / %v", first, second)
	}
}

func TestChainAllowsRepeatedAgent(t *testing.T) {
	chain := ChainWith(ChainWith([]string{"a"}, "b"), "a")
	if FormatAgentChain(chain) != "a/b/a" {
		t.Errorf("A -> B -> A is legal and must be representable, got %s", FormatAgentChain(chain))
	}
	if len(chain) >= MaxAgentChainDepth {
		t.Errorf("a three-hop chain must be well inside the depth cap")
	}
}

func TestAgentChainRoundTripsThroughContext(t *testing.T) {
	ctx := WithAgentChain(context.Background(), []string{"root", "mid"})
	if got := FormatAgentChain(AgentChain(ctx)); got != "root/mid" {
		t.Errorf("unexpected chain from context : %s", got)
	}
	if AgentChain(context.Background()) != nil {
		t.Error("expected no chain on a bare context")
	}
}

func TestStreamTargetRequiresBothParts(t *testing.T) {
	if _, ok := GetStreamTarget(WithStreamTarget(context.Background(), StreamTarget{StreamId: "s"})); ok {
		t.Error("a target without a callback url must not be usable")
	}
	if _, ok := GetStreamTarget(WithStreamTarget(context.Background(), StreamTarget{CallbackUrl: "http://x"})); ok {
		t.Error("a target without a stream id must not be usable")
	}
	target, ok := GetStreamTarget(WithStreamTarget(context.Background(), StreamTarget{StreamId: "s", CallbackUrl: "http://x"}))
	if !ok || target.StreamId != "s" {
		t.Error("expected a usable target")
	}
}

func TestStreamEventAttributeIsSetOnceBySource(t *testing.T) {
	event := StreamEvent{Event: StreamEventThinking}.Attribute("leaf", []string{"root", "mid"})
	if event.Agent != "leaf" || event.Chain != "root/mid/leaf" {
		t.Fatalf("unexpected attribution : %+v", event)
	}
	relayed := event.Attribute("mid", []string{"root"})
	if relayed.Agent != "leaf" || relayed.Chain != "root/mid/leaf" {
		t.Errorf("a relaying hop must not relabel the event : %+v", relayed)
	}
}
