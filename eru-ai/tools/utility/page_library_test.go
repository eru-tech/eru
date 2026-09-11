package utiltiy

import (
	"context"
	"strings"
	"testing"

	tools "github.com/eru-tech/eru/eru-ai/tools"
)

// fakePages stands in for the configured processo tool and records the scope it
// was called with - the part the model must never be asked to supply.
type fakePages struct {
	tools.Tool
	result    map[string]interface{}
	err       error
	gotAction string
	gotParams map[string]interface{}
}

func (f *fakePages) GetActionsList() []tools.ActionInfo {
	return []tools.ActionInfo{{Name: "fetch_pages"}, {Name: "fetch_page"}}
}

func (f *fakePages) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	f.gotAction = actionName
	f.gotParams = params
	return f.result, false, f.err
}

func rows(items ...map[string]interface{}) map[string]interface{} {
	list := make([]interface{}, 0, len(items))
	for _, item := range items {
		list = append(list, item)
	}
	return map[string]interface{}{"result": map[string]interface{}{"rows": list}}
}

func library(delegate tools.Tooling) *PageLibraryTool {
	return &PageLibraryTool{
		ListDelegate: delegate,
		GetDelegate:  delegate,
		OrgId:        "org_1",
		ProcessId:    "proc_1",
	}
}

