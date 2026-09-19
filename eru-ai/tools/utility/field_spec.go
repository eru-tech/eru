package utiltiy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	catalog "github.com/eru-tech/eru/eru-ai/agents/processo_builder/catalog"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

const FieldSpecToolName = "get_field_spec"

// MaxFieldSpecDatatypes caps one lookup. An entity that genuinely needs more
// datatypes than this is better served by two calls.
const MaxFieldSpecDatatypes = 12

var fieldSpecToolActions = []tools.ToolAction{
	{
		ActionName:   FieldSpecToolName,
		Description:  "Look up the exact key set a processo field datatype writes",
		SystemPrompt: "Look up the exact key set a processo field datatype writes",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
}

func FieldSpecToolSchema() eru_models.JSONSchema {
	c := catalog.Get()
	allowed := make([]interface{}, 0, len(c.DatatypeNames()))
	for _, name := range c.DatatypeNames() {
		allowed = append(allowed, name)
	}
	return eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"datatypes": {
				Type: "array",
				Description: fmt.Sprintf(
					"The field datatypes to look up, at most %d per call. Ask for every datatype you are about to write whose keys you have not already been shown.",
					MaxFieldSpecDatatypes),
				Items: &eru_models.JSONSchema{Type: "string", Enum: allowed},
			},
		},
		Required: []string{"datatypes"},
	}
}

func FieldSpecToolDescription() string {
	return "Return the exact set of keys a processo field datatype writes - the keys of its own plus the " +
		"common set every field carries. Generated from the field editor itself, so it overrides anything you " +
		"remember about a field. Call it before writing a field of a datatype you have not already looked up in " +
		"this conversation: a key that does not belong to the datatype is silently dropped on save, so the field " +
		"arrives looking plain and nothing tells you why."
}

// FieldSpecTool answers "what does a field of this datatype actually carry" from
// the generated processo catalog. It is pure: no store, no network, no tenant
// state, so it is safe to call as often as the model likes.
type FieldSpecTool struct {
	tools.Tool
}

func (fsTool *FieldSpecTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(fieldSpecToolActions))
	for i, action := range fieldSpecToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (fsTool *FieldSpecTool) GetActions() []tools.ToolAction {
	return fieldSpecToolActions
}

func (fsTool *FieldSpecTool) GetSpec() tools.Tooling {
	return fsTool
}

func (fsTool *FieldSpecTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("FieldSpecTool MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &fsTool); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (fsTool *FieldSpecTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &FieldSpecTool{}
	if err := json.Unmarshal(toolObjJson, newTool); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return newTool, nil
}

func (fsTool *FieldSpecTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("FieldSpecTool Execute - Start")

	requested, err := fieldSpecDatatypes(params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	c := catalog.Get()
	known := make([]string, 0, len(requested))
	unknown := make([]string, 0)
	retired := make([]string, 0)
	for _, datatype := range requested {
		switch {
		case c.IsRetiredDatatype(datatype):
			retired = append(retired, datatype)
		case !c.IsDatatype(datatype):
			unknown = append(unknown, datatype)
		default:
			known = append(known, datatype)
		}
	}
	known = dedupe(known)

	result := map[string]interface{}{
		"catalog_version": c.CatalogVersion,
		"spec":            c.Spec(append(append([]string(nil), known...), append(unknown, retired...)...)),
	}
	if len(known) > 0 {
		result["datatypes"] = known
		result["common_keys_note"] = "Every field also carries the common keys listed in the system prompt; they are repeated in each spec so one lookup is enough to write the field."
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		result["unknown_datatypes"] = unknown
	}
	if len(retired) > 0 {
		sort.Strings(retired)
		result["retired_datatypes"] = retired
		result["note"] = fmt.Sprintf(
			"%s: retired. The backend still accepts them but the editor no longer offers them - do not write a field with these.",
			strings.Join(retired, ", "))
	}
	return result, false, nil
}

func fieldSpecDatatypes(params map[string]interface{}) ([]string, error) {
	raw, ok := params["datatypes"]
	if !ok {
		return nil, errors.New("get_field_spec requires \"datatypes\": an array of field datatype names")
	}
	var datatypes []string
	switch value := raw.(type) {
	case []interface{}:
		for _, item := range value {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				datatypes = append(datatypes, strings.TrimSpace(text))
			}
		}
	case []string:
		datatypes = value
	case string:
		// A model that passes "status, number" instead of an array still gets an answer.
		for _, part := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				datatypes = append(datatypes, trimmed)
			}
		}
	default:
		return nil, fmt.Errorf("get_field_spec \"datatypes\" must be an array of strings, got %T", raw)
	}
	if len(datatypes) == 0 {
		return nil, errors.New("get_field_spec requires at least one datatype")
	}
	if len(datatypes) > MaxFieldSpecDatatypes {
		datatypes = datatypes[:MaxFieldSpecDatatypes]
	}
	return datatypes, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:       false,
		ToolType:     "FieldSpec",
		Category:     "Utility",
		Description:  "The key set each processo field datatype writes, generated from the field editor",
		Actions:      []tools.ActionInfo{{Name: FieldSpecToolName}},
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(FieldSpecTool{}), []string{}),
	})
}
