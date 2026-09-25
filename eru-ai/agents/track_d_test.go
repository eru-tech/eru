package agents

import (
	"context"
	"strings"
	"testing"

	ruleset "github.com/eru-tech/eru/eru-ai/agents/ruleset"
)

// ---- write-once ----

func writeCtx(effects map[string]ToolEffect) context.Context {
	ctx := WithToolEffects(context.Background(), effects)
	return WithWriteOnce(ctx, NewWriteOnce())
}

// The bug: the loop retries, the agent saves the entity again, and the user
// asked for one entity and got three.
func TestARepeatedWriteIsNotPerformedTwice(t *testing.T) {
	ctx := writeCtx(map[string]ToolEffect{"save_entity": {}})
	args := map[string]interface{}{"name": "invoices"}

	if _, suppressed := guardWrite(ctx, "save_entity", args); suppressed {
		t.Fatal("the first call must go through")
	}
	WriteOnceFrom(ctx).Record("save_entity", args, map[string]interface{}{"id": "e1"})

	result, suppressed := guardWrite(ctx, "save_entity", args)
	if !suppressed {
		t.Fatal("an identical second write must be suppressed")
	}
	if result["id"] != "e1" {
		t.Errorf("the first call's result must be returned, got %+v", result)
	}
	// Telling the agent it failed would send it round the loop fixing something
	// that already worked.
	note, _ := result["note"].(string)
	if !strings.Contains(note, "already done") {
		t.Errorf("the note must say the work is done, not that the call failed: %q", note)
	}
}

func TestDifferentArgumentsAreDifferentWork(t *testing.T) {
	ctx := writeCtx(map[string]ToolEffect{"save_entity": {}})
	WriteOnceFrom(ctx).Record("save_entity", map[string]interface{}{"name": "invoices"}, map[string]interface{}{"id": "e1"})
	if _, suppressed := guardWrite(ctx, "save_entity", map[string]interface{}{"name": "customers"}); suppressed {
		t.Error("saving a different entity is a second piece of work, not a repeat")
	}
}

// A regenerated request id must not make a repeat look new.
func TestVolatileArgumentsAreIgnoredInTheKey(t *testing.T) {
	one := callKey("save", map[string]interface{}{"name": "x", "request_id": "a1"})
	two := callKey("save", map[string]interface{}{"name": "x", "request_id": "b2"})
	if one != two {
		t.Error("a changed request id must not defeat the guard")
	}
	if one == callKey("save", map[string]interface{}{"name": "y", "request_id": "a1"}) {
		t.Error("a changed name is different work")
	}
}

func TestReadOnlyAndIdempotentToolsRepeatFreely(t *testing.T) {
	ctx := writeCtx(map[string]ToolEffect{
		"run_query":   {ReadOnly: true},
		"set_by_id":   {Idempotent: true},
		"append_note": {},
	})
	args := map[string]interface{}{"x": 1}
	for _, action := range []string{"run_query", "set_by_id"} {
		WriteOnceFrom(ctx).Record(action, args, map[string]interface{}{"ok": true})
		if _, suppressed := guardWrite(ctx, action, args); suppressed {
			t.Errorf("%s may repeat", action)
		}
	}
	WriteOnceFrom(ctx).Record("append_note", args, map[string]interface{}{"ok": true})
	if _, suppressed := guardWrite(ctx, "append_note", args); !suppressed {
		t.Error("an undeclared-but-listed write must be guarded")
	}
}

// An agent that declares nothing must behave exactly as before.
func TestAnUndeclaredToolIsNeverSuppressed(t *testing.T) {
	ctx := writeCtx(map[string]ToolEffect{"save_entity": {}})
	args := map[string]interface{}{"x": 1}
	WriteOnceFrom(ctx).Record("something_else", args, map[string]interface{}{"ok": true})
	if _, suppressed := guardWrite(ctx, "something_else", args); suppressed {
		t.Error("a tool nobody described must be left alone")
	}
}

func TestEffectsAreCollectedFromNestedTools(t *testing.T) {
	effects := EffectsOf([]AgentTools{
		{ActionName: "save_entity", Effect: &ToolEffect{}},
		{ActionName: "parent", DependentTools: []AgentTools{
			{ActionName: "run_query", Effect: &ToolEffect{ReadOnly: true}},
		}},
		{ActionName: "unannotated"},
	})
	if len(effects) != 2 {
		t.Fatalf("expected two declared effects, got %+v", effects)
	}
	if !effects["run_query"].ReadOnly {
		t.Error("a dependent tool's declaration must be collected")
	}
	if _, declared := effects["unannotated"]; declared {
		t.Error("a tool that says nothing must not appear")
	}
}

