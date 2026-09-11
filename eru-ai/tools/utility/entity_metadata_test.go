package utiltiy

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

func init() {
	logs.LogInit("test", "entity-metadata-test")
}

// fakeEruql stands in for the configured eru-ql tool and records what it was
// asked to run, which is the part that has to be exactly right.
type fakeEruql struct {
	tools.Tool
	result     map[string]interface{}
	err        error
	gotAction  string
	gotParams  map[string]interface{}
	gotTenant  string
	gotProject string
	calls      int
}

func (f *fakeEruql) GetActionsList() []tools.ActionInfo {
	return []tools.ActionInfo{{Name: "execute_query"}, {Name: "execute_sql"}}
}

func (f *fakeEruql) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	f.calls++
	f.gotAction = actionName
	f.gotParams = params
	f.gotTenant = tenantId
	f.gotProject = projectId
	return f.result, false, f.err
}

func metadataResult(rows ...map[string]interface{}) map[string]interface{} {
	list := make([]interface{}, 0, len(rows))
	for _, row := range rows {
		list = append(list, row)
	}
	return map[string]interface{}{"fetch_entity_table_column_metadata": map[string]interface{}{"rows": list}}
}

func TestEntityMetadataRunsExactlyOneKnownQuery(t *testing.T) {
	delegate := &fakeEruql{result: metadataResult(
		map[string]interface{}{"entity_name": "invoice", "field_name": "invoice_no", "display_name": "Invoice Number", "data_type": "text", "is_mandatory": true},
	)}
	tool := &EntityMetadataTool{Delegate: delegate}

	result, _, err := tool.Execute(context.Background(), "processo", "tenant_42", EntityMetadataToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	if delegate.gotAction != "execute_query" {
		t.Errorf("action = %q", delegate.gotAction)
	}
	if delegate.gotParams["query_name"] != EntityMetadataQuery {
		t.Errorf("query_name = %v", delegate.gotParams["query_name"])
	}
	// The tenant comes from the execution context, so the model cannot ask for
	// another tenant's entities.
	vars, _ := delegate.gotParams["vars"].(map[string]interface{})
	if vars["org_process_id"] != "tenant_42" {
		t.Errorf("org_process_id = %v, want the tenant from the execution context", vars["org_process_id"])
	}
	if delegate.gotProject != "processo" {
		t.Errorf("project = %q", delegate.gotProject)
	}

	entities, _ := result["entities"].([]metadataEntity)
	if len(entities) != 1 || entities[0].Name != "invoice" {
		t.Fatalf("entities = %+v", result["entities"])
	}
	field := entities[0].Fields[0]
	if field.Name != "invoice_no" || field.Label != "Invoice Number" || field.Type != "text" || !field.Mandatory {
		t.Errorf("field = %+v", field)
	}
}

func TestEntityMetadataIgnoresAModelSuppliedTenant(t *testing.T) {
	delegate := &fakeEruql{result: metadataResult(map[string]interface{}{"entity_name": "invoice", "field_name": "id"})}
	tool := &EntityMetadataTool{Delegate: delegate}

	_, _, err := tool.Execute(context.Background(), "processo", "tenant_42", EntityMetadataToolName, map[string]interface{}{
		"vars":       map[string]interface{}{"org_process_id": "someone_elses_tenant"},
		"query_name": "drop_everything",
	})
	if err != nil {
		t.Fatal(err)
	}
	if delegate.gotParams["query_name"] != EntityMetadataQuery {
		t.Errorf("the model chose the query: %v", delegate.gotParams["query_name"])
	}
	vars, _ := delegate.gotParams["vars"].(map[string]interface{})
	if vars["org_process_id"] != "tenant_42" {
		t.Errorf("the model chose the tenant: %v", vars["org_process_id"])
	}
}

func TestEntityMetadataGroupsFieldsByEntity(t *testing.T) {
	delegate := &fakeEruql{result: metadataResult(
		map[string]interface{}{"entity_name": "invoice", "field_name": "invoice_no", "display_name": "Invoice Number"},
		map[string]interface{}{"entity_name": "invoice", "field_name": "amount", "display_name": "Amount"},
		map[string]interface{}{"entity_name": "customer", "field_name": "name", "display_name": "Customer Name"},
	)}
	tool := &EntityMetadataTool{Delegate: delegate}

	result, _, err := tool.Execute(context.Background(), "processo", "t", EntityMetadataToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	entities, _ := result["entities"].([]metadataEntity)
	if len(entities) != 2 {
		t.Fatalf("expected two entities, got %+v", entities)
	}
	if entities[0].Name != "invoice" || len(entities[0].Fields) != 2 {
		t.Errorf("invoice = %+v", entities[0])
	}
	if result["field_count"] != 3 {
		t.Errorf("field_count = %v", result["field_count"])
	}
}

func TestEntityMetadataFiltersByEntityName(t *testing.T) {
	delegate := &fakeEruql{result: metadataResult(
		map[string]interface{}{"entity_name": "invoice", "field_name": "invoice_no"},
		map[string]interface{}{"entity_name": "customer", "field_name": "name"},
	)}
	tool := &EntityMetadataTool{Delegate: delegate}

	for _, filter := range []interface{}{
		[]interface{}{"customer"},
		"customer",
		[]string{"CUSTOMER"},
	} {
		result, _, err := tool.Execute(context.Background(), "processo", "t", EntityMetadataToolName, map[string]interface{}{"entity_names": filter})
		if err != nil {
			t.Fatal(err)
		}
		entities, _ := result["entities"].([]metadataEntity)
		if len(entities) != 1 || entities[0].Name != "customer" {
			t.Errorf("filter %v gave %+v", filter, entities)
		}
	}

	// A filter that matches nothing says so, rather than looking like an empty
	// tenant.
	result, _, err := tool.Execute(context.Background(), "processo", "t", EntityMetadataToolName, map[string]interface{}{"entity_names": []interface{}{"nope"}})
	if err != nil {
		t.Fatal(err)
	}
	note, _ := result["note"].(string)
	if !strings.Contains(note, "no entity matched") {
		t.Errorf("note = %q", note)
	}
}

func TestEntityMetadataFindsRowsInAnyEnvelope(t *testing.T) {
	row := map[string]interface{}{"entity_name": "invoice", "field_name": "id"}
	envelopes := []map[string]interface{}{
		{"rows": []interface{}{row}},
		{"data": []interface{}{row}},
		{"fetch_entity_table_column_metadata": []interface{}{row}},
		{"fetch_entity_table_column_metadata": map[string]interface{}{"rows": []interface{}{row}}},
		{"result": map[string]interface{}{"records": []interface{}{row}}},
	}
	for i, envelope := range envelopes {
		if rows := extractRows(envelope); len(rows) != 1 {
			encoded, _ := json.Marshal(envelope)
			t.Errorf("envelope %d (%s) yielded %d rows", i, encoded, len(rows))
		}
	}
	if rows := extractRows(nil); rows != nil {
		t.Error("a nil result yielded rows")
	}
	if rows := extractRows(map[string]interface{}{"count": float64(0)}); len(rows) != 0 {
		t.Error("a result with no rows yielded rows")
	}
}

func TestEntityMetadataToleratesAlternativeColumnNames(t *testing.T) {
	// The stored query owns its column names; a rename should degrade, not break.
	delegate := &fakeEruql{result: metadataResult(
		map[string]interface{}{"table_name": "invoice", "column_name": "invoice_no", "label": "Invoice No", "datatype": "varchar", "mandatory": "Y"},
	)}
	tool := &EntityMetadataTool{Delegate: delegate}

	result, _, err := tool.Execute(context.Background(), "processo", "t", EntityMetadataToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	entities, _ := result["entities"].([]metadataEntity)
	if len(entities) != 1 || entities[0].Name != "invoice" {
		t.Fatalf("entities = %+v", entities)
	}
	field := entities[0].Fields[0]
	if field.Name != "invoice_no" || field.Label != "Invoice No" || field.Type != "varchar" || !field.Mandatory {
		t.Errorf("field = %+v", field)
	}
}

func TestEntityMetadataIsCapped(t *testing.T) {
	rows := make([]map[string]interface{}, 0, MaxEntityMetadataFields*2)
	for i := 0; i < MaxEntityMetadataFields*2; i++ {
		rows = append(rows, map[string]interface{}{"entity_name": "big", "field_name": "f"})
	}
	delegate := &fakeEruql{result: metadataResult(rows...)}
	tool := &EntityMetadataTool{Delegate: delegate}

	result, _, err := tool.Execute(context.Background(), "processo", "t", EntityMetadataToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result["field_count"] != MaxEntityMetadataFields {
		t.Errorf("field_count = %v, want the cap", result["field_count"])
	}
	if note, _ := result["note"].(string); !strings.Contains(note, "truncated") {
		t.Errorf("a truncated answer does not say so: %q", note)
	}
}

func TestEntityMetadataReportsAnEmptyTenantUsefully(t *testing.T) {
	delegate := &fakeEruql{result: map[string]interface{}{"rows": []interface{}{}}}
	tool := &EntityMetadataTool{Delegate: delegate}

	result, _, err := tool.Execute(context.Background(), "processo", "t", EntityMetadataToolName, nil)
	if err != nil {
		t.Fatal(err)
	}
	if note, _ := result["note"].(string); !strings.Contains(note, "no entity metadata") {
		t.Errorf("note = %q - the model needs to be told to fall back to the user's words", note)
	}
}

func TestEntityMetadataFailsClearlyWithNoDelegate(t *testing.T) {
	tool := &EntityMetadataTool{}
	_, _, err := tool.Execute(context.Background(), "processo", "t", EntityMetadataToolName, nil)
	if err == nil {
		t.Fatal("a tool with no eru-ql delegate ran anyway")
	}
	if !strings.Contains(err.Error(), "no eru-ql tool is attached") {
		t.Errorf("error = %v", err)
	}
}

func TestIsEruqlToolIdentifiesTheDelegate(t *testing.T) {
	if !IsEruqlTool(context.Background(), &fakeEruql{}) {
		t.Error("a tool with execute_query was not recognised")
	}
	if IsEruqlTool(context.Background(), nil) {
		t.Error("nil was recognised as an eru-ql tool")
	}
	if IsEruqlTool(context.Background(), &ComponentSpecTool{}) {
		t.Error("the component spec tool was recognised as an eru-ql tool")
	}
}
