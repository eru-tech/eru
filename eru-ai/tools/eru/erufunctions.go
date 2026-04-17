package eru

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

type ErufuncSaveFuncParams struct {
	ProjectId string                 `json:"project_id" eru:"required" desc:"project id in which to save the function"`
	FuncGroup map[string]interface{} `json:"func_group" eru:"required" desc:"function group object to save"`
}

type ErufuncValidateFuncParams struct {
	FuncGroup map[string]interface{} `json:"func_group" eru:"required" desc:"function group object to validate"`
}

type ErufuncRemoveFuncParams struct {
	ProjectId string `json:"project_id" eru:"required" desc:"project id from which to remove the function"`
	FuncName  string `json:"func_name" eru:"required" desc:"name of the function to remove"`
}

type ErufuncRunFuncParams struct {
	ProjectId       string                 `json:"project_id" eru:"required" desc:"project id for the function run"`
	Func            map[string]interface{} `json:"func" eru:"required" desc:"inline function group to execute"`
	Body            interface{}            `json:"body" desc:"request body passed to the function"`
	ReqVars         map[string]interface{} `json:"req_vars" desc:"request template vars keyed by step name" default:"{}"`
	ResVars         map[string]interface{} `json:"res_vars" desc:"response template vars keyed by step name" default:"{}"`
	FuncStepName    string                 `json:"func_step_name" desc:"optional starting step name"`
	EndFuncStepName string                 `json:"end_func_step_name" desc:"optional ending step name (requires func_step_name)"`
}

type ErufuncExecuteTemplateParams struct {
	Name     string      `json:"name" eru:"required" desc:"template name"`
	Template string      `json:"template" eru:"required" desc:"go template string"`
	Object   interface{} `json:"object" desc:"object passed as template data"`
}

type ErufuncListMyQueriesParams struct {
	ProjectId string `json:"project_id" eru:"required" desc:"project id to list stored queries for"`
}

type ErufuncListFuncsParams struct {
	ProjectId string `json:"project_id" eru:"required" desc:"project id to list functions for"`
}

type ErufuncListAgentsParams struct {
	ProjectId string `json:"project_id" eru:"required" desc:"project id to list agents for"`
	TenantId  string `json:"tenant_id" desc:"optional tenant id to scope the list"`
}

type ErufuncListToolsParams struct {
	ProjectId string `json:"project_id" eru:"required" desc:"project id to list tools for"`
	TenantId  string `json:"tenant_id" desc:"optional tenant id to scope the list"`
}

type ErufuncScheduleFuncParams struct {
	ProjectId string                 `json:"project_id" eru:"required" desc:"project id of the function to schedule"`
	FuncName  string                 `json:"func_name" eru:"required" desc:"name of the function to schedule"`
	Schedule  map[string]interface{} `json:"schedule" eru:"required" desc:"schedule configuration object"`
	Body      map[string]interface{} `json:"body" desc:"additional body data to pass to the scheduled function" default:"{}"`
}

type ErufuncUnScheduleFuncParams struct {
	ProjectId string `json:"project_id" eru:"required" desc:"project id of the scheduled function"`
	JobId     string `json:"job_id" eru:"required" desc:"scheduler job id to cancel"`
}

type ErufunctionsTool struct {
	tools.Tool
}

const (
	SaveFunc        = "save_func"
	ValidateFunc    = "validate_func"
	RemoveFunc      = "remove_func"
	RunFunc         = "run_func"
	ExecuteTemplate = "execute_template"
	ListMyQueries   = "list_myqueries"
	ListFuncs       = "list_funcs"
	ListAgents      = "list_agents"
	ListTools       = "list_tools"
	ScheduleFunc    = "schedule_func"
	UnScheduleFunc  = "unschedule_func"
)

