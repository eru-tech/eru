package utiltiy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// realMetadataFixture is the answer this query gives for an actual tenant with
// 61 entities, captured from the running service. The tool once read it as zero
// entities, and that is the regression these tests exist for.
//
// It describes a customer's data model - entity names, field labels, what each
// one is for - so it is not ours to commit. It lives in the module's ignored
// folder, and a checkout without it skips rather than fails: the tests that can
// run on the committed sample below still do.
const realMetadataFixture = "../../local-testdata/entity_metadata_tables.json"

// sampleMetadataFixture is a synthetic stand-in with the same shape and invented
// names, so the parsing this tool depends on is covered everywhere, by everyone.
const sampleMetadataFixture = "testdata/entity_metadata_sample.json"

func realMetadataResult(t *testing.T) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(realMetadataFixture)
	if os.IsNotExist(err) {
		t.Skipf("%s is not present - it is captured from a live tenant and never committed", realMetadataFixture)
	}
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return decodeMetadata(t, raw)
}

func sampleMetadataResult(t *testing.T) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(sampleMetadataFixture)
	if err != nil {
		t.Fatalf("reading sample fixture: %v", err)
	}
	return decodeMetadata(t, raw)
}

func decodeMetadata(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("parsing fixture: %v", err)
	}
	return result
}

func TestTablesShapeIsReadAsEntities(t *testing.T) {
	entities, total, _, ok := groupTables(realMetadataResult(t), nil)
	if !ok {
		t.Fatal("the tables shape was not recognised")
	}
	if len(entities) < 50 {
		t.Fatalf("expected the tenant's many entities, got %d", len(entities))
	}
	if total == 0 {
		t.Fatal("no fields were read")
	}

	var financiers *metadataEntity
	for i := range entities {
		if entities[i].Name == "fn" {
			financiers = &entities[i]
		}
	}
	if financiers == nil {
		t.Fatal("the financier entity is in the tenant's metadata but was not returned")
	}
	if financiers.Label != "Financiers" {
		t.Errorf("label = %q, want \"Financiers\"", financiers.Label)
	}
	// A page binds to the entity name; only SQL needs the physical table. The
	// two are different, and deriving either from the other is guesswork.
	if financiers.Table != "scf_fn" {
		t.Errorf("table = %q, want \"scf_fn\"", financiers.Table)
	}

	byName := map[string]metadataField{}
	for _, field := range financiers.Fields {
		byName[field.Name] = field
	}
	name, ok := byName["fn"]
	if !ok {
		t.Fatalf("the financier entity should have the field \"fn\", got %v", financiers.Fields)
	}
	if name.Label != "Financier Name" {
		t.Errorf("fn label = %q, want \"Financier Name\"", name.Label)
	}
	if limit, ok := byName["fl"]; !ok || limit.Type != "numeric" {
		t.Errorf("fl should be numeric, got %+v", limit)
	}
}

// The storage columns are not fields anyone binds a form to.
func TestStorageColumnsAreNotReportedAsFields(t *testing.T) {
	entities, _, _, _ := groupTables(realMetadataResult(t), nil)
	for _, entity := range entities {
		for _, field := range entity.Fields {
			if field.Name == entityIdColumn || field.Name == entityDataColumn {
				t.Fatalf("%s reported the storage column %q as a field", entity.Name, field.Name)
			}
		}
	}
}

// A caller may know the entity ("fn"), the physical table ("scf_fn") or only
// the words the user used ("financiers"). All three have to land on it.
func TestEntityFilterFindsItByEitherNameOrLabel(t *testing.T) {
	for _, asked := range []string{"fn", "scf_fn", "financiers"} {
		entities, _, _, _ := groupTables(realMetadataResult(t), []string{asked})
		found := false
		for _, entity := range entities {
			if entity.Name == "fn" {
				found = true
			}
		}
		if !found {
			t.Errorf("asking for %q did not find the financier entity", asked)
		}
	}
}

// The entity name and the table name are genuinely different values, and the
// table is the process name joined to the entity name. If this ever collapses
// to one field, a page binds to a table or a query reads a non-existent one.
func TestEntityNameAndTableNameAreBothCarried(t *testing.T) {
	entities, _, _, _ := groupTables(realMetadataResult(t), nil)
	checked := 0
	for _, entity := range entities {
		if entity.Table == "" {
			continue
		}
		checked++
		if entity.Table == entity.Name {
			t.Errorf("%s: table and name are the same value", entity.Name)
		}
		if !strings.HasSuffix(entity.Table, "_"+entity.Name) {
			t.Errorf("%s: table %q is not the process name joined to the entity name", entity.Name, entity.Table)
		}
	}
	if checked == 0 {
		t.Fatal("no entity carried a table name")
	}
}

func TestFieldCountIsCapped(t *testing.T) {
	_, total, _, _ := groupTables(realMetadataResult(t), nil)
	if total > MaxEntityMetadataFields {
		t.Errorf("field count %d exceeds the cap %d", total, MaxEntityMetadataFields)
	}
}

// A result that is not in this shape must fall through to the flat-row path
// rather than report an empty tenant.
func TestAnUnknownShapeIsDeclined(t *testing.T) {
	if _, _, _, ok := groupTables(map[string]interface{}{
		"rows": []interface{}{map[string]interface{}{"entity_name": "x", "field_name": "y"}},
	}, nil); ok {
		t.Error("a flat-row result should not be claimed by the tables parser")
	}
}

