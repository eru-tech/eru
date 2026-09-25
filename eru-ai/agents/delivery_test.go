package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	eru_models "github.com/eru-tech/eru/eru-models"
)

type deliveryProvider struct {
	reject map[string]bool
	checks int
}

func (deliveryProvider) GetSystemPrompt() string { return "" }
func (deliveryProvider) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	return eru_models.JSONSchema{}
}
func (p *deliveryProvider) ValidateOutput(ctx context.Context, output map[string]interface{}) error {
	p.checks++
	tag, _ := output["v"].(string)
	if p.reject[tag] {
		return errors.New("component type \"nope\" is not in the library")
	}
	return nil
}

// A provider that declares no rules at all - the default for a configured agent.
type bareProvider struct{}

func (bareProvider) GetSystemPrompt() string { return "" }
func (bareProvider) GetOutputSchema(ctx context.Context) eru_models.JSONSchema {
	return eru_models.JSONSchema{}
}

func answer(tag string) AgentMessage {
	message := AgentMessage{Actions: []AgentOutputAction{{
		ActionType: ActionTypeAnswer, ActionName: "test_agent",
		Action: map[string]interface{}{"v": tag},
	}}}
	SealAnswer(&message)
	return message
}

func TestTheSealTracksTheArtifactNotTheEnvelope(t *testing.T) {
	message := answer("a")
	// Things that change for reasons unrelated to the artifact must not break it.
	message.RetryCount = 3
	message.Metrics = &ExecutionMetrics{TotalIterations: 9}
	if SealOf(message.Actions) != message.Seal {
		t.Error("the seal must survive changes to anything but the actions")
	}
	message.Actions[0].Action["v"] = "b"
	if SealOf(message.Actions) == message.Seal {
		t.Error("changing the artifact must break the seal")
	}
}

// Key ordering must not matter, or the seal breaks at random.
func TestTheSealIsStableAcrossKeyOrder(t *testing.T) {
	one := []AgentOutputAction{{ActionType: ActionTypeAnswer, Action: map[string]interface{}{
		"b": 2, "a": map[string]interface{}{"y": 1, "x": []interface{}{1, 2}},
	}}}
	two := []AgentOutputAction{{ActionType: ActionTypeAnswer, Action: map[string]interface{}{
		"a": map[string]interface{}{"x": []interface{}{1, 2}, "y": 1}, "b": 2,
	}}}
	if SealOf(one) != SealOf(two) {
		t.Error("two identical artifacts must seal the same")
	}
}

// The case the whole file exists for: a transform that breaks the artifact must
// not be delivered.
func TestATransformThatIntroducesAFaultIsRefused(t *testing.T) {
	provider := &deliveryProvider{reject: map[string]bool{"broken": true}}
	_, err := DeliverTransformed(context.Background(), provider, answer("good"), FaultRefuses, func(out AgentMessage) (AgentMessage, error) {
		out.Actions[0].Action = map[string]interface{}{"v": "broken"}
		return out, nil
	})
	if err == nil {
		t.Fatal("a transform that breaks the answer must be refused")
	}
	var broken *DeliveryBroken
	if !errors.As(err, &broken) {
		t.Fatalf("want DeliveryBroken, got %T", err)
	}
	if !strings.Contains(err.Error(), "test_agent") {
		t.Errorf("the error must name the agent: %v", err)
	}
}

