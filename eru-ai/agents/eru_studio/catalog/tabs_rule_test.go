package catalog

import (
	"strings"
	"testing"
)

func tabsComponent(id string, titles interface{}, children []interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"id":         id,
		"type":       "tabs",
		"properties": map[string]interface{}{"base": map[string]interface{}{"tabs": titles}},
		"styles":     validStyles(),
	}
	if children != nil {
		out["children"] = children
	}
	return out
}

func panel(id string) map[string]interface{} {
	return component(id, "flex_container", map[string]interface{}{"children": []interface{}{}})
}

func tabsCodes(issues []Issue) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		out = append(out, string(i.Code))
	}
	return out
}

// Exactly the shape the agent produced: two tabs, panels named after their
// contents instead of carrying the tabs component's id prefix.
func TestPanelsNamedAfterTheirContentsAreReported(t *testing.T) {
	p := page(tabsComponent("detail_tabs_section", "Invoices,Financed Invoices", []interface{}{
		panel("tab_invoices_content"),
		panel("tab_finv_content"),
	}))
	issues := Get().ValidatePage(p)
	var notPanel, count int
	for _, issue := range issues {
		switch issue.Code {
		case CodeTabsChildNotAPanel:
			notPanel++
			if !strings.Contains(issue.Message, "detail_tabs_section-tab-0") {
				t.Errorf("the message should name the id to use: %s", issue.Message)
			}
		case CodeTabsPanelCount:
			count++
		}
	}
	if notPanel != 2 {
		t.Errorf("both mis-named panels should be reported, got %d (%v)", notPanel, tabsCodes(issues))
	}
	if count != 1 {
		t.Errorf("the missing-panel count should be reported once, got %d", count)
	}
}

func TestCorrectlyPrefixedPanelsAreAccepted(t *testing.T) {
	p := page(tabsComponent("detail_tabs_section", "Invoices,Financed Invoices", []interface{}{
		panel("detail_tabs_section-tab-0"),
		panel("detail_tabs_section-tab-1"),
	}))
	for _, issue := range Get().ValidatePage(p) {
		if issue.Code == CodeTabsChildNotAPanel || issue.Code == CodeTabsPanelCount {
			t.Errorf("a correct tabs component was reported: %s", issue.String())
		}
	}
}

// Four children for two tabs - two orphans plus two auto-created empties - is
// the state the user actually saw.
func TestTooManyPanelsAreReported(t *testing.T) {
	p := page(tabsComponent("t", "One,Two", []interface{}{
		panel("t-tab-0"), panel("t-tab-1"), panel("t-tab-2"),
	}))
	found := false
	for _, issue := range Get().ValidatePage(p) {
		if issue.Code == CodeTabsPanelCount {
			found = true
		}
	}
	if !found {
		t.Error("three panels for two tabs should be reported")
	}
}

func TestTabsWithNoChildrenAtAllAreReported(t *testing.T) {
	p := page(tabsComponent("t", "One,Two", nil))
	found := false
	for _, issue := range Get().ValidatePage(p) {
		if issue.Code == CodeTabsPanelCount {
			found = true
			if !strings.Contains(issue.Message, "t-tab-0") {
				t.Errorf("the message should name the ids to create: %s", issue.Message)
			}
		}
	}
	if !found {
		t.Error("tabs with no children should be reported")
	}
}

// The list may arrive as an array rather than a comma-separated string.
func TestTabTitlesMayBeAnArray(t *testing.T) {
	p := page(tabsComponent("t", []interface{}{"One", "Two"}, []interface{}{panel("t-tab-0")}))
	found := false
	for _, issue := range Get().ValidatePage(p) {
		if issue.Code == CodeTabsPanelCount {
			found = true
		}
	}
	if !found {
		t.Error("an array of titles should be counted the same way")
	}
}

// A patch that does not restate children is not rewriting them.
func TestAPartialTabsComponentIsNotHeldToItsChildren(t *testing.T) {
	upsert := []interface{}{tabsComponent("t", "One,Two", nil)}
	for _, issue := range Get().ValidateComponents(upsert, "patch.upsert") {
		if issue.Code == CodeTabsPanelCount {
			t.Errorf("a patch that only renames tabs should not be failed for children it did not send: %s", issue.String())
		}
	}
}