func TestListPagesReturnsWhatIsNeededToPickOne(t *testing.T) {
	delegate := &fakePages{result: rows(
		map[string]interface{}{"page_id": "inv_1", "page_name": "invoice_detail", "title": "Invoice", "route": "/invoice"},
		map[string]interface{}{"page_id": "dash_1", "page_name": "dashboard", "title": "Dashboard"},
	)}
	tool := library(delegate)

	result, _, err := tool.Execute(context.Background(), "processo", "tenant_1", ListPagesToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	if delegate.gotAction != "fetch_pages" {
		t.Errorf("action = %q", delegate.gotAction)
	}
	// The org and process come from the agent, never from the model.
	if delegate.gotParams["org_id"] != "org_1" || delegate.gotParams["process_id"] != "proc_1" {
		t.Errorf("scope = %v", delegate.gotParams)
	}

	pages, _ := result["pages"].([]pageSummary)
	if len(pages) != 2 {
		t.Fatalf("pages = %v", result["pages"])
	}
	if pages[0].Name != "dashboard" {
		t.Errorf("pages are not in a stable order: %v", pages)
	}
	if result["count"] != 2 {
		t.Errorf("count = %v", result["count"])
	}
}

func TestListPagesNarrowsByWhatTheUserCalledIt(t *testing.T) {
	delegate := &fakePages{result: rows(
		map[string]interface{}{"page_id": "inv_1", "page_name": "invoice_detail"},
		map[string]interface{}{"page_id": "dash_1", "page_name": "dashboard"},
	)}
	tool := library(delegate)

	result, _, err := tool.Execute(context.Background(), "processo", "t", ListPagesToolName, map[string]interface{}{"name_contains": "INVOICE"})
	if err != nil {
		t.Fatal(err)
	}
	pages, _ := result["pages"].([]pageSummary)
	if len(pages) != 1 || pages[0].PageId != "inv_1" {
		t.Errorf("filtering by name gave %v", pages)
	}

	// A filter that matches nothing says so, instead of looking like an empty
	// workspace.
	empty, _, _ := tool.Execute(context.Background(), "processo", "t", ListPagesToolName, map[string]interface{}{"name_contains": "nope"})
	if note, _ := empty["note"].(string); !strings.Contains(note, "no page name or id contains") {
		t.Errorf("note = %q", note)
	}
}

func TestListPagesReadsTheNameOutOfANestedDefinition(t *testing.T) {
	delegate := &fakePages{result: rows(
		map[string]interface{}{"page_id": "p1", "page_def": map[string]interface{}{"name": "invoice_detail", "title": "Invoice"}},
	)}
	result, _, err := library(delegate).Execute(context.Background(), "processo", "t", ListPagesToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	pages, _ := result["pages"].([]pageSummary)
	if len(pages) != 1 || pages[0].Name != "invoice_detail" || pages[0].Title != "Invoice" {
		t.Errorf("pages = %v", pages)
	}
}

const referencePage = `{"id":"inv_1","name":"invoice_detail","title":"Invoice","styles":{},
  "components":[{"id":"root","type":"flex_container","properties":{"base":{}},
    "children":[{"id":"hdr","type":"text","properties":{"base":{"label":"Invoice"}}}]}]}`

func TestGetPageReturnsThePageAndSaysItIsAReference(t *testing.T) {
	delegate := &fakePages{result: rows(map[string]interface{}{"page_id": "inv_1", "page_def": referencePage})}
	tool := library(delegate)

	result, _, err := tool.Execute(context.Background(), "processo", "t", GetPageToolName, map[string]interface{}{"page_id": "inv_1"})
	if err != nil {
		t.Fatal(err)
	}
	if delegate.gotAction != "fetch_page" || delegate.gotParams["page_id"] != "inv_1" {
		t.Errorf("delegate call = %s %v", delegate.gotAction, delegate.gotParams)
	}
	page, ok := result["page"].(map[string]interface{})
	if !ok {
		t.Fatalf("no page in %v", result)
	}
	if page["name"] != "invoice_detail" {
		t.Errorf("page = %v", page)
	}
	// The model has to be told this is not its answer, or it returns it.
	note, _ := result["note"].(string)
	for _, want := range []string{"REFERENCE", "never return it", "component ids"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note does not warn about %q: %q", want, note)
		}
	}
}

func TestGetPageNeedsAnId(t *testing.T) {
	tool := library(&fakePages{result: rows()})
	if _, _, err := tool.Execute(context.Background(), "processo", "t", GetPageToolName, nil); err == nil {
		t.Fatal("get_page ran without a page id")
	} else if !strings.Contains(err.Error(), "list_pages") {
		t.Errorf("the error does not say how to find the id: %v", err)
	}
}

func TestGetPageReportsAnIdThatFindsNothing(t *testing.T) {
	tool := library(&fakePages{result: rows()})
	result, _, err := tool.Execute(context.Background(), "processo", "t", GetPageToolName, map[string]interface{}{"page_id": "ghost"})
	if err != nil {
		t.Fatal(err)
	}
	if note, _ := result["note"].(string); !strings.Contains(note, "returned nothing") {
		t.Errorf("note = %q", note)
	}
}

func TestGetPageFallsBackToStructureWhenThePageIsHuge(t *testing.T) {
	// A reference page is read for its patterns; a 120KB page would crowd out the
	// page being built.
	big := strings.Repeat("x", MaxReferencePageBytes)
	page := map[string]interface{}{
		"id": "big_1", "name": "big_page", "filler": big,
		"components": []interface{}{
			map[string]interface{}{"id": "root", "type": "flex_container", "properties": map[string]interface{}{"base": map[string]interface{}{}},
				"children": []interface{}{
					map[string]interface{}{"id": "inner", "type": "grid", "properties": map[string]interface{}{"base": map[string]interface{}{}}},
				}},
		},
	}
	delegate := &fakePages{result: rows(map[string]interface{}{"page_def": page})}

	result, _, err := library(delegate).Execute(context.Background(), "processo", "t", GetPageToolName, map[string]interface{}{"page_id": "big_1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, carriesPage := result["page"]; carriesPage {
		t.Error("a huge page was returned whole")
	}
	structure, ok := result["structure"].(map[string]interface{})
	if !ok {
		t.Fatalf("no structure in %v", result)
	}
	if structure["name"] != "big_page" {
		t.Errorf("structure = %v", structure)
	}
	components, _ := structure["components"].([]interface{})
	root, _ := components[0].(map[string]interface{})
	if root["type"] != "flex_container" {
		t.Errorf("the component tree was lost: %v", root)
	}
	if _, hasProps := root["properties"]; hasProps {
		t.Error("the structure kept properties - it should be ids and types only")
	}
	children, _ := root["children"].([]interface{})
	if len(children) != 1 {
		t.Errorf("nesting was lost: %v", root)
	}
	if note, _ := result["note"].(string); !strings.Contains(note, "too large") {
		t.Errorf("note = %q", note)
	}
}

func TestPageLibraryFailsClearlyWithNoDelegate(t *testing.T) {
	tool := &PageLibraryTool{}
	if _, _, err := tool.Execute(context.Background(), "processo", "t", ListPagesToolName, nil); err == nil ||
		!strings.Contains(err.Error(), "fetch_pages") {
		t.Errorf("list_pages error = %v", err)
	}
	if _, _, err := tool.Execute(context.Background(), "processo", "t", GetPageToolName, map[string]interface{}{"page_id": "x"}); err == nil ||
		!strings.Contains(err.Error(), "fetch_page") {
		t.Errorf("get_page error = %v", err)
	}
	if _, _, err := tool.Execute(context.Background(), "processo", "t", "nonsense", nil); err == nil {
		t.Error("an unknown action was accepted")
	}
}

func TestFirstPageDefinitionFindsThePageWhereverItIs(t *testing.T) {
	cases := []map[string]interface{}{
		rows(map[string]interface{}{"page_def": map[string]interface{}{"id": "p", "components": []interface{}{}}}),
		rows(map[string]interface{}{"page_def": referencePage}),
		rows(map[string]interface{}{"page": map[string]interface{}{"id": "p", "components": []interface{}{}}}),
		rows(map[string]interface{}{"definition": map[string]interface{}{"id": "p", "components": []interface{}{}}}),
		rows(map[string]interface{}{"id": "p", "components": []interface{}{}}),
	}
	for i, envelope := range cases {
		if page := firstPageDefinition(envelope); page == nil {
			t.Errorf("envelope %d yielded no page", i)
		}
	}
	if page := firstPageDefinition(rows(map[string]interface{}{"unrelated": 1})); page != nil {
		t.Errorf("a row with no page yielded %v", page)
	}
}
