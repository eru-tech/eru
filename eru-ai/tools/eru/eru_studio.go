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

type EruStudioFetchPagesParams struct {
	OrgId     string `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId string `json:"process_id" eru:"required" desc:"process id"`
}

type EruStudioFetchPageParams struct {
	OrgId     string `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId string `json:"process_id" eru:"required" desc:"process id"`
	PageId    string `json:"page_id" eru:"required" desc:"page id to fetch"`
}

type EruStudioSavePageParams struct {
	OrgId     string                 `json:"org_id" eru:"required" desc:"organization id"`
	ProcessId string                 `json:"process_id" eru:"required" desc:"process id"`
	PageId    string                 `json:"page_id" eru:"required" desc:"page id to save"`
	PageName  string                 `json:"page_name" eru:"required" desc:"page name"`
	PageDef   map[string]interface{} `json:"page_def" eru:"required" desc:"page definition as JSON string"`
}

type EruStudioTool struct {
	tools.Tool
	EruqlProjectId string `json:"eruql_project_id" desc:"eruql project id used for myquery/execute calls" default:"processo"`
}

const (
	FetchPages = "fetch_pages"
	FetchPage  = "fetch_page"
	SavePage   = "save_page"
)

var eruStudioToolActions = []tools.ToolAction{
	{
		ActionName:   FetchPages,
		Description:  "Fetch all pages for an org and process",
		SystemPrompt: "Fetch all pages for an org and process",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruStudioFetchPagesParams{}), []string{})
		},
	},
	{
		ActionName:   FetchPage,
		Description:  "Fetch a single page by id",
		SystemPrompt: "Fetch a single page by id",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruStudioFetchPageParams{}), []string{})
		},
	},
	{
		ActionName:   SavePage,
		Description:  "Save a page definition",
		SystemPrompt: "Save a page definition",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(EruStudioSavePageParams{}), []string{})
		},
	},
}

func (t *EruStudioTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(eruStudioToolActions))
	for i, action := range eruStudioToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (t *EruStudioTool) GetActions() []tools.ToolAction {
	return eruStudioToolActions
}

func (t *EruStudioTool) GetSpec() tools.Tooling {
	return t
}

func (t *EruStudioTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &t); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (t *EruStudioTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &EruStudioTool{}
	if err := json.Unmarshal(toolObjJson, newTool); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return newTool, nil
}

func (t *EruStudioTool) GetBytes(ctx context.Context) ([]byte, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return b, nil
}

func (t *EruStudioTool) SetToolAction(actionName string) {
	for _, action := range eruStudioToolActions {
		if action.ActionName == actionName {
			t.ToolAction = action
			return
		}
	}
	t.ToolAction = tools.ToolAction{}
}

func (t *EruStudioTool) GetAttribute(ctx context.Context, attributeName string) (interface{}, error) {
	switch attributeName {
	case "tool_name":
		return t.ToolName, nil
	case "tool_type":
		return t.ToolType, nil
	case "system_prompt":
		return t.SystemPrompt, nil
	case "output_schema":
		return t.OutputSchema, nil
	case "parameters":
		return t.Parameters, nil
	case "description":
		return t.Description, nil
	case "eruql_project_id":
		return t.EruqlProjectId, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (t *EruStudioTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) error {
	switch attributeName {
	case "tool_name":
		t.ToolName = attributeValue.(string)
	case "tool_type":
		t.ToolType = attributeValue.(string)
	case "system_prompt":
		t.SystemPrompt = attributeValue.(string)
	case "output_schema":
		t.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		t.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		t.Description = attributeValue.(string)
	case "eruql_project_id":
		t.EruqlProjectId = attributeValue.(string)
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (t *EruStudioTool) getEruqlBaseUrl(ctx context.Context) (string, error) {
	v := ctx.Value("eruqlbaseurl")
	if v == nil {
		return "", errors.New("eruqlbaseurl not found in context")
	}
	s, ok := v.(string)
	if !ok {
		return "", errors.New("eruqlbaseurl is not a string")
	}
	if s == "" {
		return "", errors.New("eruqlbaseurl is not set")
	}
	return s, nil
}

func (t *EruStudioTool) buildHeaders(ctx context.Context) http.Header {
	headers := http.Header{}
	claims := ctx.Value("claims")
	if claims != nil {
		headers.Add("claims", fmt.Sprint(claims))
	}
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")
	return headers
}

func (t *EruStudioTool) projectId() string {
	if t.EruqlProjectId == "" {
		return "processo"
	}
	return t.EruqlProjectId
}

func (t *EruStudioTool) unmarshalParams(ctx context.Context, params map[string]interface{}, target interface{}) error {
	b, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("error marshalling params: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return logs.Err(ctx, err, "")
	}
	return nil
}

func (t *EruStudioTool) callMyQuery(ctx context.Context, queryName string, body map[string]interface{}) (map[string]interface{}, error) {
	baseUrl, err := t.getEruqlBaseUrl(ctx)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprint(baseUrl, "/store/", t.projectId(), "/myquery/execute/", queryName)
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, t.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return map[string]interface{}{"result": res}, nil
}

func (t *EruStudioTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("EruStudioTool Execute - Start")
	switch actionName {
	case FetchPages:
		toolResult, _, persistStore, err = t.FetchPages(ctx, projectId, tenantId, params)
	case FetchPage:
		toolResult, _, persistStore, err = t.FetchPage(ctx, projectId, tenantId, params)
	case SavePage:
		toolResult, _, persistStore, err = t.SavePage(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
	return toolResult, persistStore, err
}

func (t *EruStudioTool) FetchPages(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, interface{}, bool, error) {
	logs.WithContext(ctx).Debug("EruStudioTool FetchPages - Start")
	p := EruStudioFetchPagesParams{}
	if err := t.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	body := map[string]interface{}{
		"org_id":     p.OrgId,
		"process_id": p.ProcessId,
	}
	res, err := t.callMyQuery(ctx, FetchPages, body)
	if err != nil {
		return nil, nil, false, err
	}
	return res, body, true, nil
}

func (t *EruStudioTool) FetchPage(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, interface{}, bool, error) {
	logs.WithContext(ctx).Debug("EruStudioTool FetchPage - Start")
	p := EruStudioFetchPageParams{}
	if err := t.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	body := map[string]interface{}{
		"org_id":     p.OrgId,
		"process_id": p.ProcessId,
		"page_id":    p.PageId,
	}
	res, err := t.callMyQuery(ctx, FetchPage, body)
	if err != nil {
		return nil, nil, false, err
	}
	return res, body, true, nil
}

func (t *EruStudioTool) SavePage(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, interface{}, bool, error) {
	logs.WithContext(ctx).Debug("EruStudioTool SavePage - Start")
	p := EruStudioSavePageParams{}
	if err := t.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	body := map[string]interface{}{
		"docs": []map[string]interface{}{
			{
				"page_id":    p.PageId,
				"page_def":   p.PageDef,
				"org_id":     p.OrgId,
				"process_id": p.ProcessId,
				"page_name":  p.PageName,
			},
		},
	}
	res, err := t.callMyQuery(ctx, SavePage, body)
	if err != nil {
		return nil, nil, false, err
	}
	return res, body, true, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:      false,
		ToolType:    "ERUSTUDIO",
		Category:    "Data",
		Description: "Eru Studio tool to manage pages (fetch and edit) via eru-ql myquery endpoints",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(eruStudioToolActions))
			for i, a := range eruStudioToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(EruStudioTool{}), []string{}),
	})
}
