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
	server "github.com/eru-tech/eru/eru-server/server"
	"github.com/eru-tech/eru/eru-templates/gotemplate"
	utils "github.com/eru-tech/eru/eru-utils"
)

type EruqlToolParams struct {
	QueryName string                 `json:"query_name" desc:"name of the query to be executed on eruql"`
	Vars      map[string]interface{} `json:"vars" desc:"variables to execute the query with" default:"{}"`
}

type EruqSQLParams struct {
	Query string                 `json:"query" eru:"required" desc:"SQL query to be executed directly"`
	Vars  map[string]interface{} `json:"vars" desc:"variables to execute the query with" default:"{}"`
	Cols  string                 `json:"cols" desc:"column specifications for the query"`
}

type EruqlGraphQLParams struct {
	Query     string                 `json:"query" eru:"required" desc:"GraphQL query to be executed directly"`
	Operation string                 `json:"operation" desc:"GraphQL operation name"`
	Vars      map[string]interface{} `json:"vars" desc:"variables to execute the query with" default:"{}"`
}

type EruqlSaveQueryParams struct {
	QueryName    string                 `json:"query_name" eru:"required" desc:"name of the query to save"`
	QueryType    string                 `json:"query_type" eru:"required" desc:"type of the query: sql or graphql"`
	Query        string                 `json:"query" eru:"required" desc:"query content to save"`
	Variables    map[string]interface{} `json:"variables" desc:"variables for the query" default:"{}"`
	DbAlias      string                 `json:"db_alias" desc:"database alias (used for sql queries)"`
	Cols         string                 `json:"cols" desc:"column specifications (used for sql queries)"`
	Operation    string                 `json:"operation" desc:"GraphQL operation name (used for graphql queries)"`
	SecurityRule map[string]interface{} `json:"security_rule" desc:"security rule for the query" default:"{}"`
}

type EruqlRemoveQueryParams struct {
	QueryName string `json:"query_name" eru:"required" desc:"name of the query to remove"`
}

type EruqlListQueriesParams struct {
	QueryType string `json:"query_type" eru:"required" desc:"type of queries to list: sql or graphql"`
}

type EruqlListQueryNamesParams struct {
}

type EruqlGetQueryParams struct {
	QueryName string `json:"query_name" eru:"required" desc:"name of the query to fetch"`
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
	SaveQuery      = "save_query"
	RemoveQuery    = "remove_query"
	ListQueries    = "list_queries"
	ListQueryNames = "list_query_names"
	GetQuery       = "get_query"
)

var eruqlToolActions = []tools.ToolAction{
	{
		ActionName:   ExecuteQuery,
		Description:  "Execute stored query",
		SystemPrompt: "Execute stored query",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlToolParams{}), []string{})
		},
	},
	{
		ActionName:   ExecuteSQL,
		Description:  "This action executes a SQL query directly against the database",
		SystemPrompt: "the values of 'vars' will be a map with key value pairs and it will be provided in the user's prompt.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqSQLParams{}), []string{})
		},
	},
	{
		ActionName:   ExecuteGraphQL,
		Description:  "Execute GraphQL query directly",
		SystemPrompt: "Execute GraphQL query directly",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlGraphQLParams{}), []string{})
		},
	},
	{
		ActionName:   SaveQuery,
		Description:  "Save a stored query (sql or graphql) under a given name in a project",
		SystemPrompt: "Save a stored query (sql or graphql) under a given name in a project",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlSaveQueryParams{}), []string{})
		},
	},
	{
		ActionName:   RemoveQuery,
		Description:  "Remove a stored query by name from a project",
		SystemPrompt: "Remove a stored query by name from a project",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlRemoveQueryParams{}), []string{})
		},
	},
	{
		ActionName:   ListQueries,
		Description:  "List stored queries of a given type (sql or graphql) in a project",
		SystemPrompt: "List stored queries of a given type (sql or graphql) in a project",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlListQueriesParams{}), []string{})
		},
	},
	{
		ActionName:   ListQueryNames,
		Description:  "List names of all stored queries in a project",
		SystemPrompt: "List names of all stored queries in a project",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlListQueryNamesParams{}), []string{})
		},
	},
	{
		ActionName:   GetQuery,
		Description:  "Fetch a stored query by name from a project",
		SystemPrompt: "Fetch a stored query by name from a project",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruqlGetQueryParams{}), []string{})
		},
	},
}

