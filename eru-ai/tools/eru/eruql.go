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
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	utils "github.com/eru-tech/eru/eru-utils"
)

type EruqlToolParams struct {
	QueryName string                 `json:"query_name" desc:"name of the query to be executed on eruql"`
	ProjectId string                 `json:"project_id" eru:"required" desc:"project id in which the query is stored"`
	Vars      map[string]interface{} `json:"vars" desc:"variables to execute the query with" default:"{}"`
}

type EruqSQLParams struct {
	Query     string                 `json:"query" eru:"required" desc:"SQL query to be executed directly"`
	ProjectId string                 `json:"project_id" eru:"required" desc:"project id for the query execution"`
	Vars      map[string]interface{} `json:"vars" desc:"variables to execute the query with" default:"{}"`
	Cols      string                 `json:"cols" desc:"column specifications for the query"`
}

type EruqlGraphQLParams struct {
	Query     string                 `json:"query" eru:"required" desc:"GraphQL query to be executed directly"`
	ProjectId string                 `json:"project_id" eru:"required" desc:"project id for the query execution"`
	Operation string                 `json:"operation" desc:"GraphQL operation name"`
	Vars      map[string]interface{} `json:"vars" desc:"variables to execute the query with" default:"{}"`
}

type EruqlTool struct {
	tools.Tool
	DbAlias                string `json:"db_alias" eru:"required" desc:"eruql database alias to execute the query against"`
	MandatoryVarsQuery     string `json:"mandatory_vars_query" desc:"query to fetch mandatory variables" default:"get_org_processes"`
	MandatoryVarsTransform string `json:"mandatory_vars_transform" desc:"gotemplate to transform query output into mandatory variables" default:"json"`
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
		Description:  "This action executes a SQL query directly against the database",
		SystemPrompt: "the values of 'vars' will be a map with key value pairs and it will be provided in the user's prompt.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqSQLParams{}))
		},
	},
	{
		ActionName:   ExecuteGraphQL,
		Description:  "Execute GraphQL query directly",
		SystemPrompt: "Execute GraphQL query directly",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlGraphQLParams{}))
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
	mandatoryVarsCheck := false
	if eruqlTool.MandatoryVarsQuery != "" {
		mandatoryVarsCheck = true
	}
	switch actionName {
	case ExecuteQuery:
		return eruqlTool.ExecuteQuery(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	case ExecuteSQL:
		return eruqlTool.ExecuteSQL(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	case ExecuteGraphQL:
		return eruqlTool.ExecuteGraphQL(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}
func (eruqlTool *EruqlTool) fetchMandatoryQueryVariables(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, error) {
	if params == nil {
		err := logs.Err(ctx, errors.New("params is nil"), "")
		return nil, err
	}
	vars, varsOk := params["vars"].(map[string]interface{})
	if !varsOk || vars == nil {
		params["vars"] = make(map[string]interface{})
		vars = make(map[string]interface{})
	} else {
		vars = params["vars"].(map[string]interface{})
	}
	vars["org_process_id"] = tenantId
	params["vars"] = vars
	params["query_name"] = eruqlTool.MandatoryVarsQuery
	result, _, err := eruqlTool.ExecuteQuery(ctx, projectId, tenantId, params, false)
	if err != nil {
		return nil, err
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("result: %+v", result))
	transformTemplate := gotemplate.GoTemplate{Name: "transform_mandatory_vars", Template: eruqlTool.MandatoryVarsTransform}
	output, err := transformTemplate.Execute(ctx, result, "json")
	if err != nil {
		return nil, err
	}

	if out, outoK := output.(map[string]interface{}); outoK {
		return out, nil
	}
	err = logs.Err(ctx, errors.New("output is not a map"), "")
	return nil, err
}

func (eruqlTool *EruqlTool) checkMandatoryVars(ctx context.Context, projectId string, tenantId string, vars map[string]interface{}, mandatoryVarsCheck bool) (map[string]interface{}, error) {
	if vars != nil {
		_, orgIdOk := vars["org_id"].(string)
		_, processIdOk := vars["process_id"].(string)
		if orgIdOk && processIdOk {
			mandatoryVarsCheck = false
		}
	}
	if mandatoryVarsCheck {
		mandatoryVars, err := eruqlTool.fetchMandatoryQueryVariables(ctx, projectId, tenantId, vars)
		if err != nil {
			return nil, err
		}
		return mandatoryVars, nil
	}
	return nil, nil
}
func (eruqlTool *EruqlTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &eruqlTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return eruqlTool, nil
}
func (eruqlTool *EruqlTool) ExecuteQuery(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, mandatoryVarsCheck bool) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ExecuteQuery - Start")
	eruqlToolParams := EruqlToolParams{}
	eruqlParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling eruqlparams: %w", err)
	}

	err = json.Unmarshal(eruqlParamsBytes, &eruqlToolParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, false, err
	}

	mVars, err := eruqlTool.checkMandatoryVars(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	if err != nil {
		return nil, false, err
	}
	if eruqlToolParams.Vars == nil {
		eruqlToolParams.Vars = make(map[string]interface{})
	}
	for k, v := range mVars {
		eruqlToolParams.Vars[k] = v
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

func (eruqlTool *EruqlTool) ExecuteSQL(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, mandatoryVarsCheck bool) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ExecuteDirectSQL - Start")
	eruqlSQLParams := EruqSQLParams{}
	eruqlParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling eruqlparams: %w", err)
	}

	err = json.Unmarshal(eruqlParamsBytes, &eruqlSQLParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, false, err
	}
	mVars, err := eruqlTool.checkMandatoryVars(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	if err != nil {
		return nil, false, err
	}
	for k, v := range mVars {
		eruqlSQLParams.Vars[k] = v
	}
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")

	body := map[string]interface{}{
		"query":    eruqlSQLParams.Query,
		"db_alias": eruqlTool.DbAlias,
		"cols":     eruqlSQLParams.Cols,
	}
	if eruqlSQLParams.Vars != nil {
		body["variables"] = eruqlSQLParams.Vars
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
	url := fmt.Sprint(eruqlBaseUrl, "/sql/", eruqlSQLParams.ProjectId, "/execute")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, true, nil
}

func (eruqlTool *EruqlTool) ExecuteGraphQL(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, mandatoryVarsCheck bool) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ExecuteDirectGraphQL - Start")
	eruqlGraphQLParams := EruqlGraphQLParams{}
	eruqlParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, false, fmt.Errorf("error marshalling eruqlparams: %w", err)
	}

	err = json.Unmarshal(eruqlParamsBytes, &eruqlGraphQLParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, false, err
	}
	mVars, err := eruqlTool.checkMandatoryVars(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	if err != nil {
		return nil, false, err
	}
	for k, v := range mVars {
		eruqlGraphQLParams.Vars[k] = v
	}
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")

	body := map[string]interface{}{
		"query":     eruqlGraphQLParams.Query,
		"operation": eruqlGraphQLParams.Operation,
	}
	if eruqlGraphQLParams.Vars != nil {
		body["variables"] = eruqlGraphQLParams.Vars
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
	url := fmt.Sprint(eruqlBaseUrl, "/graphql/", eruqlGraphQLParams.ProjectId, "/execute")
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
