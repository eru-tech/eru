package eru

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

type EruqlToolParams struct {
	QueryName string                 `json:"query_name" desc:"name of the query to be executed on eruql"`
	Query     string                 `json:"query" desc:"actual graphql or sql query to be executed on eruql"`
	ProjectId string                 `json:"project_id" eru:"required" desc:"project id in which the query is stored"`
	Vars      map[string]interface{} `json:"vars" desc:"variables to execute the query with" default:"{}"`
}

type EruqlDirectSQLParams struct {
	Query     string                 `json:"query" eru:"required" desc:"SQL query to be executed directly"`
	ProjectId string                 `json:"project_id" eru:"required" desc:"project id for the query execution"`
	DBAlias   string                 `json:"db_alias" eru:"required" desc:"database alias to execute the query against"`
	Vars      map[string]interface{} `json:"vars" desc:"variables to execute the query with" default:"{}"`
	Cols      string                 `json:"cols" desc:"column specifications for the query"`
}

type EruqlDirectGraphQLParams struct {
	Query     string                 `json:"query" eru:"required" desc:"GraphQL query to be executed directly"`
	ProjectId string                 `json:"project_id" eru:"required" desc:"project id for the query execution"`
	Operation string                 `json:"operation" desc:"GraphQL operation name"`
	Vars      map[string]interface{} `json:"vars" desc:"variables to execute the query with" default:"{}"`
}

type EruqlTool struct {
	tools.Tool
}

const (
	ExecuteQuery   = "execute_query"
	ExecuteSQL     = "execute_sql"
	ExecuteGraphQL = "execute_graphql"
)

var eruqlToolActions = []tools.ToolAction{
	{
		ActionName:   ExecuteQuery,
		Description:  "Execute stored query",
		SystemPrompt: "Execute stored query",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlToolParams{}))
		},
	},
	{
		ActionName:   ExecuteSQL,
		Description:  "Execute SQL query directly",
		SystemPrompt: "Execute SQL query directly",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlDirectSQLParams{}))
		},
	},
	{
		ActionName:   ExecuteGraphQL,
		Description:  "Execute GraphQL query directly",
		SystemPrompt: "Execute GraphQL query directly",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlDirectGraphQLParams{}))
		},
	},
}

func (eruqlTool *EruqlTool) GetActionsList() []string {
	actions := []string{}
	for _, action := range eruqlToolActions {
		actions = append(actions, action.ActionName)
	}
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
	case ExecuteSQL:
		return eruqlTool.ExecuteSQL(ctx, params)
	case ExecuteGraphQL:
		return eruqlTool.ExecuteGraphQL(ctx, params)
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
	if eruqlParams, eruqlParamsOk := params["params"]; eruqlParamsOk {
		eruqlParamsBytes, err := json.Marshal(eruqlParams)
		if err != nil {
			return nil, false, fmt.Errorf("error marshalling eruqlparams: %w", err)
		}

		err = json.Unmarshal(eruqlParamsBytes, &eruqlToolParams)
		if err != nil {
			err = logs.Err(ctx, err, "")
			return nil, false, err
		}
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

func (eruqlTool *EruqlTool) ExecuteSQL(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ExecuteDirectSQL - Start")
	eruqlDirectSQLParams := EruqlDirectSQLParams{}
	if eruqlParams, eruqlParamsOk := params["params"]; eruqlParamsOk {
		eruqlParamsBytes, err := json.Marshal(eruqlParams)
		if err != nil {
			return nil, false, fmt.Errorf("error marshalling eruqlparams: %w", err)
		}

		err = json.Unmarshal(eruqlParamsBytes, &eruqlDirectSQLParams)
		if err != nil {
			err = logs.Err(ctx, err, "")
			return nil, false, err
		}
	}
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")

	body := map[string]interface{}{
		"query":    eruqlDirectSQLParams.Query,
		"db_alias": eruqlDirectSQLParams.DBAlias,
		"cols":     eruqlDirectSQLParams.Cols,
	}
	if eruqlDirectSQLParams.Vars != nil {
		body["variables"] = eruqlDirectSQLParams.Vars
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
	url := fmt.Sprint(eruqlBaseUrl, "/sql/", eruqlDirectSQLParams.ProjectId, "/execute")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, true, nil
}

func (eruqlTool *EruqlTool) ExecuteGraphQL(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ExecuteDirectGraphQL - Start")
	eruqlDirectGraphQLParams := EruqlDirectGraphQLParams{}
	if eruqlParams, eruqlParamsOk := params["params"]; eruqlParamsOk {
		eruqlParamsBytes, err := json.Marshal(eruqlParams)
		if err != nil {
			return nil, false, fmt.Errorf("error marshalling eruqlparams: %w", err)
		}

		err = json.Unmarshal(eruqlParamsBytes, &eruqlDirectGraphQLParams)
		if err != nil {
			err = logs.Err(ctx, err, "")
			return nil, false, err
		}
	}
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")

	body := map[string]interface{}{
		"query":     eruqlDirectGraphQLParams.Query,
		"operation": eruqlDirectGraphQLParams.Operation,
	}
	if eruqlDirectGraphQLParams.Vars != nil {
		body["variables"] = eruqlDirectGraphQLParams.Vars
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
	url := fmt.Sprint(eruqlBaseUrl, "/graphql/", eruqlDirectGraphQLParams.ProjectId, "/execute")
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

func (eruqlTool *EruqlTool) SetToolAction(actionName string) {
	for _, action := range eruqlToolActions {
		if action.ActionName == actionName {
			eruqlTool.ToolAction = action
			return
		}
	}
	eruqlTool.ToolAction = tools.ToolAction{}
}
