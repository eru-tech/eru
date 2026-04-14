package esign

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
	Initiate = "initiate"
	Callback = "callback"
)

const (
	PichainBaseUrl    = "https://api.pichainlabs.com/v1/onboard"
	INSERT_FUNC_ASYNC = "insert into eruai_cb_pichain (project_id, tenant_id, request_body, request_params) values ($1, $2, $3, $4)"
)

type PichainTool struct {
	tools.Tool
	PichainAccount PichainAccount `json:"pichain_account"`
}

type PichainAccount struct {
	OrgId  string `json:"org_id" eru:"required"`
	ApiKey string `json:"api_key" eru:"required"`
}

type Signee struct {
	Name          string `json:"name" eru:"required"`
	Email         string `json:"email" eru:"required"`
	PhoneNumber   string `json:"phoneNumber" eru:"required"`
	SignatureType string `json:"signature_type" eru:"required"`
	DueDate       string `json:"duedate" eru:"required"`
	Observer      bool   `json:"observer"`
	PageNo        string `json:"pageNo" eru:"required"`
	Reason        string `json:"reason" eru:"required"`
	Location      string `json:"location" eru:"required"`
	Rectangle     string `json:"rectangle" eru:"required"`
}

type InitiateParams struct {
	Id              string   `json:"id" eru:"required"`
	Data            string   `json:"data" eru:"required"`
	Signees         []Signee `json:"signees" eru:"required"`
	ReturnUrl       string   `json:"return_url"`
	CustomReference string   `json:"custom_reference"`
	OtpRequired     string   `json:"otp_required"`
	FaceCapture     string   `json:"face_capture"`
	LocationCapture string   `json:"location_capture"`
	EStampRequired  string   `json:"e_stamp_required"`
	SignatureExpiry string   `json:"signature_expiry"`
	File            string   `json:"file"`
	FileName        string   `json:"file_name"`
}

func (pichainTool *PichainTool) GetActionsList() []tools.ActionInfo {
	return []tools.ActionInfo{
		{Name: Initiate},
		{Name: Callback},
	}
}

func (pichainTool *PichainTool) GetToolCallback() tools.ToolCallback {
	return tools.ToolCallback{
		ResponseContentType: "application/json",
	}
}

func (pichainTool *PichainTool) Callback(ctx context.Context, projectId string, tenantId string, actionName string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Callback Execute - Start")

	gm := server.GetGlobalGoroutineManager(ctx)
	gm.SafeGoWithRestartBehavior("pichain-callback", func(bgCtx context.Context) {
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
		insertQueryFuncAsync.Query = pichainTool.ToolDb.GetDbQuery(bgCtx, INSERT_FUNC_ASYNC)
		insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, projectId, tenantId, string(bodyBytes), string(paramBytes))
		insertQueryFuncAsync.Rank = 1
		insertQueries = append(insertQueries, &insertQueryFuncAsync)
		_, insertOutputErr := utils.ExecuteDbSave(bgCtx, pichainTool.ToolDb.GetConn(), insertQueries)
		if insertOutputErr != nil {
			logs.WithContext(bgCtx).Error(insertOutputErr.Error())
			return
		}

		hookResult, err := pichainTool.ExecuteHook(bgCtx, "clbk", "", projectId, tenantId, body, params)
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

func (pichainTool *PichainTool) GetSpec() tools.Tooling {
	return pichainTool
}

func (pichainTool *PichainTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &pichainTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (pichainTool *PichainTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("PichainTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case Initiate:
		toolResult, toolRequest, persistStore, err = pichainTool.Initiate(ctx, params)
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

		hookResult, err := pichainTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (pichainTool *PichainTool) Initiate(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Initiate Execute - Start")

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to marshal params: %s", err.Error()), "failed to marshal params")
		return nil, nil, false, err
	}

	initiateParams := InitiateParams{}
	err = json.Unmarshal(paramsBytes, &initiateParams)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to unmarshal initiate params: %s", err.Error()), "failed to unmarshal initiate params")
		return nil, nil, false, err
	}

	err = utils.ValidateStruct(ctx, initiateParams, "")
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("invalid initiate params: %s", err.Error()), fmt.Sprintf("invalid initiate params: %s", err.Error()))
		return nil, nil, false, err
	}

	url := fmt.Sprintf("%s/initiate_contract", PichainBaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "multipart/form-data")
	headers.Set("apikey", pichainTool.PichainAccount.ApiKey)

	formData := map[string]string{
		"data":  initiateParams.Data,
		"orgId": pichainTool.PichainAccount.OrgId,
	}

	if initiateParams.ReturnUrl != "" {
		formData["return_url"] = initiateParams.ReturnUrl
	}
	if initiateParams.CustomReference != "" {
		formData["custom_reference"] = initiateParams.CustomReference
	}
	if initiateParams.OtpRequired != "" {
		formData["otpRequired"] = initiateParams.OtpRequired
	}
	if initiateParams.FaceCapture != "" {
		formData["face_capture"] = initiateParams.FaceCapture
	}
	if initiateParams.LocationCapture != "" {
		formData["location_capture"] = initiateParams.LocationCapture
	}
	if initiateParams.EStampRequired != "" {
		formData["eStampRequired"] = initiateParams.EStampRequired
	}
	if initiateParams.SignatureExpiry != "" {
		formData["signature_expiry"] = initiateParams.SignatureExpiry
	}
	if initiateParams.File != "" {
		formData["file"] = initiateParams.File
	}
	if initiateParams.FileName != "" {
		formData["file_name"] = initiateParams.FileName
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, formData, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to initiate contract: %s", err.Error()), "failed to initiate contract")
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["response"] = res
	}

	return toolResult, formData, false, nil
}

func (pichainTool *PichainTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(pichainTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (pichainTool *PichainTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &PichainTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		ToolType:     "PICHAIN",
		Category:     "ESign",
		Description:  "Pichain e-signature integration for contract initiation and callbacks",
		Actions:      []tools.ActionInfo{{Name: Initiate}, {Name: Callback}},
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(PichainTool{}), []string{}),
	})
}
