package utiltiy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	catalog "github.com/eru-tech/eru/eru-ai/agents/eru_studio/catalog"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

const ComponentSpecToolName = "get_component_spec"

// MaxComponentSpecTypes caps one lookup. A page that genuinely needs more types
// than this is better served by two calls than by a 40k-token tool result.
const MaxComponentSpecTypes = 16

var componentSpecToolActions = []tools.ToolAction{
	{
		ActionName:   ComponentSpecToolName,
		Description:  "Look up the property contract of eru-studio component types",
		SystemPrompt: "Look up the property contract of eru-studio component types",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
}

// ComponentSpecToolSchema is the parameter schema of get_component_spec.
func ComponentSpecToolSchema() eru_models.JSONSchema {
	c := catalog.Get()
	allowed := make([]interface{}, 0, len(c.ComponentTypes()))
	for _, componentType := range c.ComponentTypes() {
		allowed = append(allowed, componentType)
	}
	return eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"types": {
				Type: "array",
				Description: fmt.Sprintf(
					"The component types to look up, at most %d per call. Ask for every type you are about to use whose properties you have not already been shown.",
					MaxComponentSpecTypes),
				Items: &eru_models.JSONSchema{Type: "string", Enum: allowed},
			},
		},
		Required: []string{"types"},
	}
}

// ComponentSpecToolDescription is the description the model reads when deciding
// whether to call the tool.
func ComponentSpecToolDescription() string {
	return "Return the authoritative property contract for eru-studio component types: " +
		"every property key, its allowed values, its default and what it does, plus whether the type " +
		"accepts children and which events it emits. Generated from the component library itself, so it " +
		"overrides anything you remember about a component. Call it before setting properties on a type " +
		"you have not already looked up in this conversation."
}

// ComponentSpecTool answers "what can this component actually do" from the
// generated eru-studio catalog. It is pure: no store, no network, no tenant
// state, so it is safe to call as often as the model likes.
type ComponentSpecTool struct {
	tools.Tool
}

func (csTool *ComponentSpecTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(componentSpecToolActions))
	for i, action := range componentSpecToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (csTool *ComponentSpecTool) GetActions() []tools.ToolAction {
	return componentSpecToolActions
}

func (csTool *ComponentSpecTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("ComponentSpecTool MakeFromJson - Start")
	err := json.Unmarshal(*rj, &csTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (csTool *ComponentSpecTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("ComponentSpecTool Execute - Start")

	requested, err := componentSpecTypes(params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	c := catalog.Get()
	known := make([]string, 0, len(requested))
	unknown := make([]string, 0)
	renamed := map[string]string{}
	for _, componentType := range requested {
		component, ok := c.Component(componentType)
		switch {
		case !ok:
			unknown = append(unknown, componentType)
		case component.Deprecated:
			renamed[componentType] = component.AliasOf
			known = append(known, component.AliasOf)
		default:
			known = append(known, componentType)
		}
	}
	known = dedupe(known)

	result := map[string]interface{}{
		"catalog_version": c.CatalogVersion,
		"spec":            c.Spec(known),
	}
	if len(known) > 0 {
		result["types"] = known
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		result["unknown_types"] = unknown
		result["note"] = fmt.Sprintf(
			"%s: not component types. Use only the types listed in the component library; the closest matches are %s.",
			strings.Join(unknown, ", "), strings.Join(suggestFor(c, unknown), ", "))
	}
	if len(renamed) > 0 {
		notes := make([]string, 0, len(renamed))
		for from, to := range renamed {
			notes = append(notes, fmt.Sprintf("%s is a legacy alias of %s - emit %s", from, to, to))
		}
		sort.Strings(notes)
		result["legacy_aliases"] = notes
	}
	if len(known) > 0 {
		result["common_properties_note"] = "Every component also carries the common properties described in the system prompt; they are omitted here unless this type redefines or drops one."
	}
	return result, false, nil
}

func componentSpecTypes(params map[string]interface{}) ([]string, error) {
	raw, ok := params["types"]
	if !ok {
		return nil, errors.New("get_component_spec requires \"types\": an array of component type names")
	}
	var types []string
	switch value := raw.(type) {
	case []interface{}:
		for _, item := range value {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				types = append(types, strings.TrimSpace(text))
			}
		}
	case []string:
		types = value
	case string:
		// A model that passes "button, grid" instead of an array still gets an answer.
		for _, part := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				types = append(types, trimmed)
			}
		}
	default:
		return nil, fmt.Errorf("get_component_spec \"types\" must be an array of strings, got %T", raw)
	}
	if len(types) == 0 {
		return nil, errors.New("get_component_spec was called with an empty \"types\" list")
	}
	if len(types) > MaxComponentSpecTypes {
		return nil, fmt.Errorf("get_component_spec accepts at most %d types per call, got %d - split the lookup", MaxComponentSpecTypes, len(types))
	}
	return types, nil
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func suggestFor(c *catalog.Catalog, unknown []string) []string {
	out := make([]string, 0, len(unknown))
	for _, componentType := range unknown {
		if suggestion := c.SuggestType(componentType); suggestion != "" {
			out = append(out, fmt.Sprintf("%s -> %s", componentType, suggestion))
		}
	}
	if len(out) == 0 {
		return []string{"none"}
	}
	return out
}

func (csTool *ComponentSpecTool) GetSpec() tools.Tooling {
	return csTool
}

func (csTool *ComponentSpecTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &ComponentSpecTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:       false,
		ToolType:     "ComponentSpec",
		Category:     "Utility",
		Description:  "eru-studio component library lookup: property keys, allowed values and defaults for a component type",
		Actions:      []tools.ActionInfo{{Name: ComponentSpecToolName}},
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(ComponentSpecTool{}), []string{}),
	})
}
