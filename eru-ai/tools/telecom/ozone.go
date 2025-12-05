package telecom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	Call     = "call"
	FetchCDR = "fetch_cdr"
)

const (
	OzoneBaseUrl = "https://in1-ccaas-api.ozonetel.com"
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

func (ozoneTool *OzoneTool) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, Call)
	actions = append(actions, FetchCDR)
	return actions
}

func (ozoneTool *OzoneTool) GetMcpTools() []tools.McpToolList {
	mcpTools := []tools.McpToolList{}
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        Call,
		ToolDescription: "Make a manual phone dial using Ozone API",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", Call),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        FetchCDR,
		ToolDescription: "Fetch CDR (Call Detail Record) details from Ozone API",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", FetchCDR),
	})
	return mcpTools
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
	switch actionName {
	case Call:
		return ozoneTool.Call(ctx, params)
	case FetchCDR:
		return ozoneTool.FetchCDR(ctx, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (ozoneTool *OzoneTool) Call(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Call Execute - Start")

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to marshal params: %s", err.Error()), "failed to marshal params")
		return nil, false, err
	}

	callParams := CallParams{
		CheckStatus: true,
	}
	err = json.Unmarshal(paramsBytes, &callParams)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to unmarshal call params: %s", err.Error()), "failed to unmarshal call params")
		return nil, false, err
	}

	err = utils.ValidateStruct(ctx, callParams, "")
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("invalid call params: %s", err.Error()), fmt.Sprintf("invalid call params: %s", err.Error()))
		return nil, false, err
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

func (ozoneTool *OzoneTool) FetchCDR(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("FetchCDR Execute - Start")

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to marshal params: %s", err.Error()), "failed to marshal params")
		return nil, false, err
	}

	fetchCDRParams := FetchCDRParams{}
	err = json.Unmarshal(paramsBytes, &fetchCDRParams)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to unmarshal fetch CDR params: %s", err.Error()), "failed to unmarshal fetch CDR params")
		return nil, false, err
	}

	err = utils.ValidateStruct(ctx, fetchCDRParams, "")
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("invalid fetch CDR params: %s", err.Error()), fmt.Sprintf("invalid fetch CDR params: %s", err.Error()))
		return nil, false, err
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
		return toolResult, false, err
	}

	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, false, nil
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
	err := json.Unmarshal(toolObjJson, &ozoneTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return ozoneTool, nil
}
