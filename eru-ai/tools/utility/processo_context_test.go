package utiltiy

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// contextResult wraps rows the way the eru-ql tool hands them back: the tool's
// own {"result": ...} envelope around the query's [{alias: [rows]}] envelope.
func contextResult(alias string, rows ...map[string]interface{}) map[string]interface{} {
	list := make([]interface{}, 0, len(rows))
	for _, row := range rows {
		list = append(list, row)
	}
	return map[string]interface{}{"result": []interface{}{map[string]interface{}{alias: list}}}
}

func TestProcessoContextRunsOneKnownQueryWithTheRequestTenant(t *testing.T) {
	delegate := &fakeEruql{result: contextResult("op", map[string]interface{}{
		"org_id":       "org-1",
		"process_id":   "proc-1",
		"process_name": "devcrm",
	})}
	tool := &ProcessoContextTool{Delegate: delegate}

	out, _, err := tool.Execute(context.Background(), "processo", "opid-9", ProcessoContextToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	if delegate.gotAction != "execute_query" {
		t.Errorf("action = %q", delegate.gotAction)
	}
	if delegate.gotParams["query_name"] != ProcessoContextQuery {
		t.Errorf("query_name = %v", delegate.gotParams["query_name"])
	}
	// The tenant comes from the execution context, so the model cannot ask for
	// another workspace's ids.
	vars, _ := delegate.gotParams["vars"].(map[string]interface{})
	if vars["org_process_id"] != "opid-9" {
		t.Errorf("org_process_id = %v, want the request tenant", vars["org_process_id"])
	}
	if out["org_process_id"] != "opid-9" {
		t.Errorf("org_process_id = %v", out["org_process_id"])
	}
	if out["org_id"] != "org-1" || out["process_id"] != "proc-1" || out["process_name"] != "devcrm" {
		t.Errorf("resolved ids = %v", out)
	}
}

// The stored query is graphql and only some of its columns are aliased, so
// process_name really does arrive under the generated name.
func TestProcessoContextReadsGeneratedColumnNames(t *testing.T) {
	delegate := &fakeEruql{result: contextResult("op", map[string]interface{}{
		"org_id":                               "org-1",
		"org_name":                             "VK Organizations",
		"owner_id":                             "owner-1",
		"process_id":                           "proc-1",
		"public___org_processes__process_id":   "proc-1",
		"public___org_processes__process_name": "devcrm",
		"rowIndex":                             1,
	})}
	tool := &ProcessoContextTool{Delegate: delegate}

	out, _, err := tool.Execute(context.Background(), "processo", "opid-9", ProcessoContextToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["process_name"] != "devcrm" {
		t.Errorf("process_name = %v", out["process_name"])
	}
	if out["org_name"] != "VK Organizations" || out["owner_id"] != "owner-1" {
		t.Errorf("optional fields = %v", out)
	}
}

func TestProcessoContextReadsSqlResultsEnvelope(t *testing.T) {
	delegate := &fakeEruql{result: contextResult("Results", map[string]interface{}{
		"org_id":       "org-2",
		"process_id":   "proc-2",
		"process_name": "propmgmt",
	})}
	tool := &ProcessoContextTool{Delegate: delegate}

	out, _, err := tool.Execute(context.Background(), "processo", "opid-2", ProcessoContextToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["process_name"] != "propmgmt" {
		t.Errorf("process_name = %v", out["process_name"])
	}
}

// A workspace that cannot be resolved has to fail loudly. Answering with blank
// ids would let the model write with an empty org_id.
func TestProcessoContextFailsWhenNoOrgProcessMatches(t *testing.T) {
	delegate := &fakeEruql{result: contextResult("op")}
	tool := &ProcessoContextTool{Delegate: delegate}

	_, _, err := tool.Execute(context.Background(), "processo", "opid-unknown", ProcessoContextToolName, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "opid-unknown") {
		t.Errorf("error should name the tenant it could not resolve: %v", err)
	}
}

func TestProcessoContextFailsOnAnIncompleteRow(t *testing.T) {
	delegate := &fakeEruql{result: contextResult("op", map[string]interface{}{
		"org_id": "org-1",
	})}
	tool := &ProcessoContextTool{Delegate: delegate}

	if _, _, err := tool.Execute(context.Background(), "processo", "opid-9", ProcessoContextToolName, nil); err == nil {
		t.Fatal("expected an error when process_id and process_name are missing")
	}
}

func TestProcessoContextFailsWithoutADelegate(t *testing.T) {
	tool := &ProcessoContextTool{}
	if _, _, err := tool.Execute(context.Background(), "processo", "opid-9", ProcessoContextToolName, nil); err == nil {
		t.Fatal("expected an error when no eru-ql tool is attached")
	}
}

func TestProcessoContextFailsWithoutATenant(t *testing.T) {
	delegate := &fakeEruql{result: contextResult("op")}
	tool := &ProcessoContextTool{Delegate: delegate}

	if _, _, err := tool.Execute(context.Background(), "processo", "  ", ProcessoContextToolName, nil); err == nil {
		t.Fatal("expected an error when the request carries no tenant")
	}
	if delegate.calls != 0 {
		t.Errorf("the query should not run without a tenant, ran %d time(s)", delegate.calls)
	}
}

func TestProcessoContextSurfacesADelegateFailure(t *testing.T) {
	delegate := &fakeEruql{err: errors.New("eru-ql unreachable")}
	tool := &ProcessoContextTool{Delegate: delegate}

	_, _, err := tool.Execute(context.Background(), "processo", "opid-9", ProcessoContextToolName, nil)
	if err == nil || !strings.Contains(err.Error(), "eru-ql unreachable") {
		t.Fatalf("the underlying failure should survive: %v", err)
	}
}

// The model has nothing to supply, and a schema with properties would invite it
// to supply something.
func TestProcessoContextTakesNoParameters(t *testing.T) {
	if props := ProcessoContextToolSchema().Properties; len(props) != 0 {
		t.Errorf("schema should declare no properties, got %v", props)
	}
	if req := ProcessoContextToolSchema().Required; len(req) != 0 {
		t.Errorf("schema should require nothing, got %v", req)
	}
}