func (eruqlTool *EruqlTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(eruqlToolActions))
	for i, action := range eruqlToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (eruqlTool *EruqlTool) GetActions() []tools.ToolAction {
	return eruqlToolActions
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
	var toolRequest interface{}
	switch actionName {
	case ExecuteQuery:
		toolResult, toolRequest, persistStore, err = eruqlTool.ExecuteQuery(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	case ExecuteSQL:
		toolResult, toolRequest, persistStore, err = eruqlTool.ExecuteSQL(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	case ExecuteGraphQL:
		toolResult, toolRequest, persistStore, err = eruqlTool.ExecuteGraphQL(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	case SaveQuery:
		toolResult, toolRequest, persistStore, err = eruqlTool.SaveQuery(ctx, projectId, tenantId, params)
	case RemoveQuery:
		toolResult, toolRequest, persistStore, err = eruqlTool.RemoveQuery(ctx, projectId, tenantId, params)
	case ListQueries:
		toolResult, toolRequest, persistStore, err = eruqlTool.ListQueries(ctx, projectId, tenantId, params)
	case ListQueryNames:
		toolResult, toolRequest, persistStore, err = eruqlTool.ListQueryNames(ctx, projectId, tenantId, params)
	case GetQuery:
		toolResult, toolRequest, persistStore, err = eruqlTool.GetQuery(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}

	gm := server.GetGlobalGoroutineManager(ctx)
	gm.SafeGoWithRestartBehavior("tool-post-execute-hook", func(bgCtx context.Context) {
		claims := ctx.Value("claims")
		if claims != nil {
			bgCtx = context.WithValue(bgCtx, "claims", claims)
		}
		efurl := ctx.Value(tools.EruFuncBaseUrlKey)
		if efurl == nil {
			err = errors.New("erufuncbaseurl not found in context")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		efurlString, ok := efurl.(string)
		if !ok {
			err = errors.New("erufuncbaseurl is not a string")
			logs.WithContext(ctx).Error(err.Error())
			return
		} else {
			bgCtx = context.WithValue(bgCtx, tools.EruFuncBaseUrlKey, efurlString)
		}

		body := make(map[string]interface{})
		if toolRequest != nil {
			body["request"] = toolRequest
		}
		if toolResult != nil {
			body["response"] = toolResult
		}
		body["tenant_id"] = tenantId
		body["project_id"] = projectId

		if params["metadata"] != nil {
			body["metadata"] = params["metadata"]
		}

		hookResult, err := eruqlTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
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
	result, _, _, err := eruqlTool.ExecuteQuery(ctx, projectId, tenantId, params, false)
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
	newTool := &EruqlTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}

func (eruqlTool *EruqlTool) ExecuteQuery(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, mandatoryVarsCheck bool) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ExecuteQuery - Start")
	eruqlToolParams := EruqlToolParams{}
	eruqlParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling eruqlparams: %w", err)
	}

	err = json.Unmarshal(eruqlParamsBytes, &eruqlToolParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	mVars, err := eruqlTool.checkMandatoryVars(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	if err != nil {
		return nil, nil, false, err
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
		return nil, nil, false, err
	}
	if eruqlBaseUrl == "" {
		err = errors.New("eruqlbaseurl is not set")
		return nil, nil, false, err
	}
	url := fmt.Sprint(eruqlBaseUrl, "/store/", projectId, "/", tenantId, "/myquery/execute/", eruqlToolParams.QueryName)
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, body, true, nil
}

func (eruqlTool *EruqlTool) ExecuteSQL(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, mandatoryVarsCheck bool) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ExecuteDirectSQL - Start")
	eruqlSQLParams := EruqSQLParams{}
	eruqlParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling eruqlparams: %w", err)
	}

	err = json.Unmarshal(eruqlParamsBytes, &eruqlSQLParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}
	mVars, err := eruqlTool.checkMandatoryVars(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	if err != nil {
		return nil, nil, false, err
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
		return nil, nil, false, err
	}
	if eruqlBaseUrl == "" {
		err = errors.New("eruqlbaseurl is not set")
		return nil, nil, false, err
	}
	url := fmt.Sprint(eruqlBaseUrl, "/sql/", projectId, "/", tenantId, "/execute")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, body, true, nil
}

func (eruqlTool *EruqlTool) ExecuteGraphQL(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, mandatoryVarsCheck bool) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ExecuteDirectGraphQL - Start")
	eruqlGraphQLParams := EruqlGraphQLParams{}
	eruqlParamsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling eruqlparams: %w", err)
	}

	err = json.Unmarshal(eruqlParamsBytes, &eruqlGraphQLParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}
	mVars, err := eruqlTool.checkMandatoryVars(ctx, projectId, tenantId, params, mandatoryVarsCheck)
	if err != nil {
		return nil, nil, false, err
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
		return nil, nil, false, err
	}
	if eruqlBaseUrl == "" {
		err = errors.New("eruqlbaseurl is not set")
		return nil, nil, false, err
	}
	url := fmt.Sprint(eruqlBaseUrl, "/graphql/", projectId, "/", tenantId, "/execute")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, body, true, nil
}

func (eruqlTool *EruqlTool) getEruqlBaseUrl(ctx context.Context) (string, error) {
	eruqlBaseUrlAny := ctx.Value("eruqlbaseurl")
	eruqlBaseUrl, ok := eruqlBaseUrlAny.(string)
	if !ok {
		return "", errors.New("eruqlbaseurl is not a string")
	}
	if eruqlBaseUrl == "" {
		return "", errors.New("eruqlbaseurl is not set")
	}
	return eruqlBaseUrl, nil
}

func (eruqlTool *EruqlTool) buildHeaders(ctx context.Context) http.Header {
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")
	return headers
}

func (eruqlTool *EruqlTool) SaveQuery(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool SaveQuery - Start")
	saveParams := EruqlSaveQueryParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling save query params: %w", err)
	}
	if err = json.Unmarshal(paramsBytes, &saveParams); err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}
	if saveParams.QueryType != "sql" && saveParams.QueryType != "graphql" {
		return nil, nil, false, fmt.Errorf("invalid query_type %q, expected 'sql' or 'graphql'", saveParams.QueryType)
	}

	body := map[string]interface{}{
		"query":     saveParams.Query,
		"variables": saveParams.Variables,
	}
	if saveParams.SecurityRule != nil {
		body["security_rule"] = saveParams.SecurityRule
	}
	if saveParams.QueryType == "sql" {
		body["db_alias"] = saveParams.DbAlias
		body["cols"] = saveParams.Cols
	} else {
		body["operation"] = saveParams.Operation
	}

	eruqlBaseUrl, err := eruqlTool.getEruqlBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(eruqlBaseUrl, "/store/", projectId, "/", tenantId, "/myquery/save/", saveParams.QueryName, "/", saveParams.QueryType)
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, eruqlTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, body, true, nil
}

func (eruqlTool *EruqlTool) RemoveQuery(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool RemoveQuery - Start")
	removeParams := EruqlRemoveQueryParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling remove query params: %w", err)
	}
	if err = json.Unmarshal(paramsBytes, &removeParams); err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	eruqlBaseUrl, err := eruqlTool.getEruqlBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(eruqlBaseUrl, "/store/", projectId, "/", tenantId, "/myquery/remove/", removeParams.QueryName)
	reqBody := map[string]interface{}{
		"project_id": projectId,
		"query_name": removeParams.QueryName,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodDelete, url, eruqlTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, reqBody, true, nil
}

func (eruqlTool *EruqlTool) ListQueries(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ListQueries - Start")
	listParams := EruqlListQueriesParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling list queries params: %w", err)
	}
	if err = json.Unmarshal(paramsBytes, &listParams); err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	eruqlBaseUrl, err := eruqlTool.getEruqlBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	qt := ""
	if listParams.QueryType != "" {
		qt = fmt.Sprint("/", listParams.QueryType)
	}
	url := fmt.Sprint(eruqlBaseUrl, "/store/", projectId, "/", tenantId, "/myquery/list", qt)
	reqBody := map[string]interface{}{
		"project_id": projectId,
		"query_type": listParams.QueryType,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, eruqlTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, reqBody, true, nil
}

func (eruqlTool *EruqlTool) ListQueryNames(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool ListQueryNames - Start")
	listParams := EruqlListQueryNamesParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling list query names params: %w", err)
	}
	if err = json.Unmarshal(paramsBytes, &listParams); err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	eruqlBaseUrl, err := eruqlTool.getEruqlBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(eruqlBaseUrl, "/store/", projectId, "/", tenantId, "/myquery/list")
	reqBody := map[string]interface{}{
		"project_id": projectId,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, eruqlTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, reqBody, true, nil
}

func (eruqlTool *EruqlTool) GetQuery(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("eruqlTool GetQuery - Start")
	getParams := EruqlGetQueryParams{}
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, fmt.Errorf("error marshalling get query params: %w", err)
	}
	if err = json.Unmarshal(paramsBytes, &getParams); err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	eruqlBaseUrl, err := eruqlTool.getEruqlBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(eruqlBaseUrl, "/store/", projectId, "/", tenantId, "/myquery/fetch/", getParams.QueryName)
	reqBody := map[string]interface{}{
		"project_id": projectId,
		"query_name": getParams.QueryName,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, eruqlTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, reqBody, true, nil
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

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		ToolType:    "Eruql",
		Category:    "Data",
		Description: "Eru query engine supporting SQL, GraphQL, and custom EruQL for data access",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(eruqlToolActions))
			for i, a := range eruqlToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(EruqlTool{}), []string{}),
	})
}