// ---- delegation ----

func TestAnAgentCannotCallItself(t *testing.T) {
	ctx, err := EnterAgent(context.Background(), "orchestrator", 5)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err = EnterAgent(ctx, "page_builder", 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EnterAgent(ctx, "orchestrator", 5); err == nil {
		t.Fatal("an agent already running must not be re-entered")
	}
	if !strings.Contains(err.Error(), "orchestrator -> page_builder -> orchestrator") {
		t.Errorf("the error must name the chain, or it cannot be fixed: %v", err)
	}
}

func TestDelegationHasADepthLimit(t *testing.T) {
	ctx := context.Background()
	var err error
	for _, name := range []string{"a", "b", "c"} {
		if ctx, err = EnterAgent(ctx, name, 3); err != nil {
			t.Fatalf("depth 3 must be reachable: %v", err)
		}
	}
	if _, err = EnterAgent(ctx, "d", 3); err == nil {
		t.Fatal("the fourth layer must be refused")
	}
	if !strings.Contains(err.Error(), "a -> b -> c") {
		t.Errorf("the error must name the chain: %v", err)
	}
}

func TestDepthAndChainAreReadable(t *testing.T) {
	ctx, _ := EnterAgent(context.Background(), "a", 0)
	ctx, _ = EnterAgent(ctx, "b", 0)
	if DelegationDepth(ctx) != 2 {
		t.Errorf("depth = %d", DelegationDepth(ctx))
	}
	if chain := DelegationChain(ctx); len(chain) != 2 || chain[0] != "a" {
		t.Errorf("chain = %v", chain)
	}
}

// ---- claims ----

func claimCtx(calls ...ToolInvocation) context.Context {
	record := NewToolRecord()
	for _, call := range calls {
		record.Add(call)
	}
	return WithToolRecord(context.Background(), record)
}

func TestAClaimAboutWorkThatNeverHappenedIsCaught(t *testing.T) {
	ctx := claimCtx(
		ToolInvocation{Action: "save_field", Args: map[string]interface{}{"name": "inv_no"}, Ok: true},
	)
	answer := map[string]interface{}{"created_fields": []interface{}{"inv_no", "inv_dt", "amt"}}
	rules := []ClaimRule{{Claims: "created_fields", Action: "save_field", ArgKey: "name"}}

	unsupported := VerifyClaims(ctx, rules, answer, ruleset.SeverityQuality)
	if len(unsupported) != 2 {
		t.Fatalf("two of the three were never saved: %v", unsupported)
	}
	joined := strings.Join(unsupported, " ")
	if !strings.Contains(joined, "inv_dt") || !strings.Contains(joined, "amt") {
		t.Errorf("both unsupported claims must be named: %v", unsupported)
	}
	if strings.Contains(joined, `"inv_no"`) {
		t.Errorf("the claim that IS supported must not be reported: %v", unsupported)
	}
	// Saying what was actually done is what makes it fixable.
	if !strings.Contains(joined, "inv_no") {
		t.Errorf("the message should list what was called: %v", unsupported)
	}
}

func TestAFailedCallDoesNotSupportAClaim(t *testing.T) {
	ctx := claimCtx(
		ToolInvocation{Action: "save_field", Args: map[string]interface{}{"name": "inv_no"}, Ok: false, Error: "boom"},
	)
	answer := map[string]interface{}{"created_fields": []interface{}{"inv_no"}}
	rules := []ClaimRule{{Claims: "created_fields", Action: "save_field", ArgKey: "name"}}
	if unsupported := VerifyClaims(ctx, rules, answer, ruleset.SeverityQuality); len(unsupported) != 1 {
		t.Fatalf("a call that errored did not do the thing: %v", unsupported)
	}
}

func TestAnActionThatNeverRanIsSaidDifferently(t *testing.T) {
	ctx := claimCtx(ToolInvocation{Action: "run_query", Ok: true})
	answer := map[string]interface{}{"created_fields": []interface{}{"inv_no"}}
	rules := []ClaimRule{{Claims: "created_fields", Action: "save_field", ArgKey: "name"}}
	unsupported := VerifyClaims(ctx, rules, answer, ruleset.SeverityQuality)
	if len(unsupported) != 1 || !strings.Contains(unsupported[0], "never called") {
		t.Errorf("an action that never ran is a stronger statement than a mismatch: %v", unsupported)
	}
}

// No record means no evidence either way, and a check that cannot run must not
// read as a check that failed.
func TestNoToolRecordMeansNoVerdict(t *testing.T) {
	answer := map[string]interface{}{"created_fields": []interface{}{"anything"}}
	rules := []ClaimRule{{Claims: "created_fields", Action: "save_field", ArgKey: "name"}}
	if unsupported := VerifyClaims(context.Background(), rules, answer, ruleset.SeverityQuality); len(unsupported) != 0 {
		t.Errorf("without a record nothing can be concluded: %v", unsupported)
	}
}

// Severity decides whether an unsupported claim rejects or marks down, and the
// default is the cautious one because the evidence is incomplete.
func TestClaimSeverityIsRespectedAndDefaultsToQuality(t *testing.T) {
	ctx := claimCtx(ToolInvocation{Action: "save_field", Ok: true})
	answer := map[string]interface{}{"f": []interface{}{"nope"}}

	soft := []ClaimRule{{Claims: "f", Action: "save_field", ArgKey: "name"}}
	if len(VerifyClaims(ctx, soft, answer, ruleset.SeverityQuality)) != 1 {
		t.Error("an undeclared severity must be quality")
	}
	if len(VerifyClaims(ctx, soft, answer, ruleset.SeverityError)) != 0 {
		t.Error("and must not reject")
	}
	// The trap this flag exists to avoid: SeverityError is ruleset's zero value,
	// so a claim rule that said nothing would have rejected if severity were
	// reused for it.
	if len(VerifyClaims(ctx, soft, answer, ruleset.SeverityQuality)) == 0 {
		t.Error("silence must mean mark-it-down")
	}

	hard := []ClaimRule{{Claims: "f", Action: "save_field", ArgKey: "name", Reject: true}}
	if len(VerifyClaims(ctx, hard, answer, ruleset.SeverityError)) != 1 {
		t.Error("a rule declared as an error must reject")
	}
}

// The guard must be installed on EVERY agent that runs, not only the ones that
// happen to use the reasoning loop. An orchestrator that never entered the chain
// would make the depth limit count only the layers below it - the half that was
// never going to loop in the first place.
func TestTheChainCountsEveryAgentThatRuns(t *testing.T) {
	ctx, err := EnterAgent(context.Background(), "orchestrator", 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EnterAgent(ctx, "page_builder", 2); err != nil {
		t.Fatalf("depth 2 is allowed: %v", err)
	}
	// If the orchestrator had not entered, this third layer would still be
	// within the limit - which is exactly the bug.
	ctx2, _ := EnterAgent(ctx, "page_builder", 2)
	if _, err = EnterAgent(ctx2, "helper", 2); err == nil {
		t.Error("the orchestrator's own layer must count toward the limit")
	}
}

// The more dangerous claim: "it was already correct, nothing to do". A wrong
// "created" is found out; a wrong "already there" tells the user to stop looking.
func TestAFalseAlreadyThereIsCaught(t *testing.T) {
	ctx := claimCtx(
		ToolInvocation{Action: "get_entity_metadata", Args: map[string]interface{}{"field": "inv_no"}, Ok: true},
	)
	answer := map[string]interface{}{"unchanged_fields": []interface{}{"inv_no", "invented_field"}}
	rules := []ClaimRule{{Unchanged: "unchanged_fields", Action: "get_entity_metadata", ArgKey: "field"}}

	unsupported := VerifyClaims(ctx, rules, answer, ruleset.SeverityQuality)
	if len(unsupported) != 1 {
		t.Fatalf("only the invented one is unsupported: %v", unsupported)
	}
	if !strings.Contains(unsupported[0], "invented_field") {
		t.Errorf("it must name the claim: %v", unsupported)
	}
	if !strings.Contains(unsupported[0], "stop looking") {
		t.Errorf("the message must say why this one matters more: %v", unsupported)
	}
}

// Not calling a save is exactly what an agent SHOULD do when the thing already
// exists, so an unchanged-only rule must say nothing when the action never ran.
func TestAnUnchangedOnlyRuleIsSilentWhenNothingRan(t *testing.T) {
	ctx := claimCtx(ToolInvocation{Action: "something_else", Ok: true})
	answer := map[string]interface{}{"unchanged_fields": []interface{}{"inv_no"}}
	rules := []ClaimRule{{Unchanged: "unchanged_fields", Action: "get_entity_metadata", ArgKey: "field"}}
	if unsupported := VerifyClaims(ctx, rules, answer, ruleset.SeverityQuality); len(unsupported) != 0 {
		t.Errorf("correct behaviour must not be faulted: %v", unsupported)
	}
}
