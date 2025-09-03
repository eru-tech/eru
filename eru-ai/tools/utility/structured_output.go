package utiltiy

import (
	"context"
	"encoding/json"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
)

type StructuredOutputTool struct {
	tools.Tool
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
	return params, false, nil
}

func (soTool *StructuredOutputTool) GetSpec() tools.Tooling {
	return soTool
}
func (soTool *StructuredOutputTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &soTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return soTool, nil
}
