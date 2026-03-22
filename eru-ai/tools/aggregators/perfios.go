package aggregators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	ProfileAction                 = "profile"
	FinancialsAction              = "financials"
	FinancialsLLPAction           = "financials_llp"
	DocumentDownloadRequestAction = "document_download_request"
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

func (perfiosTool *PerfiosTool) GetActionsList() []string {
	actions := []string{}
	for _, action := range perfiosToolActions {
		actions = append(actions, action.ActionName)
	}
	return actions
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
	switch actionName {
	case ProfileAction:
		return perfiosTool.Profile(ctx, projectId, tenantId, params)
	case FinancialsAction:
		return perfiosTool.Financials(ctx, projectId, tenantId, params)
	case FinancialsLLPAction:
		return perfiosTool.FinancialsLLP(ctx, projectId, tenantId, params)
	case DocumentDownloadRequestAction:
		return perfiosTool.DocumentDownloadRequest(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (perfiosTool *PerfiosTool) Profile(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("PerfiosTool Profile - Start")

	id, ok := params["id"].(string)
	if !ok || id == "" {
		return nil, false, fmt.Errorf("id is required and must be a non-empty string")
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
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, false, nil
}

func (perfiosTool *PerfiosTool) Financials(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	return perfiosTool.executeFinancials(ctx, "/v3/corp/docs/financialSummary/", params, false)
}

func (perfiosTool *PerfiosTool) FinancialsLLP(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	return perfiosTool.executeFinancials(ctx, "/v3/corp/docs/llpFinancialSummary/", params, true)
}

func (perfiosTool *PerfiosTool) executeFinancials(ctx context.Context, endpoint string, params map[string]interface{}, isLLP bool) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("PerfiosTool executeFinancials - Start")

	id, ok := params["id"].(string)
	if !ok || id == "" {
		return nil, false, fmt.Errorf("id is required and must be a non-empty string")
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
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, false, nil
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

func (perfiosTool *PerfiosTool) DocumentDownloadRequest(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("PerfiosTool DocumentDownloadRequest - Start")

	id, ok := params["id"].(string)
	if !ok || id == "" {
		return nil, false, fmt.Errorf("id is required and must be a non-empty string")
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
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, false, nil
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
