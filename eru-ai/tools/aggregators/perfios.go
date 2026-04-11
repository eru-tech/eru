package aggregators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"errors"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	ProfileAction                 = "profile"
	FinancialsAction              = "financials"
	FinancialsLLPAction           = "financials_llp"
	DocumentDownloadRequestAction = "document_download_request"
	CallbackAction                = "callback"
	INSERT_FUNC_ASYNC             = "insert into eruai_cb_perfios (project_id, tenant_id, request_body, request_params) values ($1, $2, $3, $4)"
)

type PerfiosTool struct {
	tools.Tool
	BaseUrl string `json:"base_url" eru:"required"`
	ApiKey  string `json:"api_key" eru:"required"`
}

var perfiosToolActions = []tools.ToolAction{
	{
		ActionName:   ProfileAction,
		Description:  "Get profile from Perfios",
		SystemPrompt: "Get profile from Perfios API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"id": {Type: "string", Description: "ID for profile stitch"},
			},
			Required: []string{"id"},
		},
	},
	{
		ActionName:   FinancialsAction,
		Description:  "Get financial summary from Perfios",
		SystemPrompt: "Get financial summary from Perfios API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"id":            {Type: "string", Description: "Entity ID for financial summary"},
				"consent":       {Type: "string", Description: "Consent flag (default: Y)"},
				"financialYear": {Type: "array", Items: &models.JSONSchema{Type: "string"}, Description: "Financial years list"},
				"financialType": {Type: "string", Description: "Financial type (default: both)"},
			},
			Required: []string{"id"},
		},
	},
	{
		ActionName:   FinancialsLLPAction,
		Description:  "Get LLP financial summary from Perfios",
		SystemPrompt: "Get LLP financial summary from Perfios API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"id":            {Type: "string", Description: "Entity ID for LLP financial summary"},
				"consent":       {Type: "string", Description: "Consent flag (default: Y)"},
				"financialYear": {Type: "array", Items: &models.JSONSchema{Type: "string"}, Description: "Financial years list"},
			},
			Required: []string{"id"},
		},
	},
	{
		ActionName:   DocumentDownloadRequestAction,
		Description:  "Request corporate document download from Perfios",
		SystemPrompt: "Request corporate document download from Perfios API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"id":            {Type: "string", Description: "Entity ID for document download"},
				"doc_type":      {Type: "array", Items: &models.JSONSchema{Type: "string"}, Description: "Document types list"},
				"financialYear": {Type: "array", Items: &models.JSONSchema{Type: "string"}, Description: "Financial years list"},
				"notify_email":  {Type: "string", Description: "Email to notify"},
				"periodFrom":    {Type: "string", Description: "Period from (YYYY)"},
				"periodTo":      {Type: "string", Description: "Period to (YYYY)"},
				"file_format":   {Type: "string", Description: "File format"},
				"webhook":       {Type: "boolean", Description: "Enable webhook callback"},
				"webhook_url":   {Type: "string", Description: "Webhook URL for callback"},
				"consent":       {Type: "string", Description: "Consent flag (default: y)"},
			},
			Required: []string{"id"},
		},
	},
}

func (perfiosTool *PerfiosTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(perfiosToolActions))
	for i, action := range perfiosToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (perfiosTool *PerfiosTool) GetActions() []tools.ToolAction {
	return perfiosToolActions
}

func (perfiosTool *PerfiosTool) GetSpec() tools.Tooling {
	return perfiosTool
}