var erufunctionsToolActions = []tools.ToolAction{
	{
		ActionName:   SaveFunc,
		Description:  "Save a function group under a project",
		SystemPrompt: "Save a function group under a project",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncSaveFuncParams{}), []string{})
		},
	},
	{
		ActionName:   ValidateFunc,
		Description:  "Validate a function group configuration",
		SystemPrompt: "Validate a function group configuration",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncValidateFuncParams{}), []string{})
		},
	},
	{
		ActionName:   RemoveFunc,
		Description:  "Remove a function by name from a project",
		SystemPrompt: "Remove a function by name from a project",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncRemoveFuncParams{}), []string{})
		},
	},
	{
		ActionName:   RunFunc,
		Description:  "Run an inline function group, optionally starting and ending at specific steps",
		SystemPrompt: "Run an inline function group, optionally starting and ending at specific steps",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncRunFuncParams{}), []string{})
		},
	},
	{
		ActionName:   ExecuteTemplate,
		Description:  "Execute a Go template against a data object",
		SystemPrompt: "Execute a Go template against a data object",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncExecuteTemplateParams{}), []string{})
		},
	},
	{
		ActionName:   ListMyQueries,
		Description:  "List stored query names for a project (via eru-functions)",
		SystemPrompt: "List stored query names for a project",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncListMyQueriesParams{}), []string{})
		},
	},
	{
		ActionName:   ListFuncs,
		Description:  "List function names for a project",
		SystemPrompt: "List function names for a project",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncListFuncsParams{}), []string{})
		},
	},
	{
		ActionName:   ListAgents,
		Description:  "List agent names for a project (optionally scoped to a tenant)",
		SystemPrompt: "List agent names for a project (optionally scoped to a tenant)",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncListAgentsParams{}), []string{})
		},
	},
	{
		ActionName:   ListTools,
		Description:  "List tool names for a project (optionally scoped to a tenant)",
		SystemPrompt: "List tool names for a project (optionally scoped to a tenant)",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncListToolsParams{}), []string{})
		},
	},
	{
		ActionName:   ScheduleFunc,
		Description:  "Schedule a function to run on a configured schedule",
		SystemPrompt: "Schedule a function to run on a configured schedule",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncScheduleFuncParams{}), []string{})
		},
	},
	{
		ActionName:   UnScheduleFunc,
		Description:  "Unschedule a function by job id",
		SystemPrompt: "Unschedule a function by job id",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufuncUnScheduleFuncParams{}), []string{})
		},
	},
}

func (erufuncTool *ErufunctionsTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(erufunctionsToolActions))
	for i, action := range erufunctionsToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (erufuncTool *ErufunctionsTool) GetActions() []tools.ToolAction {
	return erufunctionsToolActions
}

func (erufuncTool *ErufunctionsTool) GetSpec() tools.Tooling {
	return erufuncTool
}