// Legitimate shaping is the common case and must go through untouched.
func TestASoundTransformIsDeliveredAndResealed(t *testing.T) {
	provider := &deliveryProvider{}
	out, err := DeliverTransformed(context.Background(), provider, answer("good"), FaultRefuses, func(out AgentMessage) (AgentMessage, error) {
		out.Actions[0].Action = map[string]interface{}{"v": "good", "resolved": true}
		return out, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Seal != SealOf(out.Actions) {
		t.Error("the transformed answer must be resealed, or the tripwire fires on it later")
	}
	if provider.checks != 1 {
		t.Errorf("the transform must be re-validated exactly once, was %d", provider.checks)
	}
}

// A transform that changes nothing must not cost a validation pass.
func TestANoOpTransformIsNotRevalidated(t *testing.T) {
	provider := &deliveryProvider{}
	out, err := DeliverTransformed(context.Background(), provider, answer("good"), FaultRefuses, func(out AgentMessage) (AgentMessage, error) {
		return out, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.checks != 0 {
		t.Errorf("nothing changed, so nothing needed re-checking; ran %d", provider.checks)
	}
	if out.Seal == "" {
		t.Error("the original seal must be kept")
	}
}

// An agent that declares no rules gets the reseal and nothing else - the right
// amount of checking for an agent with nothing to check against.
func TestAnAgentWithNoValidatorStillGetsResealed(t *testing.T) {
	out, err := DeliverTransformed(context.Background(), bareProvider{}, answer("x"), FaultRefuses, func(out AgentMessage) (AgentMessage, error) {
		out.Actions[0].Action = map[string]interface{}{"v": "y"}
		return out, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Seal != SealOf(out.Actions) {
		t.Error("the answer must still be resealed")
	}
}

// The tripwire: a transform that bypassed DeliverTransformed entirely.
func TestVerifyDeliveryCatchesAnUnsealedChange(t *testing.T) {
	provider := &deliveryProvider{reject: map[string]bool{"smuggled": true}}
	message := answer("good")
	message.Actions[0].Action = map[string]interface{}{"v": "smuggled"} // no reseal

	err := VerifyDelivery(context.Background(), provider, message)
	if err == nil {
		t.Fatal("a broken seal over an invalid artifact must be reported")
	}
	if !strings.Contains(err.Error(), "does not pass") {
		t.Errorf("the reason must say the delivered artifact fails: %v", err)
	}
}

// Changed but still sound: reported in the log, not turned into an error. The
// answer already exists and refusing it would turn a reporting problem into an
// outage.
func TestAChangedButValidArtifactIsNotFatal(t *testing.T) {
	provider := &deliveryProvider{}
	message := answer("good")
	message.Actions[0].Action = map[string]interface{}{"v": "good", "extra": 1}
	if err := VerifyDelivery(context.Background(), provider, message); err != nil {
		t.Errorf("a sound artifact must still be delivered: %v", err)
	}
}

// Not every message is sealed - a clarification question, an agent type that
// does not use the loop. "No claim made" must not read as "claim broken".
func TestAnUnsealedMessagePasses(t *testing.T) {
	message := AgentMessage{Actions: []AgentOutputAction{{
		ActionType: ActionTypeQuestion, ActionName: "a", Action: map[string]interface{}{"q": "which entity?"},
	}}}
	if err := VerifyDelivery(context.Background(), &deliveryProvider{}, message); err != nil {
		t.Errorf("an unsealed message is not a violation: %v", err)
	}
}

// A transform that errors is the agent type's own failure and must surface as
// itself, not be dressed up as a delivery violation.
func TestATransformErrorIsReturnedAsItIs(t *testing.T) {
	sentinel := errors.New("the patch could not be applied")
	_, err := DeliverTransformed(context.Background(), &deliveryProvider{}, answer("good"), FaultRefuses, func(out AgentMessage) (AgentMessage, error) {
		return AgentMessage{}, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("want the transform's own error, got %v", err)
	}
}

// A fault the answer itself discloses is not a silent one, and the seal exists
// to stop silent delivery. eru_studio's composer attaches a flawed nested page
// WITH a warning, deliberately - refusing that would override a product
// decision and turn one imperfection into an error.
func TestADisclosedFaultIsDeliveredRatherThanRefused(t *testing.T) {
	provider := &deliveryProvider{reject: map[string]bool{"flawed": true}}
	out, err := DeliverTransformed(context.Background(), provider, answer("good"), FaultReports,
		func(out AgentMessage) (AgentMessage, error) {
			out.Actions[0].Action = map[string]interface{}{"v": "flawed", "warnings": []interface{}{"the card page has 1 unresolved issue"}}
			return out, nil
		})
	if err != nil {
		t.Fatalf("a disclosed fault must still be delivered: %v", err)
	}
	if out.Seal != SealOf(out.Actions) {
		t.Error("and must be resealed, or the tripwire fires on it later")
	}
}

// The default stays strict: an undisclosed fault is refused.
func TestTheDefaultStillRefuses(t *testing.T) {
	provider := &deliveryProvider{reject: map[string]bool{"broken": true}}
	if _, err := DeliverTransformed(context.Background(), provider, answer("good"), FaultRefuses,
		func(out AgentMessage) (AgentMessage, error) {
			out.Actions[0].Action = map[string]interface{}{"v": "broken"}
			return out, nil
		}); err == nil {
		t.Fatal("a silent fault must still be refused")
	}
	if FaultRefuses != 0 {
		t.Error("refusing must be the zero value, so a caller that says nothing gets the safe behaviour")
	}
}
