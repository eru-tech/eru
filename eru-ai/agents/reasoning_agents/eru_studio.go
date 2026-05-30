package reasoning_agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	"github.com/google/uuid"
)

type EruStudioAgent struct {
	ReasoningAgent
}

func (eruStudioAgent *EruStudioAgent) GetSpec() agents.AgentI {
	return eruStudioAgent
}

func (eruStudioAgent *EruStudioAgent) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("EruStudioAgent MakeFromJson - Start")
	err := eruStudioAgent.ReasoningAgent.MakeFromJson(ctx, rj)
	if err != nil {
		return err
	}
	eruStudioAgent.ReasoningAgent.Agent.Provider = eruStudioAgent
	return nil
}

func (eruStudioAgent *EruStudioAgent) Execute(ctx context.Context, agentMessage agents.AgentMessage, conversationId string, projectId string, tenantId string) (agents.AgentMessage, error) {
	logs.WithContext(ctx).Debug("EruStudioAgent Execute - Start")

	augment := buildEruStudioContextAugmentation(ctx, agentMessage.Params, conversationId)
	if augment != "" {
		if strings.TrimSpace(agentMessage.Content) == "" {
			agentMessage.Content = augment
		} else {
			agentMessage.Content = fmt.Sprintf("%s\n\n--- USER PROMPT ---\n%s", augment, agentMessage.Content)
		}
	}

	return eruStudioAgent.ReasoningAgent.Execute(ctx, agentMessage, conversationId, projectId, tenantId)
}

func (eruStudioAgent *EruStudioAgent) GetOutputSchema(_ context.Context) eru_models.JSONSchema {
	return buildEruPageOutputSchema()
}

func (eruStudioAgent *EruStudioAgent) GetSystemPrompt() string {
	return eruStudioSystemPrompt
}

func buildEruStudioContextAugmentation(_ context.Context, params map[string]any, conversationId string) string {
	var b strings.Builder

	pageId := conversationId
	if pageId == "" {
		pageId = uuid.New().String()
	}
	fmt.Fprintf(&b, "Use %q as the EruPage.id (do not change it across iterations).\n\n", pageId)

	if codeRaw, ok := params["code"]; ok {
		codeStr := stringifyParam(codeRaw)
		if strings.TrimSpace(codeStr) != "" && strings.TrimSpace(codeStr) != "{}" && strings.TrimSpace(codeStr) != "null" {
			b.WriteString("--- EXISTING ERU PAGE JSON ---\n")
			b.WriteString("This is the page produced in a previous iteration. Build on top of it: preserve component ids, properties, styles, events, and validation_rules unless the user prompt clearly requires changing them. Add, remove, or restructure components only as needed to satisfy the new prompt.\n\n")
			b.WriteString(codeStr)
			b.WriteString("\n--- END EXISTING ERU PAGE JSON ---\n\n")
		} else {
			b.WriteString("No existing EruPage was provided. Build the page from scratch.\n\n")
		}
	}

	if ctxRaw, ok := params["context"]; ok {
		ctxStr := stringifyParam(ctxRaw)
		if strings.TrimSpace(ctxStr) != "" && strings.TrimSpace(ctxStr) != "{}" {
			b.WriteString("--- DATA CONTEXT (sample data and entity hints) ---\n")
			b.WriteString("Use this to choose appropriate component types, field names, and to populate component `data` properties (stringified JSON) where applicable. Handle nil/empty data gracefully via component defaults.\n\n")
			b.WriteString(ctxStr)
			b.WriteString("\n--- END DATA CONTEXT ---\n\n")
		}
	}

	if entitiesRaw, ok := params["entities"]; ok {
		entStr := stringifyParam(entitiesRaw)
		if strings.TrimSpace(entStr) != "" && strings.TrimSpace(entStr) != "[]" {
			b.WriteString("--- AVAILABLE ENTITIES ---\n")
			b.WriteString("Use these entities and their fields when wiring `name`, `entity_name`, and form-field `identifier`.\n\n")
			b.WriteString(entStr)
			b.WriteString("\n--- END AVAILABLE ENTITIES ---\n\n")
		}
	}

	if apisRaw, ok := params["apis"]; ok {
		apisStr := stringifyParam(apisRaw)
		if strings.TrimSpace(apisStr) != "" && strings.TrimSpace(apisStr) != "[]" {
			b.WriteString("--- AVAILABLE APIs ---\n")
			b.WriteString("Use these api names when wiring `call-api` events or chart `api` properties. Do not invent api names.\n\n")
			b.WriteString(apisStr)
			b.WriteString("\n--- END AVAILABLE APIs ---\n\n")
		}
	}

	return b.String()
}

