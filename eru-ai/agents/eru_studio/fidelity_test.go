package eru_studio

import (
	"encoding/json"
	"reflect"
	"testing"
)

func pageFrom(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var page map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return page
}

// The shape the real drift took: a priority control lost the icon on one of its
// options, on a request that only touched the title.
const fidelityBase = `{"components":[{"id":"root","type":"flex_container","children":[
  {"id":"tb-title","type":"text","styles":{"responsive_styles":{"base":{"color":"#111827"}}}},
  {"id":"tb-pri","type":"priority","properties":{"base":{"options":[
      {"label":"Urgent","color":"#EF6C6C","icon":"priority_high"},
      {"label":"Low","color":"#6B7280"}]}}},
  {"id":"tb-tog","type":"slide_toggle","properties":{"base":{"events":[],"name":"","label":""}}}
]}]}`

const fidelityDrifted = `{"components":[{"id":"root","type":"flex_container","children":[
  {"id":"tb-title","type":"text","styles":{"responsive_styles":{"base":{"color":"#1f2937","background_color":"#F59E0B"}}}},
  {"id":"tb-pri","type":"priority","properties":{"base":{"options":[
      {"label":"Urgent","color":"#EF6C6C"},
      {"label":"Low","color":"#6B7280"}]}}},
  {"id":"tb-tog","type":"slide_toggle","properties":{"base":{"label":""}}}
]}]}`

func fidelityComponent(page map[string]interface{}, id string) map[string]interface{} {
	return indexComponentsById(page)[id]
}

func TestALeafLostFromAnUntouchedComponentIsPutBack(t *testing.T) {
	base := pageFrom(t, fidelityBase)
	live := pageFrom(t, fidelityDrifted)

	restored := RestoreInheritedLeaves(base, live, nil)
	if len(restored) == 0 {
		t.Fatal("the dropped leaves were not noticed")
	}

	pri := fidelityComponent(live, "tb-pri")
	options := pri["properties"].(map[string]interface{})["base"].(map[string]interface{})["options"].([]interface{})
	urgent := options[0].(map[string]interface{})
	if urgent["icon"] != "priority_high" {
		t.Fatalf("the icon inside the options array was not restored: %v", urgent)
	}

	tog := fidelityComponent(live, "tb-tog")
	togBase := tog["properties"].(map[string]interface{})["base"].(map[string]interface{})
	if _, ok := togBase["events"]; !ok {
		t.Fatal("events was not restored")
	}
	if _, ok := togBase["name"]; !ok {
		t.Fatal("name was not restored")
	}
}

func TestTheComponentTheEditWasWorkingOnIsLeftAlone(t *testing.T) {
	base := pageFrom(t, fidelityBase)
	live := pageFrom(t, fidelityDrifted)

	RestoreInheritedLeaves(base, live, nil)

	title := fidelityComponent(live, "tb-title")
	styles := title["styles"].(map[string]interface{})["responsive_styles"].(map[string]interface{})["base"].(map[string]interface{})
	if styles["color"] != "#1f2937" {
		t.Fatalf("the edit's own change was reverted: %v", styles)
	}
	if styles["background_color"] != "#F59E0B" {
		t.Fatalf("the edit's own addition was removed: %v", styles)
	}
}

func TestAScopedComponentMayRemoveThings(t *testing.T) {
	// A scoped edit named this component, so a removal on it was asked for.
	base := pageFrom(t, fidelityBase)
	live := pageFrom(t, fidelityDrifted)

	restored := RestoreInheritedLeaves(base, live, map[string]bool{"tb-pri": true, "tb-tog": true})
	if len(restored) != 0 {
		t.Fatalf("a scoped removal was undone: %v", restored)
	}
	pri := fidelityComponent(live, "tb-pri")
	options := pri["properties"].(map[string]interface{})["base"].(map[string]interface{})["options"].([]interface{})
	if _, ok := options[0].(map[string]interface{})["icon"]; ok {
		t.Fatal("the icon came back on a component the edit was licensed to change")
	}
}

func TestAFaithfulEchoIsNotTouched(t *testing.T) {
	base := pageFrom(t, fidelityBase)
	live := pageFrom(t, fidelityBase)
	before, _ := json.Marshal(live)

	if restored := RestoreInheritedLeaves(base, live, nil); len(restored) != 0 {
		t.Fatalf("restored something on an identical page: %v", restored)
	}
	after, _ := json.Marshal(live)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("an unchanged page was modified")
	}
}

func TestADeletedComponentIsNotResurrected(t *testing.T) {
	base := pageFrom(t, fidelityBase)
	live := pageFrom(t, `{"components":[{"id":"root","type":"flex_container","children":[
	  {"id":"tb-title","type":"text","styles":{"responsive_styles":{"base":{"color":"#111827"}}}}
	]}]}`)

	RestoreInheritedLeaves(base, live, nil)
	if fidelityComponent(live, "tb-pri") != nil {
		t.Fatal("a deleted component was put back - deletion is a structural change, not a lost leaf")
	}
}

func TestARewrittenListIsLeftToTheEdit(t *testing.T) {
	// A list of a different length was rewritten; pairing its entries back up
	// would be guesswork, so nothing is restored into it.
	base := pageFrom(t, `{"components":[{"id":"c","type":"tag","properties":{"base":{"options":[
	  {"label":"A","icon":"a"},{"label":"B","icon":"b"}]}}}]}`)
	live := pageFrom(t, `{"components":[{"id":"c","type":"tag","properties":{"base":{"options":[
	  {"label":"A"}]}}}]}`)

	if restored := RestoreInheritedLeaves(base, live, nil); len(restored) != 0 {
		t.Fatalf("guessed at a rewritten list: %v", restored)
	}
}

func TestRestoredValuesDoNotAliasTheBasePage(t *testing.T) {
	base := pageFrom(t, `{"components":[{"id":"c","type":"tag","properties":{"base":{"opts":{"k":"v"}}}}]}`)
	live := pageFrom(t, `{"components":[{"id":"c","type":"tag","properties":{"base":{}}}]}`)

	RestoreInheritedLeaves(base, live, nil)

	restoredOpts := fidelityComponent(live, "c")["properties"].(map[string]interface{})["base"].(map[string]interface{})["opts"].(map[string]interface{})
	restoredOpts["k"] = "mutated"

	baseOpts := fidelityComponent(base, "c")["properties"].(map[string]interface{})["base"].(map[string]interface{})["opts"].(map[string]interface{})
	if baseOpts["k"] != "v" {
		t.Fatal("the restored value aliases the base page, so editing the result corrupts the baseline")
	}
}
