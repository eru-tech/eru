package catalog

import "testing"

// Every issue the validator raises has to carry a code, because the retry path
// compares issues by code. An uncoded issue silently falls back to comparing
// message text, which is the behaviour codes exist to replace - so a new v.add
// that forgets its code has to fail here rather than degrade quietly in
// production.
func TestEveryValidatorIssueCarriesACode(t *testing.T) {
	broken := map[string]interface{}{
		"id":     "p1",
		"name":   "p",
		"theme":  "dark",
		"styles": map[string]interface{}{"nonsense": 1},
		"state":  []interface{}{map[string]interface{}{"nope": 1}},
		"components": []interface{}{
			"not an object",
			map[string]interface{}{"type": "text"},
			map[string]interface{}{
				"id":         "a",
				"type":       "NotAComponent",
				"properties": map[string]interface{}{"base": map[string]interface{}{}},
				"styles":     validStyles(),
			},
			map[string]interface{}{
				"id":         "b",
				"type":       "text",
				"invented":   true,
				"properties": map[string]interface{}{"base": map[string]interface{}{"no_such_property": 1}},
				"styles":     validStyles(),
				"children":   []interface{}{map[string]interface{}{"id": "c", "type": "text"}},
				"events":     []interface{}{map[string]interface{}{"action": "call-function"}},
				"validation_rules": []interface{}{
					map[string]interface{}{"type": "not_a_rule"},
				},
			},
			map[string]interface{}{
				"id":         "b",
				"type":       "text",
				"properties": map[string]interface{}{"label": "unnested"},
				"styles":     validStyles(),
			},
		},
	}

	issues := Get().ValidatePage(broken)
	if len(issues) < 10 {
		t.Fatalf("expected this page to produce a broad spread of issues, got %d:\n%s", len(issues), issueText(issues))
	}
	for _, issue := range issues {
		if issue.Code == "" {
			t.Errorf("issue has no code: %s", issue.String())
		}
	}
}

// An issue raised about something inside a component is attributed to that
// component, not just to the path it happened to sit at.
func TestIssuesAreAttributedToTheirComponent(t *testing.T) {
	p := page(component("outer", "flex_container", map[string]interface{}{
		"children": []interface{}{
			component("inner", "text", map[string]interface{}{
				"properties": map[string]interface{}{"base": map[string]interface{}{"no_such_property": 1}},
			}),
		},
	}))

	var found bool
	for _, issue := range Get().ValidatePage(p) {
		if issue.Code == CodePropertyUnknown {
			found = true
			if issue.ComponentId != "inner" {
				t.Errorf("property issue attributed to %q, want \"inner\"", issue.ComponentId)
			}
		}
	}
	if !found {
		t.Fatal("expected an unknown-property issue")
	}
}

func TestFingerprintIgnoresWordingAndPosition(t *testing.T) {
	before := Issue{
		Path:        "components[0] (id \"grid1\")",
		Code:        CodeMountBoardCardUnset,
		ComponentId: "grid1",
		Message:     "grid is in board view but has no \"card_page_id\"",
	}
	// The same complaint after a sibling was inserted above it and the message
	// was rewritten.
	after := Issue{
		Path:        "components[3] (id \"grid1\")",
		Code:        CodeMountBoardCardUnset,
		ComponentId: "grid1",
		Message:     "a board grid needs a card page - set card_page_id",
	}
	if before.Fingerprint() != after.Fingerprint() {
		t.Errorf("reworded and reindexed issue changed fingerprint:\n%s\n%s", before.Fingerprint(), after.Fingerprint())
	}

	other := after
	other.ComponentId = "grid2"
	if other.Fingerprint() == after.Fingerprint() {
		t.Error("the same complaint about a different component must not share a fingerprint")
	}

	different := after
	different.Code = CodeMountPageRefUnset
	if different.Fingerprint() == after.Fingerprint() {
		t.Error("different complaints about the same component must not share a fingerprint")
	}
}

func TestUncodedIssueStillComparesAsItself(t *testing.T) {
	a := Issue{Path: "patch", Message: "something hand-built"}
	b := Issue{Path: "patch", Message: "something hand-built"}
	c := Issue{Path: "patch", Message: "something else"}
	if a.Fingerprint() != b.Fingerprint() {
		t.Error("identical uncoded issues should match")
	}
	if a.Fingerprint() == c.Fingerprint() {
		t.Error("different uncoded issues should not match")
	}
}