func stringifyParam(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		bytes, err := json.Marshal(val)
		if err != nil {
			return ""
		}
		return string(bytes)
	}
}

// allowedComponentTypes mirrors the union of all entries in
// eru-studio/src/lib/services/component-definitions.ts (BASIC + LAYOUT +
// FORM + ERU + NAVIGATION + DATA + LOADING) plus the `widget` type
// registered in component-registry.service.ts.
var allowedComponentTypes = []any{
	// basic
	"text", "button", "image", "button_toggle", "badge", "chips",
	"icon", "progress_bar", "progress_spinner", "tile", "timer",
	// layout
	"flex_container", "grid_container", "card", "divider", "expansion_panel",
	"list", "stepper", "sidebar_stepper", "tree", "grid_list", "page_ref",
	"widget",
	// form (general)
	"radio", "slider", "slide_toggle", "autocomplete",
	// eru form components
	"phone", "email", "number", "currency", "date", "datetime", "time-picker",
	"duration", "website", "textarea", "textbox", "checkbox-eru", "select-eru",
	"attachment", "location", "people", "priority", "progress", "rating",
	"status", "tag",
	// navigation
	"toolbar", "menu", "sidenav", "tabs", "nav_menu", "nav_outlet",
	// data
	"grid", "eru_page", "line_chart", "bar_chart", "pie_chart",
	// loading
	"ghost",
}

// allowedEventActions mirrors the union literal in
// eru-studio/src/lib/models/eru-project.model.ts (ComponentEventSubscription.action).
var allowedEventActions = []any{
	"no-action", "call-api", "fetch-page-data", "hide-fields", "unhide-fields",
	"save-page-data", "start-loading", "stop-loading", "hide-component",
	"show-component", "disable-field", "enable-field", "update-state",
	"start-timer", "stop-timer", "set-field", "enable-component",
	"disable-component", "refresh-grid", "step-forward", "step-back",
	"emit-to-parent",
	"toggle-side-panel", "open-side-panel", "close-side-panel",
	"navigate-to-page", "clear-page-data", "clear-all-page-data",
}

var allowedValidationTypes = []any{
	"required", "min", "max", "minLength", "maxLength", "pattern", "email", "custom",
}

var allowedDisplayModes = []any{"inline", "popup", "side_panel"}

var allowedNestingTypes = []any{"none", "object", "array", "nested_object", "nested_array"}

