package eru

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

type EruqlToolParams struct {
	QueryName string                 `json:"query_name"`
	Query     string                 `json:"query"`
	DbAlias   string                 `json:"db_alias"`
	ProjectId string                 `json:"project_id"`
	Vars      map[string]interface{} `json:"vars"`
}

type EruqlTool struct {
	tools.Tool
}

const (
	ExecuteQuery = "execute_query"
)

func (eruqlTool *EruqlTool) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, ExecuteQuery)
	return actions
}

func (eruqlTool *EruqlTool) GetSpec() tools.Tooling {
	return eruqlTool
}

func (eruqlTool *EruqlTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &eruqlTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (eruqlTool *EruqlTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool Execute - Start")
	switch actionName {
	case ExecuteQuery:
		return eruqlTool.ExecuteQuery(ctx, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (eruqlTool *EruqlTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &eruqlTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return eruqlTool, nil
}
func (eruqlTool *EruqlTool) ExecuteQuery(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ExecuteQuery - Start")
	eruqlToolParams := EruqlToolParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, false, err
	}
	err = json.Unmarshal(paramsBytes, &eruqlToolParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, false, err
	}
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")
	body := map[string]interface{}{}
	if eruqlToolParams.Vars != nil {
		for k, v := range eruqlToolParams.Vars {
			body[k] = v
		}
	}
	eruqlBaseUrlAny := ctx.Value("eruqlbaseurl")
	eruqlBaseUrl, ok := eruqlBaseUrlAny.(string)
	if !ok {
		err = errors.New("eruqlbaseurl is not a string")
		return nil, false, err
	}
	if eruqlBaseUrl == "" {
		err = errors.New("eruqlbaseurl is not set")
		return nil, false, err
	}
	url := fmt.Sprint(eruqlBaseUrl, "/store/", eruqlToolParams.ProjectId, "/myquery/execute/", eruqlToolParams.QueryName)
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, true, nil
}

func (eruqlTool *EruqlTool) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "tool_name":
		return eruqlTool.ToolName, nil
	case "tool_type":
		return eruqlTool.ToolType, nil
	case "system_prompt":
		return eruqlTool.SystemPrompt, nil
	case "output_schema":
		return eruqlTool.OutputSchema, nil
	case "parameters":
		return eruqlTool.Parameters, nil
	case "description":
		return eruqlTool.Description, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}
func (eruqlTool *EruqlTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) (err error) {
	switch attributeName {
	case "tool_name":
		eruqlTool.ToolName = attributeValue.(string)
	case "tool_type":
		eruqlTool.ToolType = attributeValue.(string)
	case "system_prompt":
		eruqlTool.SystemPrompt = attributeValue.(string)
	case "output_schema":
		eruqlTool.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		eruqlTool.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		eruqlTool.Description = attributeValue.(string)
	default:
		err = errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
func (eruqlTool *EruqlTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(eruqlTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}
