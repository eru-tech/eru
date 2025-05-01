package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	gojsonschema "github.com/xeipuuv/gojsonschema"
)

type Tool struct {
	ToolType     string                `json:"tool_type" eru:"required"`
	ToolName     string                `json:"tool_name" eru:"required"`
	Description  string                `json:"description"`
	SystemPrompt string                `json:"system_prompt"`
	OutputSchema eru_models.JSONSchema `json:"output_schema"`
	Parameters   eru_models.JSONSchema `json:"parameters"`
	Actions      map[string]ToolAction `json:"actions"`
	//Inputs       []ToolInput           `json:"inputs"`
}

type ToolCallback struct {
	ResponseContentType string `json:"response_content_type"`
}

type ToolAction struct {
	ActionName   string                `json:"action_name" eru:"required"`
	Description  string                `json:"description"`
	SystemPrompt string                `json:"system_prompt"`
	OutputSchema eru_models.JSONSchema `json:"output_schema"`
	Parameters   eru_models.JSONSchema `json:"parameters"`
}

type ToolInput struct {
	FieldName  string `json:"field_name" eru:"required"`
	FieldValue string `json:"field_value"`
}

type ToolInputFields struct {
	FieldId          string `json:"field_id" eru:"required"`
	FieldName        string `json:"field_name" eru:"required"`
	FieldLabel       string `json:"field_label"`
	FieldType        string `json:"field_type" eru:"required"`
	FieldDescription string `json:"field_description"`
	FieldRequired    bool   `json:"field_required"`
}

type Tooling interface {
	GetSpec() Tooling
	GetActionsList() []string
	ValidateAction(ctx context.Context, actionName string, realTool Tooling) (err error)
	GetInputFields() []ToolInputFields
	Execute(ctx context.Context, actionName string, params map[string]interface{}) (map[string]interface{}, error)
	Callback(ctx context.Context, actionName string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, err error)
	ValidateOutput(ctx context.Context, output json.RawMessage) error
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error)
	GetToolCallback() ToolCallback
}

func (tool *Tool) GetToolCallback() ToolCallback {
	return ToolCallback{
		ResponseContentType: "application/json",
	}
}

func (tool *Tool) GetActionsList() []string {
	actions := []string{}
	for actionName := range tool.Actions {
		actions = append(actions, actionName)
	}
	return actions
}

func (tool *Tool) GetInputFields() []ToolInputFields {
	fields := []ToolInputFields{}
	return fields
}

func (tool *Tool) ValidateOutput(ctx context.Context, output json.RawMessage) error {
	schema := gojsonschema.NewGoLoader(tool.OutputSchema)
	document := gojsonschema.NewBytesLoader(output)

	result, err := gojsonschema.Validate(schema, document)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("schema validation error: %v", err))
		return err
	}

	if !result.Valid() {
		var errors []string
		for _, err := range result.Errors() {
			errors = append(errors, err.String())
		}
		err = fmt.Errorf("schema validation error: %v", errors)
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	return nil
}

func (tool *Tool) GetSpec() Tooling {
	return tool
}

func (tool *Tool) Execute(ctx context.Context, actionName string, params map[string]interface{}) (map[string]interface{}, error) {
	err := errors.New("Execute Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func (tool *Tool) Callback(ctx context.Context, actionName string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, err error) {
	err = errors.New("Callback Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func (tool *Tool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &tool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (tool *Tool) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "tool_name":
		return tool.ToolName, nil
	case "tool_type":
		return tool.ToolType, nil
	case "system_prompt":
		return tool.SystemPrompt, nil
	case "output_schema":
		return tool.OutputSchema, nil
	case "parameters":
		return tool.Parameters, nil
	case "description":
		return tool.Description, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (tool *Tool) ValidateAction(ctx context.Context, actionName string, realTool Tooling) (err error) {
	logs.WithContext(ctx).Info("ValidateAction - Start")
	actions := realTool.GetActionsList()
	logs.WithContext(ctx).Info(fmt.Sprintf("Actions: %v", actions))
	logs.WithContext(ctx).Info(fmt.Sprintf("Action Name: %v", actionName))

	if len(actions) == 0 && actionName == "" {
		//if no actions are defined, and no action name is provided, return nil
		return
	}
	if !slices.Contains(actions, actionName) {
		err = errors.New("action " + actionName + " not found")
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	//TODO - add param validation for Action
	return
}