func (perfiosTool *PerfiosTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &perfiosTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (perfiosTool *PerfiosTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("PerfiosTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case ProfileAction:
		toolResult, toolRequest, persistStore, err = perfiosTool.Profile(ctx, projectId, tenantId, params)
	case FinancialsAction:
		toolResult, toolRequest, persistStore, err = perfiosTool.Financials(ctx, projectId, tenantId, params)
	case FinancialsLLPAction:
		toolResult, toolRequest, persistStore, err = perfiosTool.FinancialsLLP(ctx, projectId, tenantId, params)
	case DocumentDownloadRequestAction:
		toolResult, toolRequest, persistStore, err = perfiosTool.DocumentDownloadRequest(ctx, projectId, tenantId, params)
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

		hookResult, err := perfiosTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (perfiosTool *PerfiosTool) Profile(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("PerfiosTool Profile - Start")

	id, ok := params["id"].(string)
	if !ok || id == "" {
		return nil, nil, false, fmt.Errorf("id is required and must be a non-empty string")
	}
	payload := map[string]interface{}{
		"id": id,
	}
	url := fmt.Sprintf("%s/v3/stitch-profile/module/", perfiosTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("x-karza-key", perfiosTool.ApiKey)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{"id": id}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (perfiosTool *PerfiosTool) Financials(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	return perfiosTool.executeFinancials(ctx, "/v3/corp/docs/financialSummary/", params, false)
}

func (perfiosTool *PerfiosTool) FinancialsLLP(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	return perfiosTool.executeFinancials(ctx, "/v3/corp/docs/llpFinancialSummary/", params, true)
}

func (perfiosTool *PerfiosTool) executeFinancials(ctx context.Context, endpoint string, params map[string]interface{}, isLLP bool) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("PerfiosTool executeFinancials - Start")

	id, ok := params["id"].(string)
	if !ok || id == "" {
		return nil, nil, false, fmt.Errorf("id is required and must be a non-empty string")
	}

	consent, ok := params["consent"].(string)
	if !ok || consent == "" {
		consent = "Y"
	}

	var financialYears []string
	if fyRaw, ok := params["financialYear"].([]interface{}); ok && len(fyRaw) > 0 {
		for _, v := range fyRaw {
			if s, ok := v.(string); ok {
				financialYears = append(financialYears, s)
			}
		}
	} else if fyRaw, ok := params["financialYear"].([]string); ok && len(fyRaw) > 0 {
		financialYears = fyRaw
	} else {
		financialYears = perfiosTool.generateFinancialYears()
	}

	payload := map[string]interface{}{
		"entityId":      id,
		"consent":       consent,
		"financialYear": financialYears,
	}

	if !isLLP {
		financialType, ok := params["financialType"].(string)
		if !ok || financialType == "" {
			financialType = "both"
		}
		payload["financialType"] = financialType
	}

	url := fmt.Sprintf("%s%s", perfiosTool.BaseUrl, endpoint)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("x-karza-key", perfiosTool.ApiKey)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{"id": id}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (perfiosTool *PerfiosTool) generateFinancialYears() []string {
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	var startYear int
	if month >= 1 && month <= 3 {
		startYear = year - 1
	} else {
		startYear = year
	}

	fys := make([]string, 5)
	for i := 0; i < 5; i++ {
		curYear := startYear - i
		nextYearShort := (curYear + 1) % 100
		fys[i] = fmt.Sprintf("%d-%02d", curYear, nextYearShort)
	}
	return fys
}

func (perfiosTool *PerfiosTool) DocumentDownloadRequest(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("PerfiosTool DocumentDownloadRequest - Start")

	id, ok := params["id"].(string)
	if !ok || id == "" {
		return nil, nil, false, fmt.Errorf("id is required and must be a non-empty string")
	}

	consent, ok := params["consent"].(string)
	if !ok || consent == "" {
		consent = "y"
	}

	var financialYears []string
	if fyRaw, ok := params["financial_year"].([]interface{}); ok && len(fyRaw) > 0 {
		for _, v := range fyRaw {
			if s, ok := v.(string); ok {
				financialYears = append(financialYears, s)
			}
		}
	} else if fyRaw, ok := params["financial_year"].([]string); ok && len(fyRaw) > 0 {
		financialYears = fyRaw
	} else {
		financialYears = perfiosTool.generateFinancialYears()
	}

	docType := []string{}
	if dtRaw, ok := params["doc_type"].([]interface{}); ok {
		for _, v := range dtRaw {
			if s, ok := v.(string); ok {
				docType = append(docType, s)
			}
		}
	} else if dtRaw, ok := params["doc_type"].([]string); ok {
		docType = dtRaw
	}

	webhook := true
	if w, ok := params["webhook"].(bool); ok {
		webhook = w
	}

	periodFrom, ok := params["period_from"].(string)
	if !ok || periodFrom == "" {
		if len(financialYears) > 0 {
			minYear := financialYears[0][:4]
			for _, fy := range financialYears {
				if fy[:4] < minYear {
					minYear = fy[:4]
				}
			}
			periodFrom = minYear
		}
	}

	periodTo, ok := params["period_to"].(string)
	if !ok || periodTo == "" {
		if len(financialYears) > 0 {
			maxYear := financialYears[0][:4]
			for _, fy := range financialYears {
				if fy[:4] > maxYear {
					maxYear = fy[:4]
				}
			}
			periodTo = maxYear
		}
	}

	payload := map[string]interface{}{
		"entityId":      id,
		"docType":       docType,
		"financialYear": financialYears,
		"notifyToEmail": params["notify_email"],
		"periodFrom":    periodFrom,
		"periodTo":      periodTo,
		"fileFormat":    params["file_format"],
		"webhook":       webhook,
		"webhookUrl":    params["webhook_url"],
		"consent":       consent,
	}

	url := fmt.Sprintf("%s/v1/corp/docs/request-details", perfiosTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("x-karza-key", perfiosTool.ApiKey)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{"id": id}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (perfiosTool *PerfiosTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &PerfiosTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}

func (perfiosTool *PerfiosTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(perfiosTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		ToolType:    "PerfiosTool",
		Category:    "Aggregator",
		Description: "Perfios financial data aggregator for bank statements, financials, and customer data",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(perfiosToolActions))
			for i, a := range perfiosToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
	})
}