// A tenant with more fields than fit must still have every entity named. A
// model that asks "what entities are there" and gets a truncated list concludes
// the missing ones do not exist.
func TestEveryEntityIsNamedEvenWhenFieldsAreTruncated(t *testing.T) {
	entities, total, truncated, _ := groupTables(realMetadataResult(t), nil)
	if !truncated {
		t.Skip("this fixture fits inside the cap")
	}
	if len(entities) < 61 {
		t.Errorf("only %d of the tenant's entities were named; truncation dropped the rest", len(entities))
	}
	omitted := 0
	for _, entity := range entities {
		if entity.FieldsOmitted > 0 {
			omitted++
			if len(entity.Fields) != 0 {
				t.Errorf("%s reports omitted fields and also carries some", entity.Name)
			}
		}
	}
	if omitted == 0 {
		t.Error("truncation was reported but no entity says its fields were left out")
	}
	if total > MaxEntityMetadataFields {
		t.Errorf("field budget exceeded: %d", total)
	}
	t.Logf("%d entities named, %d carried fields, %d deferred, %d fields total", len(entities), len(entities)-omitted, omitted, total)
}

// Asking by name must return that entity's fields even when an unfiltered call
// would have deferred them.
func TestAskingForAnEntityReturnsItsFields(t *testing.T) {
	entities, _, _, _ := groupTables(realMetadataResult(t), []string{"fn"})
	if len(entities) == 0 {
		t.Fatal("the financier entity was not returned")
	}
	for _, entity := range entities {
		if entity.Name == "fn" {
			if len(entity.Fields) == 0 {
				t.Fatal("it came back with no fields even though it was asked for by name")
			}
			return
		}
	}
	t.Fatal("the financier entity is missing")
}

// The tests below run on the committed sample, so the parsing this tool depends
// on is covered on any checkout - including one that has never seen a tenant.

func TestSampleTablesShapeIsReadAsEntities(t *testing.T) {
	entities, total, _, ok := groupTables(sampleMetadataResult(t), nil)
	if !ok {
		t.Fatal("the tables shape was not recognised")
	}
	if len(entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(entities))
	}
	if total == 0 {
		t.Fatal("no fields were counted")
	}
}

func TestSampleEntityIsNamedByEntityNameNotTableName(t *testing.T) {
	// table_name is what SQL needs; entity_name is what a page binds to. Reading
	// the wrong one is what bound generated pages to "scf_fn" instead of "fn".
	entities, _, _, _ := groupTables(sampleMetadataResult(t), nil)
	byName := map[string]bool{}
	for _, e := range entities {
		byName[e.Name] = true
	}
	if !byName["widget"] {
		t.Fatalf("entity_name was not used as the name: %v", byName)
	}
	if byName["demo_widget"] {
		t.Fatal("the physical table name was used as the entity name")
	}
}

func TestSampleFallsBackToTableNameWhenEntityNameIsAbsent(t *testing.T) {
	// Older results carry only the table name, and dropping those rows would
	// silently hide entities that do exist.
	entities, _, _, _ := groupTables(sampleMetadataResult(t), nil)
	found := false
	for _, e := range entities {
		if e.Name == "demo_legacy" {
			found = true
		}
	}
	if !found {
		t.Fatal("an entity carrying only a table name was dropped")
	}
}

func TestSampleFilteringByNameKeepsOnlyWhatWasAsked(t *testing.T) {
	entities, _, _, _ := groupTables(sampleMetadataResult(t), []string{"gadget"})
	if len(entities) != 1 || entities[0].Name != "gadget" {
		t.Fatalf("asking for one entity returned %d: %+v", len(entities), entities)
	}
}

func TestEveryEntityIsNamedEvenWhenTheFieldBudgetRunsOut(t *testing.T) {
	// Built here rather than committed: a fixture wide enough to trip the cap is
	// mostly filler, and filler does not belong in the repository.
	wide := map[string]interface{}{"Results": []interface{}{map[string]interface{}{
		"tables": buildWideTables(MaxEntityMetadataFields + 100),
	}}}

	entities, total, truncated, ok := groupTables(wide, nil)
	if !ok {
		t.Fatal("the tables shape was not recognised")
	}
	if !truncated {
		t.Fatal("a result past the cap did not report truncation")
	}
	if total > MaxEntityMetadataFields {
		t.Errorf("field count %d exceeds the cap %d", total, MaxEntityMetadataFields)
	}
	// The point of the cap is to drop FIELDS, never entities: an agent that
	// cannot see an entity's name invents one.
	if len(entities) != 2 {
		t.Fatalf("truncation dropped an entity: %d named, want 2", len(entities))
	}
}

// buildWideTables makes two entities sharing count fields between them.
func buildWideTables(count int) []interface{} {
	keys := func(prefix string, n int) []interface{} {
		out := make([]interface{}, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, map[string]interface{}{
				"key_name":  fmt.Sprintf("%s%03d", prefix, i),
				"key_label": fmt.Sprintf("Key %d", i),
				"data_type": "string",
			})
		}
		return out
	}
	table := func(name string, n int, prefix string) interface{} {
		return map[string]interface{}{
			"table_name":  "demo_" + name,
			"entity_name": name,
			"columns": []interface{}{map[string]interface{}{
				"column_name": "entity_data",
				"data_type":   "jsonb",
				"json_schema": keys(prefix, n),
			}},
		}
	}
	half := count / 2
	return []interface{}{table("bulky", half, "a"), table("second", count-half, "b")}
}
