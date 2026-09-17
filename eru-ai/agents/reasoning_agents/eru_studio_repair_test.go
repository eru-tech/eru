package reasoning_agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
)

func repairCtx(basePage map[string]interface{}) context.Context {
	ctx := studio.WithOutputMode(context.Background(), studio.ModePatch)
	ctx = studio.WithBasePage(ctx, basePage)
	ctx = studio.WithRepairState(ctx, studio.NewRepairState())
	return ctx
}

func simplePage(id string, componentId string) map[string]interface{} {
	return map[string]interface{}{
		"id":     id,
		"name":   id,
		"styles": map[string]interface{}{},
		"components": []interface{}{
			map[string]interface{}{
				"id":         componentId,
				"type":       "text",
				"properties": map[string]interface{}{"base": map[string]interface{}{}},
				"styles":     map[string]interface{}{},
			},
		},
	}
}

func TestRepairTurnRebasesOntoTheRejectedPage(t *testing.T) {
	agent := &EruStudioAgent{}
	ctx := repairCtx(nil)
	failed := map[string]interface{}{"mode": studio.ModeFull, "page": simplePage("p1", "c1")}

	turn, ok := agent.RepairTurn(ctx, failed, errors.New("something was wrong"))
	if !ok {
		t.Fatal("a whole page that failed validation should be repairable")
	}
	if turn.Schema.Type == "" {
		t.Error("the repair turn should carry the patch schema")
	}
	if !strings.Contains(turn.Prompt, "mode \"patch\"") {
		t.Errorf("the prompt should ask for a patch: %s", turn.Prompt)
	}
	if strings.Contains(turn.Prompt, "write it again") && !strings.Contains(turn.Prompt, "Do NOT write it again") {
		t.Error("the prompt must tell the model not to rewrite the page")
	}

	state := studio.RepairStateFrom(ctx)
	if !state.Active() {
		t.Fatal("the repair should be recorded as in flight")
	}
	if id, _ := state.Base()["id"].(string); id != "p1" {
		t.Errorf("the rejected page should be the new base, got %v", state.Base()["id"])
	}
	if got := effectiveBasePage(ctx); got["id"] != "p1" {
		t.Errorf("effectiveBasePage should follow the repair, got %v", got["id"])
	}
}

// A scope is a promise about which components an answer may touch, made against
// the page the user is looking at. Re-basing the diff would change what it means.
func TestRepairTurnDeclinesAScopedEdit(t *testing.T) {
	agent := &EruStudioAgent{}
	base := simplePage("p1", "c1")
	ctx := repairCtx(base)
	ctx = studio.WithScope(ctx, &studio.ResolvedScope{})

	if _, ok := agent.RepairTurn(ctx, map[string]interface{}{"mode": studio.ModeFull, "page": base}, errors.New("x")); ok {
		t.Fatal("a scoped edit must take the ordinary retry")
	}
}

// Bare-page mode has no patch vocabulary to answer in.
func TestRepairTurnDeclinesWithoutTheEnvelope(t *testing.T) {
	agent := &EruStudioAgent{}
	ctx := studio.WithOutputMode(context.Background(), studio.ModeFull)
	ctx = studio.WithRepairState(ctx, studio.NewRepairState())
	if _, ok := agent.RepairTurn(ctx, map[string]interface{}{"page": simplePage("p1", "c1")}, errors.New("x")); ok {
		t.Fatal("bare mode has no patch schema")
	}
}

// When the answer failed on its own shape there is nothing coherent to diff.
func TestRepairTurnDeclinesAnAnswerWithNoReadablePage(t *testing.T) {
	agent := &EruStudioAgent{}
	ctx := repairCtx(nil)
	for name, failed := range map[string]map[string]interface{}{
		"no page at all":     {"mode": studio.ModeFull},
		"components missing": {"mode": studio.ModeFull, "page": map[string]interface{}{"id": "p1", "name": "p1"}},
		"unknown mode":       {"mode": "sideways"},
	} {
		if _, ok := agent.RepairTurn(ctx, failed, errors.New("x")); ok {
			t.Errorf("%s: should not be repairable", name)
		}
	}
}

// The point of the repair: the client is handed the finished page, not a diff
// against an attempt it never saw.
func TestARepairResolvesToAFullPage(t *testing.T) {
	clientPage := simplePage("p1", "c1")
	ctx := repairCtx(clientPage)

	rejected := simplePage("p1", "c1")
	rejected["components"] = append(rejected["components"].([]interface{}), map[string]interface{}{
		"id":         "c2",
		"type":       "text",
		"properties": map[string]interface{}{"base": map[string]interface{}{}},
		"styles":     map[string]interface{}{},
	})
	studio.RepairStateFrom(ctx).Begin(rejected, nil)

	repairPatch := map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"upsert": []interface{}{map[string]interface{}{
				"id":         "c2",
				"type":       "text",
				"properties": map[string]interface{}{"base": map[string]interface{}{"label": "fixed"}},
				"styles":     map[string]interface{}{},
			}},
		},
	}

	resolved, _, err := resolveStudioOutput(ctx, repairPatch, effectiveBasePage(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if resolved["mode"] != studio.ModeFull {
		t.Errorf("a repair must come back as a full page, got %v", resolved["mode"])
	}
	if resolved["patch"] != nil {
		t.Error("the patch was against a page the client never held; it must not be sent")
	}
	if resolved["base_revision"] != studio.Revision(clientPage) {
		t.Error("base_revision must identify the page the CLIENT holds, not the rejected attempt")
	}
	page, _ := resolved["page"].(map[string]interface{})
	components, _ := page["components"].([]interface{})
	if len(components) != 2 {
		t.Fatalf("the repaired page should keep both components, got %d", len(components))
	}
}

// A repair fixes the root page; the pages the rejected attempt got right are not
// re-sent and must not be lost.
func TestARepairKeepsThePagesItDidNotResend(t *testing.T) {
	ctx := repairCtx(nil)
	rejected := simplePage("p1", "root1")
	carried := []*studio.NestedPage{{
		PageId:    "card1",
		Page:      simplePage("card1", "cardtext"),
		MountedAt: "root1",
	}}
	studio.RepairStateFrom(ctx).Begin(rejected, carried)

	resolved, nestedOut, err := resolveStudioOutput(ctx, map[string]interface{}{
		"mode": studio.ModePatch,
		"patch": map[string]interface{}{
			"upsert": []interface{}{map[string]interface{}{
				"id":         "root1",
				"type":       "text",
				"properties": map[string]interface{}{"base": map[string]interface{}{"label": "fixed"}},
				"styles":     map[string]interface{}{},
			}},
		},
	}, effectiveBasePage(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if len(nestedOut) != 1 {
		t.Fatalf("the carried nested page should survive the repair, got %d", len(nestedOut))
	}
	pages, _ := resolved["pages"].([]interface{})
	if len(pages) != 1 {
		t.Errorf("the manifest should still list the nested page, got %v", resolved["pages"])
	}
}