func (erufuncTool *ErufunctionsTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &erufuncTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (erufuncTool *ErufunctionsTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &ErufunctionsTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}

func (erufuncTool *ErufunctionsTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(erufuncTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (erufuncTool *ErufunctionsTool) SetToolAction(actionName string) {
	for _, action := range erufunctionsToolActions {
		if action.ActionName == actionName {
			erufuncTool.ToolAction = action
			return
		}
	}
	erufuncTool.ToolAction = tools.ToolAction{}
}

func (erufuncTool *ErufunctionsTool) GetAttribute(ctx context.Context, attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "tool_name":
		return erufuncTool.ToolName, nil
	case "tool_type":
		return erufuncTool.ToolType, nil
	case "system_prompt":
		return erufuncTool.SystemPrompt, nil
	case "output_schema":
		return erufuncTool.OutputSchema, nil
	case "parameters":
		return erufuncTool.Parameters, nil
	case "description":
		return erufuncTool.Description, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (erufuncTool *ErufunctionsTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) (err error) {
	switch attributeName {
	case "tool_name":
		erufuncTool.ToolName = attributeValue.(string)
	case "tool_type":
		erufuncTool.ToolType = attributeValue.(string)
	case "system_prompt":
		erufuncTool.SystemPrompt = attributeValue.(string)
	case "output_schema":
		erufuncTool.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		erufuncTool.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		erufuncTool.Description = attributeValue.(string)
	default:
		err = errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (erufuncTool *ErufunctionsTool) getEruFuncBaseUrl(ctx context.Context) (string, error) {
	efurl := ctx.Value(tools.EruFuncBaseUrlKey)
	if efurl == nil {
		return "", errors.New("erufuncbaseurl not found in context")
	}
	efurlString, ok := efurl.(string)
	if !ok {
		return "", errors.New("erufuncbaseurl is not a string")
	}
	if efurlString == "" {
		return "", errors.New("erufuncbaseurl is not set")
	}
	return efurlString, nil
}

func (erufuncTool *ErufunctionsTool) buildHeaders(ctx context.Context) http.Header {
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")
	return headers
}

func (erufuncTool *ErufunctionsTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case SaveFunc:
		toolResult, toolRequest, persistStore, err = erufuncTool.SaveFunc(ctx, projectId, tenantId, params)
	case ValidateFunc:
		toolResult, toolRequest, persistStore, err = erufuncTool.ValidateFunc(ctx, projectId, tenantId, params)
	case RemoveFunc:
		toolResult, toolRequest, persistStore, err = erufuncTool.RemoveFunc(ctx, projectId, tenantId, params)
	case RunFunc:
		toolResult, toolRequest, persistStore, err = erufuncTool.RunFunc(ctx, projectId, tenantId, params)
	case ExecuteTemplate:
		toolResult, toolRequest, persistStore, err = erufuncTool.ExecuteTemplate(ctx, projectId, tenantId, params)
	case ListMyQueries:
		toolResult, toolRequest, persistStore, err = erufuncTool.ListMyQueries(ctx, projectId, tenantId, params)
	case ListFuncs:
		toolResult, toolRequest, persistStore, err = erufuncTool.ListFuncs(ctx, projectId, tenantId, params)
	case ListAgents:
		toolResult, toolRequest, persistStore, err = erufuncTool.ListAgents(ctx, projectId, tenantId, params)
	case ListTools:
		toolResult, toolRequest, persistStore, err = erufuncTool.ListTools(ctx, projectId, tenantId, params)
	case ScheduleFunc:
		toolResult, toolRequest, persistStore, err = erufuncTool.ScheduleFunc(ctx, projectId, tenantId, params)
	case UnScheduleFunc:
		toolResult, toolRequest, persistStore, err = erufuncTool.UnScheduleFunc(ctx, projectId, tenantId, params)
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
			logs.WithContext(ctx).Error("erufuncbaseurl not found in context")
			return
		}
		efurlString, ok := efurl.(string)
		if !ok {
			logs.WithContext(ctx).Error("erufuncbaseurl is not a string")
			return
		}
		bgCtx = context.WithValue(bgCtx, tools.EruFuncBaseUrlKey, efurlString)

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

		hookResult, hookErr := erufuncTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if hookErr != nil {
			logs.WithContext(bgCtx).Error(hookErr.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (erufuncTool *ErufunctionsTool) unmarshalParams(ctx context.Context, params map[string]interface{}, target interface{}) error {
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("error marshalling params: %w", err)
	}
	if err = json.Unmarshal(paramsBytes, target); err != nil {
		return logs.Err(ctx, err, "")
	}
	return nil
}

func (erufuncTool *ErufunctionsTool) SaveFunc(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool SaveFunc - Start")
	p := ErufuncSaveFuncParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/store/", p.ProjectId, "/func/save")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, p.FuncGroup)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, p.FuncGroup, true, nil
}

func (erufuncTool *ErufunctionsTool) ValidateFunc(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool ValidateFunc - Start")
	p := ErufuncValidateFuncParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/store/func/validate")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, p.FuncGroup)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, p.FuncGroup, true, nil
}

func (erufuncTool *ErufunctionsTool) RemoveFunc(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool RemoveFunc - Start")
	p := ErufuncRemoveFuncParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/store/", p.ProjectId, "/func/remove/", p.FuncName)
	reqBody := map[string]interface{}{"project_id": p.ProjectId, "func_name": p.FuncName}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodDelete, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, reqBody, true, nil
}

func (erufuncTool *ErufunctionsTool) RunFunc(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool RunFunc - Start")
	p := ErufuncRunFuncParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.EndFuncStepName != "" && p.FuncStepName == "" {
		return nil, nil, false, errors.New("func_step_name is required when end_func_step_name is provided")
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}

	pathParts := []string{baseUrl, "/store/", p.ProjectId, "/func/run"}
	if p.FuncStepName != "" {
		pathParts = append(pathParts, "/", p.FuncStepName)
	}
	if p.EndFuncStepName != "" {
		pathParts = append(pathParts, "/", p.EndFuncStepName)
	}
	url := strings.Join(pathParts, "")

	body := map[string]interface{}{
		"func": p.Func,
	}
	if p.Body != nil {
		body["body"] = p.Body
	}
	if p.ReqVars != nil {
		body["req_vars"] = p.ReqVars
	}
	if p.ResVars != nil {
		body["res_vars"] = p.ResVars
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (erufuncTool *ErufunctionsTool) ExecuteTemplate(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool ExecuteTemplate - Start")
	p := ErufuncExecuteTemplateParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/store/template/execute")
	body := map[string]interface{}{
		"Name":     p.Name,
		"Template": p.Template,
		"Object":   p.Object,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (erufuncTool *ErufunctionsTool) ListMyQueries(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool ListMyQueries - Start")
	p := ErufuncListMyQueriesParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/store/", p.ProjectId, "/myquery/list")
	reqBody := map[string]interface{}{"project_id": p.ProjectId}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, reqBody, true, nil
}

func (erufuncTool *ErufunctionsTool) ListFuncs(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool ListFuncs - Start")
	p := ErufuncListFuncsParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/store/", p.ProjectId, "/func/list")
	reqBody := map[string]interface{}{"project_id": p.ProjectId}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, reqBody, true, nil
}

func (erufuncTool *ErufunctionsTool) ListAgents(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool ListAgents - Start")
	p := ErufuncListAgentsParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	tenantSegment := ""
	if p.TenantId != "" {
		tenantSegment = fmt.Sprint("/", p.TenantId)
	}
	url := fmt.Sprint(baseUrl, "/store/", p.ProjectId, tenantSegment, "/agent/list")
	reqBody := map[string]interface{}{"project_id": p.ProjectId, "tenant_id": p.TenantId}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, reqBody, true, nil
}

func (erufuncTool *ErufunctionsTool) ListTools(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool ListTools - Start")
	p := ErufuncListToolsParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	tenantSegment := ""
	if p.TenantId != "" {
		tenantSegment = fmt.Sprint("/", p.TenantId)
	}
	url := fmt.Sprint(baseUrl, "/store/", p.ProjectId, tenantSegment, "/tool/list")
	reqBody := map[string]interface{}{"project_id": p.ProjectId, "tenant_id": p.TenantId}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, reqBody, true, nil
}

func (erufuncTool *ErufunctionsTool) ScheduleFunc(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool ScheduleFunc - Start")
	p := ErufuncScheduleFuncParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", p.ProjectId, "/schedule/func/", p.FuncName)
	body := map[string]interface{}{"schedule": p.Schedule}
	for k, v := range p.Body {
		if k == "schedule" {
			continue
		}
		body[k] = v
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (erufuncTool *ErufunctionsTool) UnScheduleFunc(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("erufuncTool UnScheduleFunc - Start")
	p := ErufuncUnScheduleFuncParams{}
	if err = erufuncTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := erufuncTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", p.ProjectId, "/unschedule/func/", p.JobId)
	reqBody := map[string]interface{}{"project_id": p.ProjectId, "job_id": p.JobId}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodDelete, url, erufuncTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, reqBody, true, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		ToolType:    "Erufunctions",
		Category:    "Data",
		Description: "Eru functions service for saving, validating, running, scheduling functions, executing templates, and listing project entities",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(erufunctionsToolActions))
			for i, a := range erufunctionsToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(ErufunctionsTool{}), []string{}),
	})
}
