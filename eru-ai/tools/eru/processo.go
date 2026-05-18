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
	utils "github.com/eru-tech/eru/eru-utils"
)

type ProcessoSaveApiParams struct {
	ApiName     string                 `json:"api_name" eru:"required" desc:"unique name of the api to save"`
	ApiCategory string                 `json:"api_category" eru:"required" desc:"category of the api"`
	OrgId       string                 `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId   string                 `json:"process_id" eru:"required" desc:"process id"`
	ApiDef      map[string]interface{} `json:"api_def" eru:"required" desc:"api definition (function group json with func_category_name, func_group_name, func_steps)"`
}

type ProcessoExecuteApiParams struct {
	ApiName   string                 `json:"api_name" eru:"required" desc:"name of the api to execute"`
	OrgId     string                 `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId string                 `json:"process_id" eru:"required" desc:"process id"`
	Body      map[string]interface{} `json:"body" desc:"additional key value pairs required by the api" default:"{}"`
}

type ProcessoTool struct {
	tools.Tool
	ProjectId string `json:"project_id" desc:"processo project id used in the url path" default:"processo"`
}

const (
	SaveApi    = "save_api"
	ExecuteApi = "execute_api"
)

var processoToolActions = []tools.ToolAction{
	{
		ActionName:   SaveApi,
		Description:  "Save an api definition (function group) under processo for an org and process",
		SystemPrompt: "This tool saves an api definition (function group) under processo for an org and process. The api_def follows the eru function group json structure with func_category_name, func_group_name and func_steps.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoSaveApiParams{}), []string{})
		},
	},
	{
		ActionName:   ExecuteApi,
		Description:  "Execute a previously saved processo api by name with additional key value pairs as required by the api",
		SystemPrompt: "This tool executes a previously saved processo api by name. Pass api_name, org_id, process_id and any other key value pairs required by the api via the body attribute.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ProcessoExecuteApiParams{}), []string{})
		},
	},
}

func (processoTool *ProcessoTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(processoToolActions))
	for i, action := range processoToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (processoTool *ProcessoTool) GetActions() []tools.ToolAction {
	return processoToolActions
}

func (processoTool *ProcessoTool) GetSpec() tools.Tooling {
	return processoTool
}

func (processoTool *ProcessoTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &processoTool); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (processoTool *ProcessoTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &ProcessoTool{}
	if err := json.Unmarshal(toolObjJson, newTool); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return newTool, nil
}

func (processoTool *ProcessoTool) GetBytes(ctx context.Context) ([]byte, error) {
	b, err := json.Marshal(processoTool)
	if err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return b, nil
}

func (processoTool *ProcessoTool) SetToolAction(actionName string) {
	for _, action := range processoToolActions {
		if action.ActionName == actionName {
			processoTool.ToolAction = action
			return
		}
	}
	processoTool.ToolAction = tools.ToolAction{}
}

func (processoTool *ProcessoTool) GetAttribute(ctx context.Context, attributeName string) (interface{}, error) {
	switch attributeName {
	case "tool_name":
		return processoTool.ToolName, nil
	case "tool_type":
		return processoTool.ToolType, nil
	case "system_prompt":
		return processoTool.SystemPrompt, nil
	case "output_schema":
		return processoTool.OutputSchema, nil
	case "parameters":
		return processoTool.Parameters, nil
	case "description":
		return processoTool.Description, nil
	case "project_id":
		return processoTool.ProjectId, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (processoTool *ProcessoTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) error {
	switch attributeName {
	case "tool_name":
		processoTool.ToolName = attributeValue.(string)
	case "tool_type":
		processoTool.ToolType = attributeValue.(string)
	case "system_prompt":
		processoTool.SystemPrompt = attributeValue.(string)
	case "output_schema":
		processoTool.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		processoTool.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		processoTool.Description = attributeValue.(string)
	case "project_id":
		processoTool.ProjectId = attributeValue.(string)
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (processoTool *ProcessoTool) getEruFuncBaseUrl(ctx context.Context) (string, error) {
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

func (processoTool *ProcessoTool) buildHeaders(ctx context.Context) http.Header {
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")
	return headers
}

func (processoTool *ProcessoTool) projectIdSegment() string {
	if processoTool.ProjectId == "" {
		return "processo"
	}
	return processoTool.ProjectId
}

func (processoTool *ProcessoTool) unmarshalParams(ctx context.Context, params map[string]interface{}, target interface{}) error {
	b, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("error marshalling params: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return logs.Err(ctx, err, "")
	}
	return nil
}

func (processoTool *ProcessoTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case SaveApi:
		toolResult, toolRequest, persistStore, err = processoTool.SaveApi(ctx, projectId, tenantId, params)
	case ExecuteApi:
		toolResult, toolRequest, persistStore, err = processoTool.ExecuteApi(ctx, projectId, tenantId, params)
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

		hookResult, hookErr := processoTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if hookErr != nil {
			logs.WithContext(bgCtx).Error(hookErr.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (processoTool *ProcessoTool) SaveApi(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool SaveApi - Start")
	p := ProcessoSaveApiParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/save_apis")
	body := map[string]interface{}{
		"api_name":     p.ApiName,
		"api_category": p.ApiCategory,
		"org_id":       p.OrgId,
		"process_id":   p.ProcessId,
		"api_def":      p.ApiDef,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func (processoTool *ProcessoTool) ExecuteApi(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("processoTool ExecuteApi - Start")
	p := ProcessoExecuteApiParams{}
	if err = processoTool.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	baseUrl, err := processoTool.getEruFuncBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(baseUrl, "/", processoTool.projectIdSegment(), "/func/exec_api")
	body := map[string]interface{}{
		"api_name":   p.ApiName,
		"org_id":     p.OrgId,
		"process_id": p.ProcessId,
	}
	for k, v := range p.Body {
		if _, exists := body[k]; exists {
			continue
		}
		body[k] = v
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, processoTool.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult = map[string]interface{}{"result": res}
	return toolResult, body, true, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		ToolType:    "PROCESSO",
		Category:    "Data",
		Description: "Processo tool to save and execute apis (function groups) for an org and process via eru-functions service",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(processoToolActions))
			for i, a := range processoToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(ProcessoTool{}), []string{}),
	})
}
