package telecom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	"github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	Call     = "call"
	FetchCDR = "fetch_cdr"
	Callback = "callback"
)

const (
	OzoneBaseUrl = "https://in1-ccaas-api.ozonetel.com"
)

const (
	INSERT_FUNC_ASYNC = "insert into eruai_cb_ozonetel (project_id, tenant_id, oz_data, oz_params) values ($1, $2, $3, $4)"
)

type OzoneTool struct {
	tools.Tool
	OzoneAccount OzoneAccount `json:"ozone_account"`
}

type OzoneAccount struct {
	ApiKey   string `json:"api_key" eru:"required"`
	UserName string `json:"user_name" eru:"required"`
}

type CallParams struct {
	PhoneName    string `json:"phone_name" eru:"required"`
	CampaignName string `json:"campaign_name" eru:"required"`
	CustNumber   string `json:"cust_number" eru:"required"`
	Did          string `json:"did" eru:"required"`
	Uui          string `json:"uui,omitempty"`
	CheckStatus  bool   `json:"check_status"`
}

type FetchCDRParams struct {
	FromDate     string `json:"from_date" eru:"required"`
	ToDate       string `json:"to_date" eru:"required"`
	CampaignName string `json:"campaign_name" eru:"required"`
}

func (ozoneTool *OzoneTool) GetActionsList() []tools.ActionInfo {
	return []tools.ActionInfo{
		{Name: Call},
		{Name: FetchCDR},
		{Name: Callback},
	}
}

func (ozoneTool *OzoneTool) GetToolCallback() tools.ToolCallback {
	return tools.ToolCallback{
		ResponseContentType: "application/json",
	}
}

func (ozoneTool *OzoneTool) Callback(ctx context.Context, projectId string, tenantId string, actionName string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Callback Execute - Start")

	// Process the message in a separate goroutine with panic recovery using global GoroutineManager
	gm := server.GetGlobalGoroutineManager(ctx)
	gm.SafeGoWithRestartBehavior("ms-email-callback", func(bgCtx context.Context) {
		// Copy any important values from the original context if needed

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
		if body == nil {
			body = make(map[string]interface{})
		}
		body["tenant_id"] = tenantId
		body["project_id"] = projectId

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		paramsMap := map[string]string{}
		for k, v := range params {
			paramsMap[k] = v[0]
		}
		paramBytes, err := json.Marshal(paramsMap)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}

		var insertQueries []*models.Queries
		insertQueryFuncAsync := models.Queries{}
		insertQueryFuncAsync.Query = ozoneTool.ToolDb.GetDbQuery(bgCtx, INSERT_FUNC_ASYNC)
		insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, projectId, tenantId, string(bodyBytes), string(paramBytes))
		insertQueryFuncAsync.Rank = 1
		insertQueries = append(insertQueries, &insertQueryFuncAsync)
		_, insertOutputErr := utils.ExecuteDbSave(bgCtx, ozoneTool.ToolDb.GetConn(), insertQueries)
		if insertOutputErr != nil {
			logs.WithContext(bgCtx).Error(insertOutputErr.Error())
			return
		}

		hookResult, err := ozoneTool.ExecuteHook(bgCtx, "clbk", "", projectId, tenantId, body, params)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	callbackResultMap := map[string]string{
		"Status": "Success",
	}
	return callbackResultMap, false, nil
}

func (ozoneTool *OzoneTool) GetSpec() tools.Tooling {
	return ozoneTool
}

func (ozoneTool *OzoneTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &ozoneTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (ozoneTool *OzoneTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("OzoneTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case Call:
		toolResult, toolRequest, persistStore, err = ozoneTool.Call(ctx, params)
	case FetchCDR:
		toolResult, toolRequest, persistStore, err = ozoneTool.FetchCDR(ctx, params)
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

		hookResult, err := ozoneTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (ozoneTool *OzoneTool) Call(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Call Execute - Start")

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to marshal params: %s", err.Error()), "failed to marshal params")
		return nil, nil, false, err
	}

	callParams := CallParams{
		CheckStatus: true,
	}
	err = json.Unmarshal(paramsBytes, &callParams)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to unmarshal call params: %s", err.Error()), "failed to unmarshal call params")
		return nil, nil, false, err
	}

	err = utils.ValidateStruct(ctx, callParams, "")
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("invalid call params: %s", err.Error()), fmt.Sprintf("invalid call params: %s", err.Error()))
		return nil, nil, false, err
	}

	url := fmt.Sprintf("%s/ca_apis/PhoneManualDial", OzoneBaseUrl)
	headers := http.Header{}
	headers.Set("apiKey", ozoneTool.OzoneAccount.ApiKey)
	headers.Set("Content-Type", "application/json")

	payload := map[string]interface{}{
		"userName":     ozoneTool.OzoneAccount.UserName,
		"phoneName":    callParams.PhoneName,
		"campaignName": callParams.CampaignName,
		"custNumber":   callParams.CustNumber,
		"did":          callParams.Did,
		"checkStatus":  callParams.CheckStatus,
	}

	if callParams.Uui != "" {
		payload["uui"] = callParams.Uui
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to make phone call: %s", err.Error()), "failed to make phone call")
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, payload, false, nil
}

func (ozoneTool *OzoneTool) FetchCDR(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("FetchCDR Execute - Start")

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to marshal params: %s", err.Error()), "failed to marshal params")
		return nil, nil, false, err
	}

	fetchCDRParams := FetchCDRParams{}
	err = json.Unmarshal(paramsBytes, &fetchCDRParams)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to unmarshal fetch CDR params: %s", err.Error()), "failed to unmarshal fetch CDR params")
		return nil, nil, false, err
	}

	err = utils.ValidateStruct(ctx, fetchCDRParams, "")
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("invalid fetch CDR params: %s", err.Error()), fmt.Sprintf("invalid fetch CDR params: %s", err.Error()))
		return nil, nil, false, err
	}

	url := fmt.Sprintf("%s/ca_reports/fetchCDRDetails", OzoneBaseUrl)
	headers := http.Header{}
	headers.Set("apiKey", ozoneTool.OzoneAccount.ApiKey)
	headers.Set("Content-Type", "application/json")

	payload := map[string]interface{}{
		"fromDate":     fetchCDRParams.FromDate,
		"toDate":       fetchCDRParams.ToDate,
		"userName":     ozoneTool.OzoneAccount.UserName,
		"campaignName": fetchCDRParams.CampaignName,
	}
	toolResult = make(map[string]interface{})
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return toolResult, nil, false, err
	}

	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (ozoneTool *OzoneTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(ozoneTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (ozoneTool *OzoneTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &OzoneTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		ToolType:     "Ozone",
		Category:     "Telecom",
		Description:  "Ozone telecom integration for voice calls, CDR fetch, and callbacks",
		Actions:      []tools.ActionInfo{{Name: Call}, {Name: FetchCDR}, {Name: Callback}},
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(OzoneTool{}), []string{}),
	})
}
