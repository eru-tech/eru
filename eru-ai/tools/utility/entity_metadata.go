package utiltiy

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

const EntityMetadataToolName = "get_entity_metadata"

// EntityMetadataQuery is the stored eru-ql query that returns every entity of
// the tenant with its fields. It is named here rather than left to the model:
// a query name the model has to remember is a query name it will eventually get
// wrong, and the only variable it takes is the tenant, which the model should
// never be handed.
const EntityMetadataQuery = "fetch_entity_table_column_metadata"

// entityKeyCandidates and fieldKeyCandidates are the column names the metadata
// query may use, most specific first. The grouping is best-effort by design: the
// rows come from a stored query that can change shape, and returning ungrouped
// rows is far better than returning nothing.
var (
	entityKeyCandidates      = []string{"entity_name", "entity", "table_name", "table", "entity_id"}
	entityDisplayCandidates  = []string{"entity_display_name", "entity_label", "display_name_entity"}
	fieldKeyCandidates       = []string{"field_name", "column_name", "field", "column"}
	fieldDisplayCandidates   = []string{"display_name", "field_display_name", "label", "column_display_name"}
	fieldTypeCandidates      = []string{"data_type", "datatype", "field_type", "column_type", "type"}
	fieldMandatoryCandidates = []string{"is_mandatory", "mandatory", "not_null", "is_required"}
)

// MaxEntityMetadataFields caps one answer. A tenant with hundreds of entities
// would otherwise return more than the page being designed could ever use.
const MaxEntityMetadataFields = 600

var entityMetadataToolActions = []tools.ToolAction{
	{
		ActionName:   EntityMetadataToolName,
		Description:  "List the tenant's entities and their fields",
		SystemPrompt: "List the tenant's entities and their fields",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
}

func EntityMetadataToolSchema() eru_models.JSONSchema {
	return eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"entity_names": {
				Type:        "array",
				Items:       &eru_models.JSONSchema{Type: "string"},
				Description: "Narrow the answer to these entities. Leave it out to list every entity, which is what you want when you do not yet know which one the page is about.",
			},
		},
	}
}

func EntityMetadataToolDescription() string {
	return "List the entities of this tenant with their fields: the real field name, its display name and its " +
		"data type. Use it before you name a form field or write a label - the field name goes in a component's " +
		"`name`/`identifier`, and the display name is the label a user expects to read. Guessing either produces a " +
		"page that looks right and binds to nothing."
}

// EntityMetadataTool answers "what are this tenant's entities and fields" by
// running one stored eru-ql query.
//
// It exists as its own tool rather than leaving the model to call execute_query
// because the call has exactly one correct form: this query name, and the tenant
// as its only variable. Exposing that as a general query runner invites the model
// to invent query names and to pass a tenant id it should not be able to choose.
type EntityMetadataTool struct {
	tools.Tool
	// Delegate is the configured eru-ql tool that actually runs the query. It is
	// wired per request by the agent that offers this tool, because the eru-ql
	// connection details belong to that agent's configuration.
	Delegate tools.Tooling `json:"-"`
	// QueryName defaults to EntityMetadataQuery and is here so a tenant on a
	// differently-named query does not need a code change.
	QueryName string `json:"query_name,omitempty"`
}

func (emTool *EntityMetadataTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(entityMetadataToolActions))
	for i, action := range entityMetadataToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (emTool *EntityMetadataTool) GetActions() []tools.ToolAction {
	return entityMetadataToolActions
}

func (emTool *EntityMetadataTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("EntityMetadataTool MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &emTool); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (emTool *EntityMetadataTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("EntityMetadataTool Execute - Start")
	if emTool.Delegate == nil {
		return nil, false, fmt.Errorf("%s is not available: no eru-ql tool is attached to this agent", EntityMetadataToolName)
	}

	queryName := emTool.QueryName
	if strings.TrimSpace(queryName) == "" {
		queryName = EntityMetadataQuery
	}

	// The tenant is taken from the execution context, never from the model.
	result, _, err := emTool.Delegate.Execute(ctx, projectId, tenantId, "execute_query", map[string]interface{}{
		"query_name": queryName,
		"vars":       map[string]interface{}{"org_process_id": tenantId},
	})
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("%s failed: %v", EntityMetadataToolName, err))
		return nil, false, fmt.Errorf("the entity metadata query failed: %w", err)
	}

	rows := extractRows(result)
	if len(rows) == 0 {
		return map[string]interface{}{
			"entities": []interface{}{},
			"note":     "this tenant has no entity metadata - name form fields from the user's own words and say so in your summary",
		}, false, nil
	}
	// The stored query owns its column names; logging the first row's keys is
	// what makes a shape change diagnosable instead of mysterious.
	logs.WithContext(ctx).Info(fmt.Sprintf("%s returned %d row(s) with keys %v", EntityMetadataToolName, len(rows), rowKeys(rows[0])))

	wanted := requestedEntities(params)
	entities, total, truncated := groupEntities(rows, wanted)

	out := map[string]interface{}{"entities": entities, "field_count": total}
	if truncated {
		out["note"] = fmt.Sprintf("truncated at %d fields - call again with entity_names to get the rest", MaxEntityMetadataFields)
	}
	if len(entities) == 0 {
		out["note"] = fmt.Sprintf("no entity matched %s - call again without entity_names to see what exists", strings.Join(wanted, ", "))
	}
	return out, false, nil
}