func buildEruPageOutputSchema() eru_models.JSONSchema {
	breakpointObject := eru_models.JSONSchema{
		Type:                 "object",
		AdditionalProperties: true,
		Description:          "Free-form key/value pairs from the component property catalog. Keys vary per component type.",
	}

	responsiveProps := eru_models.JSONSchema{
		Type:        "object",
		Description: "Responsive component properties. Put defaults under `base`. Add sm/md/lg/xl/2xl ONLY when the user requests responsive behaviour.",
		Properties: map[string]eru_models.JSONSchema{
			"base": breakpointObject,
			"sm":   breakpointObject,
			"md":   breakpointObject,
			"lg":   breakpointObject,
			"xl":   breakpointObject,
			"2xl":  breakpointObject,
		},
		AdditionalProperties: false,
	}

	responsiveStyles := eru_models.JSONSchema{
		Type:        "object",
		Description: "Responsive style values. Put defaults under `base`.",
		Properties: map[string]eru_models.JSONSchema{
			"base": breakpointObject,
			"sm":   breakpointObject,
			"md":   breakpointObject,
			"lg":   breakpointObject,
			"xl":   breakpointObject,
			"2xl":  breakpointObject,
		},
		AdditionalProperties: false,
	}

	responsiveClasses := eru_models.JSONSchema{
		Type:        "object",
		Description: "Tailwind utility classes per breakpoint.",
		Properties: map[string]eru_models.JSONSchema{
			"base": {Type: "string"},
			"sm":   {Type: "string"},
			"md":   {Type: "string"},
			"lg":   {Type: "string"},
			"xl":   {Type: "string"},
			"2xl":  {Type: "string"},
		},
		AdditionalProperties: false,
	}

	stylesSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"classes":            {Type: "string", Description: "Always-applied Tailwind classes (use empty string when none)."},
			"responsive_classes": responsiveClasses,
			"responsive_styles":  responsiveStyles,
			"custom":             {Type: "object", AdditionalProperties: true, Description: "Free-form CSS map (camelCase keys)."},
		},
		Required:             []string{"classes", "responsive_classes", "responsive_styles", "custom"},
		AdditionalProperties: false,
	}

	eventSchema := eru_models.JSONSchema{
		Type:        "object",
		Description: "ComponentEventSubscription wiring an emitted event to a runtime action.",
		Properties: map[string]eru_models.JSONSchema{
			"id":                     {Type: "string"},
			"event":                  {Type: "string", Description: "DOM/component/page event (click, valueChange, focus, blur, mouseenter, buttonpress, on_load, timeout, timer_start, ...)."},
			"action":                 {Type: "string", Enum: allowedEventActions},
			"apiName":                {Type: "string", Description: "REQUIRED when action is `call-api`."},
			"fieldNames":             {Type: "array", Items: &eru_models.JSONSchema{Type: "string"}, Description: "Target field/component ids. Use [page_ref_id] for *-side-panel actions, [timer_id] for *-timer actions, [grid_id] for refresh-grid, [component_id] for hide/show/enable/disable-component and start/stop-loading."},
			"page_id":                {Type: "string", Description: "Target page id for `navigate-to-page`."},
			"payload":                {Type: "object", AdditionalProperties: true},
			"state_key":              {Type: "string"},
			"state_formula":          {Type: "object", AdditionalProperties: true, Description: "UpdateStateFormula { fn: set|increment|decrement|toggle|set-from-field|set-from-payload|reset|expr, value?, by?, values?, field?, expr?, payload_path? }"},
			"value":                  {},
			"on_success":             {Type: "array", Items: &eru_models.JSONSchema{Type: "object", AdditionalProperties: true}},
			"on_error":               {Type: "array", Items: &eru_models.JSONSchema{Type: "object", AdditionalProperties: true}},
			"error_field":            {Type: "string"},
			"error_state_key":        {Type: "string"},
			"validate_before_action": {Type: "boolean"},
			"validate_field_names":   {Type: "array", Items: &eru_models.JSONSchema{Type: "string"}},
		},
		Required:             []string{"id", "event", "action"},
		AdditionalProperties: false,
	}

	validationRuleSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"type":    {Type: "string", Enum: allowedValidationTypes},
			"value":   {},
			"message": {Type: "string"},
		},
		Required:             []string{"type", "message"},
		AdditionalProperties: false,
	}

	// Recursive children: described as objects with the same shape; loose
	// because Go's JSON encoder cannot represent a self-referential schema
	// without infinite recursion. The root EruComponent schema below is
	// strict; the prompt enforces the same shape on children.
	componentChildSchema := eru_models.JSONSchema{
		Type: "object",
		Description: "A nested EruComponent. MUST have the SAME shape as a top-level component:\n" +
			"  { id, type (one of the allowed types), properties.{base[, sm, md, lg, xl, 2xl]}, " +
			"styles.{classes, responsive_classes, responsive_styles, custom}, events?, validation_rules?, children? }.\n" +
			"Do NOT invent extra top-level keys on a child (no `style`, `left`, `right`, `header`, `sections`, etc.).",
		AdditionalProperties: true,
	}

	componentSchema := eru_models.JSONSchema{
		Type:        "object",
		Description: "An EruComponent.",
		Properties: map[string]eru_models.JSONSchema{
			"id":               {Type: "string", Description: "Unique component id (slug + short suffix)."},
			"type":             {Type: "string", Enum: allowedComponentTypes, Description: "Component type. MUST be one of the listed values; never invent a new type."},
			"isNested":         {Type: "boolean"},
			"nesting_type":     {Type: "string", Enum: allowedNestingTypes},
			"pageId":           {Type: "string"},
			"index":            {Type: "integer"},
			"entityName":       {Type: "string"},
			"properties":       responsiveProps,
			"styles":           stylesSchema,
			"events":           {Type: "array", Items: &eventSchema},
			"validation_rules": {Type: "array", Items: &validationRuleSchema},
			"children":         {Type: "array", Items: &componentChildSchema, Description: "Only allowed for container types (flex_container, grid_container, card, expansion_panel, stepper, sidebar_stepper, sidenav, toolbar, tabs). page_ref and widget MUST NOT use children — they embed another page/widget by id."},
			"parent_id":        {Type: "string"},
			"created_at":       {Type: "string"},
			"updated_at":       {Type: "string"},
		},
		Required:             []string{"id", "type", "properties", "styles"},
		AdditionalProperties: false,
	}

	pageStateVariableSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"key":     {Type: "string"},
			"initial": {},
			"formula": {Type: "object", AdditionalProperties: true, Description: "{ fn: count|sum|avg|min|max|expr, source?: pageDataArray, field?, filter?, value? }"},
		},
		Required:             []string{"key"},
		AdditionalProperties: false,
	}

	pageStylesSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"classes":            {Type: "string"},
			"responsive_classes": responsiveClasses,
			"responsive_styles":  responsiveStyles,
			"custom":             {Type: "object", AdditionalProperties: true},
		},
		Required:             []string{"classes", "responsive_classes", "responsive_styles", "custom"},
		AdditionalProperties: false,
	}

	masterDetailSchema := eru_models.JSONSchema{
		Type: "object",
		Properties: map[string]eru_models.JSONSchema{
			"master_field":      {Type: "string"},
			"detail_field":      {Type: "string"},
			"relationship_type": {Type: "string", Enum: []any{"one-to-many", "many-to-one", "one-to-one"}},
			"auto_open":         {Type: "boolean"},
			"display_mode":      {Type: "string", Enum: []any{"popup", "side_panel"}},
		},
		AdditionalProperties: false,
	}

	pageSchema := eru_models.JSONSchema{
		Type: "object",
		Description: "EruPage root object. Mirrors the EruPage TypeScript interface in eru-studio/src/lib/models/eru-project.model.ts.\n" +
			"Allowed top-level keys: id, name, title, entity_name, route, components, styles, state, display_mode, parent_page_id, master_detail_config, events.\n" +
			"DO NOT add invented keys such as `theme`, `layout`, `slug`, `version`, `description`, `topbar`, `sidebar`, `colorScheme`.",
		Properties: map[string]eru_models.JSONSchema{
			"id":                   {Type: "string", Description: "Stable page id. Reuse the value provided in the user message verbatim."},
			"name":                 {Type: "string", Description: "snake_case page name."},
			"title":                {Type: "string"},
			"entity_name":          {Type: "string"},
			"route":                {Type: "string"},
			"components":           {Type: "array", Items: &componentSchema, Description: "Top-level components (typically a single root container holding everything else)."},
			"styles":               pageStylesSchema,
			"state":                {Type: "array", Items: &pageStateVariableSchema},
			"display_mode":         {Type: "string", Enum: allowedDisplayModes},
			"parent_page_id":       {Type: "string"},
			"master_detail_config": masterDetailSchema,
			"events":               {Type: "array", Items: &eventSchema, Description: "Page-level event subscriptions."},
		},
		Required:             []string{"id", "name", "components", "styles"},
		AdditionalProperties: false,
	}

	return pageSchema
}
