package utiltiy

import (
	"context"
	"encoding/json"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
)

var structuredOutputToolActions = []tools.ToolAction{
	{
		ActionName:   "structured_output",
		Description:  "Output the result",
		SystemPrompt: "Output the result",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
}

type StructuredOutputTool struct {
	tools.Tool
}

func (soTool *StructuredOutputTool) GetActionsList() []string {
	actions := []string{}
	for _, action := range structuredOutputToolActions {
		actions = append(actions, action.ActionName)
	}
	return actions
}
func (soTool *StructuredOutputTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &soTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (soTool *StructuredOutputTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("StructuredOutputTool Execute - Start")
	if paramsObj, ok := params["params"]; ok {
		if paramsMap, ok := paramsObj.(map[string]interface{}); ok {
			return paramsMap, false, nil
		}
	}
	return params, false, nil
}

func (soTool *StructuredOutputTool) GetSpec() tools.Tooling {
	return soTool
}
func (soTool *StructuredOutputTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &StructuredOutputTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}