// extractRows finds the row list in whatever envelope the query result arrives
// in. eru-ql wraps results per query name, so the rows are usually one level in.
func extractRows(result map[string]interface{}) []map[string]interface{} {
	if result == nil {
		return nil
	}
	if rows := asRows(result); len(rows) > 0 {
		return rows
	}
	keys := make([]string, 0, len(result))
	for key := range result {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		switch nested := result[key].(type) {
		case []interface{}:
			if rows := rowsOf(nested); len(rows) > 0 {
				return rows
			}
		case map[string]interface{}:
			if rows := extractRows(nested); len(rows) > 0 {
				return rows
			}
		}
	}
	return nil
}

func asRows(result map[string]interface{}) []map[string]interface{} {
	for _, key := range []string{"rows", "data", "result", "records"} {
		if list, ok := result[key].([]interface{}); ok {
			if rows := rowsOf(list); len(rows) > 0 {
				return rows
			}
		}
	}
	return nil
}

func rowsOf(list []interface{}) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		if row, ok := item.(map[string]interface{}); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func rowKeys(row map[string]interface{}) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func requestedEntities(params map[string]interface{}) []string {
	if params == nil {
		return nil
	}
	switch value := params["entity_names"].(type) {
	case []interface{}:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	case []string:
		return value
	case string:
		out := []string{}
		for _, part := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return nil
}

type metadataField struct {
	Name      string `json:"name"`
	Label     string `json:"label,omitempty"`
	Type      string `json:"type,omitempty"`
	Mandatory bool   `json:"mandatory,omitempty"`
}

type metadataEntity struct {
	Name   string          `json:"name"`
	Label  string          `json:"label,omitempty"`
	Fields []metadataField `json:"fields"`
}

// groupEntities turns metadata rows into entities with their fields. Rows whose
// entity column cannot be identified are still returned, under an empty entity
// name, rather than silently dropped.
func groupEntities(rows []map[string]interface{}, wanted []string) ([]metadataEntity, int, bool) {
	keep := map[string]bool{}
	for _, name := range wanted {
		keep[strings.ToLower(name)] = true
	}

	order := []string{}
	byEntity := map[string]*metadataEntity{}
	total := 0
	truncated := false

	for _, row := range rows {
		entityName := firstString(row, entityKeyCandidates)
		if len(keep) > 0 && !keep[strings.ToLower(entityName)] {
			continue
		}
		if total >= MaxEntityMetadataFields {
			truncated = true
			break
		}

		entity, ok := byEntity[entityName]
		if !ok {
			entity = &metadataEntity{Name: entityName, Label: firstString(row, entityDisplayCandidates)}
			byEntity[entityName] = entity
			order = append(order, entityName)
		}
		fieldName := firstString(row, fieldKeyCandidates)
		if fieldName == "" {
			continue
		}
		entity.Fields = append(entity.Fields, metadataField{
			Name:      fieldName,
			Label:     firstString(row, fieldDisplayCandidates),
			Type:      firstString(row, fieldTypeCandidates),
			Mandatory: firstBool(row, fieldMandatoryCandidates),
		})
		total++
	}

	out := make([]metadataEntity, 0, len(order))
	for _, name := range order {
		out = append(out, *byEntity[name])
	}
	return out, total, truncated
}

func firstString(row map[string]interface{}, candidates []string) string {
	for _, key := range candidates {
		if value, ok := row[key]; ok {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return typed
				}
			case float64:
				return fmt.Sprintf("%g", typed)
			}
		}
	}
	return ""
}

func firstBool(row map[string]interface{}, candidates []string) bool {
	for _, key := range candidates {
		switch typed := row[key].(type) {
		case bool:
			return typed
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "true", "y", "yes", "1":
				return true
			}
		case float64:
			return typed != 0
		}
	}
	return false
}

func (emTool *EntityMetadataTool) GetSpec() tools.Tooling {
	return emTool
}

func (emTool *EntityMetadataTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &EntityMetadataTool{}
	if err := json.Unmarshal(toolObjJson, newTool); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return newTool, nil
}

// IsEruqlTool reports whether a configured tool can run stored eru-ql queries,
// which is what EntityMetadataTool delegates to.
func IsEruqlTool(ctx context.Context, tool tools.Tooling) bool {
	if tool == nil {
		return false
	}
	for _, action := range tool.GetActionsList() {
		if action.Name == "execute_query" {
			return true
		}
	}
	return false
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:       false,
		ToolType:     "EntityMetadata",
		Category:     "Utility",
		Description:  "Entity and field metadata for the tenant: real field names, display names and data types",
		Actions:      []tools.ActionInfo{{Name: EntityMetadataToolName}},
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(EntityMetadataTool{}), []string{}),
	})
}
