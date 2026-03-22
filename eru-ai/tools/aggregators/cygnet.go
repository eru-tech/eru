package aggregators

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/eru-tech/eru/eru-ai/tools"
	aes "github.com/eru-tech/eru/eru-crypto/aes"
	rsa "github.com/eru-tech/eru/eru-crypto/rsa"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	Txn   = "ENRICHEDTEST_e74c3568b19b4aee989c"
	IpUse = "127.0.0.1"
)
const (
	Login                       = "login"
	B2BSales                    = "b2bsales"
	B2CSales                    = "b2csales"
	CDNRSales                   = "cdnrsales"
	B2BPurchase                 = "b2bpurchase"
	CDNRPurchase                = "cdnrpurchase"
	HSNSales                    = "hsnsales"
	NILSales                    = "nilsales"
	GenerateOTP                 = "generateotp"
	GenerateSession             = "generatesession"
	DownloadDataRequest         = "downloaddatarequest"
	CreateCustomer              = "createcustomer"
	GSTINSearch                 = "gstinsearch"
	VerifySession               = "verifysession"
	VerifyConsent               = "verifyconsent"
	VerifyDownloadRequestStatus = "verifydownloadrequeststatus"
	BankStatementUpload         = "bankstatementupload"
	BankStatementUploadStatus   = "bankstatementuploadstatus"
)

type CygnetTool struct {
	tools.Tool
	ClientId        string    `json:"client_id" eru:"required"`
	ClientSecret    string    `json:"client_secret" eru:"required"`
	BaseUrl         string    `json:"base_url" eru:"required"`
	User            string    `json:"user" eru:"required"`
	Password        string    `json:"password" eru:"required"`
	CygnetPublicKey string    `json:"cygnet_public_key" eru:"required"`
	AuthToken       string    `json:"-"`
	Expiry          int       `json:"-"`
	expiryTime      time.Time `json:"-"`
	AesKey          []byte    `json:"-"`
}

type CygnetGstDataPayload struct {
	GSTIN string `json:"gstin" eru:"required"`
	Month int    `json:"month" eru:"required"`
	Year  int    `json:"year" eru:"required"`
}

var cygnetToolActions = []tools.ToolAction{
	{
		ActionName:   Login,
		Description:  "Login to Cygnet",
		SystemPrompt: "Login to Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters:   models.JSONSchema{},
	},
	{
		ActionName:   B2BSales,
		Description:  "Get B2B Sales Invoices from Cygnet",
		SystemPrompt: "Get B2B Sales Invoices from Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin": {Type: "string", Description: "GSTIN of the customer"},
				"month": {Type: "integer", Description: "Month (e.g. 1, 2...)"},
				"year":  {Type: "integer", Description: "Year (e.g. 2024)"},
			},
			Required: []string{"gstin", "month", "year"},
		},
	},
	{
		ActionName:   B2CSales,
		Description:  "Get B2C Sales Invoices from Cygnet",
		SystemPrompt: "Get B2C Sales Invoices from Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin": {Type: "string", Description: "GSTIN of the customer"},
				"month": {Type: "integer", Description: "Month (e.g. 1, 2...)"},
				"year":  {Type: "integer", Description: "Year (e.g. 2024)"},
			},
			Required: []string{"gstin", "month", "year"},
		},
	},
	{
		ActionName:   CDNRSales,
		Description:  "Get CD Note Sales from Cygnet",
		SystemPrompt: "Get CD Note Sales from Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin": {Type: "string", Description: "GSTIN of the customer"},
				"month": {Type: "integer", Description: "Month (e.g. 1, 2...)"},
				"year":  {Type: "integer", Description: "Year (e.g. 2024)"},
			},
			Required: []string{"gstin", "month", "year"},
		},
	},
	{
		ActionName:   B2BPurchase,
		Description:  "Get B2B Purchase Invoices from Cygnet",
		SystemPrompt: "Get B2B Purchase Invoices from Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin": {Type: "string", Description: "GSTIN of the customer"},
				"month": {Type: "integer", Description: "Month (e.g. 1, 2...)"},
				"year":  {Type: "integer", Description: "Year (e.g. 2024)"},
			},
			Required: []string{"gstin", "month", "year"},
		},
	},
	{
		ActionName:   CDNRPurchase,
		Description:  "Get CD Note Purchase from Cygnet",
		SystemPrompt: "Get CD Note Purchase from Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin": {Type: "string", Description: "GSTIN of the customer"},
				"month": {Type: "integer", Description: "Month (e.g. 1, 2...)"},
				"year":  {Type: "integer", Description: "Year (e.g. 2024)"},
			},
			Required: []string{"gstin", "month", "year"},
		},
	},
	{
		ActionName:   HSNSales,
		Description:  "Get HSN Summary Sales from Cygnet",
		SystemPrompt: "Get HSN Summary Sales from Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin": {Type: "string", Description: "GSTIN of the customer"},
				"month": {Type: "integer", Description: "Month (e.g. 1, 2...)"},
				"year":  {Type: "integer", Description: "Year (e.g. 2024)"},
			},
			Required: []string{"gstin", "month", "year"},
		},
	},
	{
		ActionName:   NILSales,
		Description:  "Get Nil Exempt Non-GST Sales from Cygnet",
		SystemPrompt: "Get Nil Exempt Non-GST Sales from Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin": {Type: "string", Description: "GSTIN of the customer"},
				"month": {Type: "integer", Description: "Month (e.g. 1, 2...)"},
				"year":  {Type: "integer", Description: "Year (e.g. 2024)"},
			},
			Required: []string{"gstin", "month", "year"},
		},
	},
	{
		ActionName:   GenerateOTP,
		Description:  "Generate OTP for Cygnet",
		SystemPrompt: "Generate OTP for Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"username": {Type: "string", Description: "Username for Cygnet"},
				"gstin":    {Type: "string", Description: "GSTIN of the customer"},
			},
			Required: []string{"username", "gstin"},
		},
	},
	{
		ActionName:   GenerateSession,
		Description:  "Generate Session for Cygnet",
		SystemPrompt: "Generate Session for Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"username": {Type: "string", Description: "Username for Cygnet"},
				"gstin":    {Type: "string", Description: "GSTIN of the customer"},
				"otp":      {Type: "string", Description: "OTP received for Cygnet"},
			},
			Required: []string{"username", "gstin", "otp"},
		},
	},
	{
		ActionName:   DownloadDataRequest,
		Description:  "Request download of GST data from Cygnet",
		SystemPrompt: "Request download of GST data from Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin":    {Type: "string", Description: "GSTIN of the customer"},
				"to_mnth":  {Type: "integer", Description: "To month (e.g. 1, 2...); defaults to current month"},
				"to_yr":    {Type: "integer", Description: "To year (e.g. 2024); defaults to current year"},
				"frm_mnth": {Type: "integer", Description: "From month (e.g. 1, 2...); defaults to 4 (April)"},
				"frm_yr":   {Type: "integer", Description: "From year; defaults to current year minus 3 (or 4 if current month < April)"},
				"cgdt":     {Type: "integer", Description: "cgdt flag; defaults to 1"},
			},
			Required: []string{"gstin"},
		},
	},
	{
		ActionName:   CreateCustomer,
		Description:  "Create a new customer in Cygnet",
		SystemPrompt: "Create a new customer in Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"lgnm": {Type: "string", Description: "Legal name of the customer"},
				"pan":  {Type: "string", Description: "PAN of the customer"},
			},
			Required: []string{"lgnm", "pan"},
		},
	},
	{
		ActionName:   GSTINSearch,
		Description:  "Search taxpayer by PAN in Cygnet",
		SystemPrompt: "Search for a taxpayer by PAN in Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"pan": {Type: "string", Description: "PAN of the taxpayer to search"},
			},
			Required: []string{"pan"},
		},
	},
	{
		ActionName:   VerifySession,
		Description:  "Verify GST session status in Cygnet",
		SystemPrompt: "Verify GST session status in Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin": {Type: "string", Description: "GSTIN of the customer"},
			},
			Required: []string{"gstin"},
		},
	},
	{
		ActionName:   VerifyConsent,
		Description:  "Verify consent status in Cygnet",
		SystemPrompt: "Verify consent status in Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"gstin": {Type: "string", Description: "GSTIN of the customer"},
			},
			Required: []string{"gstin"},
		},
	},
	{
		ActionName:   VerifyDownloadRequestStatus,
		Description:  "Verify download request status in Cygnet",
		SystemPrompt: "Verify download request status in Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"ref_id": {Type: "string", Description: "Reference ID of the download request"},
			},
			Required: []string{"ref_id"},
		},
	},
	{
		ActionName:   BankStatementUpload,
		Description:  "Upload bank statement to Cygnet",
		SystemPrompt: "Upload bank statement to Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"cid":          {Type: "integer", Description: "Customer ID (default: 30)"},
				"bnk_typ":      {Type: "integer", Description: "Bank type (default: 3)"},
				"bnk_stmt":     {Type: "string", Description: "Bank statement file as base64"},
				"bnk_password": {Type: "string", Description: "Bank statement password (optional)"},
			},
			Required: []string{"bnk_stmt"},
		},
	},
	{
		ActionName:   BankStatementUploadStatus,
		Description:  "Check bank statement upload status in Cygnet",
		SystemPrompt: "Check bank statement upload status in Cygnet API",
		OutputSchema: models.JSONSchema{},
		Parameters: models.JSONSchema{
			Type: "object",
			Properties: map[string]models.JSONSchema{
				"process_id": {Type: "string", Description: "Process ID received from bank statement upload"},
			},
			Required: []string{"process_id"},
		},
	},
}

func (cygnetTool *CygnetTool) GetActionsList() []string {
	actions := []string{}
	for _, action := range cygnetToolActions {
		actions = append(actions, action.ActionName)
	}
	return actions
}

func (cygnetTool *CygnetTool) GetSpec() tools.Tooling {
	return cygnetTool
}

func (cygnetTool *CygnetTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &cygnetTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (cygnetTool *CygnetTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool Execute - Start")
	var toolRequest interface{}
	if actionName != Login {
		if cygnetTool.AuthToken == "" || cygnetTool.expiryTime.IsZero() || time.Now().After(cygnetTool.expiryTime) {
			logs.WithContext(ctx).Info("Token empty or expired, triggering auto-login")
			_, _, _, loginErr := cygnetTool.Login(ctx, projectId, tenantId, params)
			if loginErr != nil {
				return nil, false, fmt.Errorf("auto-login failed: %v", loginErr)
			}
		}
	}

	switch actionName {
	case Login:
		toolResult, toolRequest, persistStore, err = cygnetTool.Login(ctx, projectId, tenantId, params)
	case B2BSales:
		toolResult, toolRequest, persistStore, err = cygnetTool.gstDataAction(ctx, B2BSales, "/v0.1/customer/sales/b2binvoices/", params)
	case B2CSales:
		toolResult, toolRequest, persistStore, err = cygnetTool.gstDataAction(ctx, B2CSales, "/v0.1/customer/sales/b2csinvoices/", params)
	case CDNRSales:
		toolResult, toolRequest, persistStore, err = cygnetTool.gstDataAction(ctx, CDNRSales, "/v0.1/customer/sales/cdnotes/", params)
	case B2BPurchase:
		toolResult, toolRequest, persistStore, err = cygnetTool.gstDataAction(ctx, B2BPurchase, "/v0.1/customer/purchase/b2binvoices/", params)
	case CDNRPurchase:
		toolResult, toolRequest, persistStore, err = cygnetTool.gstDataAction(ctx, CDNRPurchase, "/v0.1/customer/purchase/cdnotes/", params)
	case HSNSales:
		toolResult, toolRequest, persistStore, err = cygnetTool.gstDataAction(ctx, HSNSales, "/v0.1/customer/sales/hsnsummary/", params)
	case NILSales:
		toolResult, toolRequest, persistStore, err = cygnetTool.gstDataAction(ctx, NILSales, "/v0.1/customer/sales/nilexemptnongst/", params)
	case GenerateOTP:
		toolResult, toolRequest, persistStore, err = cygnetTool.GenerateOTP(ctx, projectId, tenantId, params)
	case GenerateSession:
		toolResult, toolRequest, persistStore, err = cygnetTool.GenerateSession(ctx, projectId, tenantId, params)
	case DownloadDataRequest:
		toolResult, toolRequest, persistStore, err = cygnetTool.DownloadDataRequest(ctx, projectId, tenantId, params)
	case CreateCustomer:
		toolResult, toolRequest, persistStore, err = cygnetTool.CreateCustomer(ctx, projectId, tenantId, params)
	case GSTINSearch:
		toolResult, toolRequest, persistStore, err = cygnetTool.GSTINSearch(ctx, projectId, tenantId, params)
	case VerifySession:
		toolResult, toolRequest, persistStore, err = cygnetTool.VerifySession(ctx, projectId, tenantId, params)
	case VerifyConsent:
		toolResult, toolRequest, persistStore, err = cygnetTool.VerifyConsent(ctx, projectId, tenantId, params)
	case VerifyDownloadRequestStatus:
		toolResult, toolRequest, persistStore, err = cygnetTool.VerifyDownloadRequestStatus(ctx, projectId, tenantId, params)
	case BankStatementUpload:
		toolResult, toolRequest, persistStore, err = cygnetTool.BankStatementUpload(ctx, projectId, tenantId, params)
	case BankStatementUploadStatus:
		toolResult, toolRequest, persistStore, err = cygnetTool.BankStatementUploadStatus(ctx, projectId, tenantId, params)
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

		hookResult, err := cygnetTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (cygnetTool *CygnetTool) Login(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool Login - Start")

	// Prepare login data
	loginData := map[string]interface{}{
		"username": cygnetTool.User,
		"password": cygnetTool.Password,
	}

	// Encrypt request
	encryptedData, rawKey, encryptedAppKey, err := cygnetTool.encryptRequest(ctx, loginData)
	if err != nil {
		return nil, nil, false, err
	}

	// Prepare final payload
	payload := map[string]interface{}{
		"data":    encryptedData,
		"app_key": encryptedAppKey,
	}
	// Send request
	url := fmt.Sprintf("%s/v0.1/authenticate/token/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	// Decrypt response
	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, rawKey)
	if err != nil {
		return nil, nil, false, err
	}

	type CygnetLoginResponse struct {
		AuthToken string `json:"auth-token"`
		Expiry    int    `json:"expiry"`
	}

	decryptedResponseBytes, err := json.Marshal(decryptedResponse)
	if err != nil {
		return nil, nil, false, err
	}

	var cygnetLoginResponse CygnetLoginResponse
	err = json.Unmarshal(decryptedResponseBytes, &cygnetLoginResponse)
	if err != nil {
		toolResult = make(map[string]interface{})
		toolResult["login_result"] = decryptedResponse
		return toolResult, payload, false, err
	}
	cygnetTool.AuthToken = cygnetLoginResponse.AuthToken
	cygnetTool.Expiry = cygnetLoginResponse.Expiry
	cygnetTool.expiryTime = time.Now().Add(time.Duration(cygnetLoginResponse.Expiry-10) * time.Minute)
	cygnetTool.AesKey = rawKey

	toolResult = make(map[string]interface{})
	toolResult["login_result"] = cygnetLoginResponse
	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (cygnetTool *CygnetTool) GenerateOTP(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool GenerateOTP - Start")

	// 1. Validate and prepare data
	username, ok := params["username"].(string)
	if !ok || username == "" {
		return nil, nil, false, fmt.Errorf("username is required and must be a non-empty string")
	}
	gstin, ok := params["gstin"].(string)
	if !ok || gstin == "" {
		return nil, nil, false, fmt.Errorf("gstin is required and must be a non-empty string")
	}
	otpData := map[string]interface{}{
		"username": username,
		"gstin":    gstin,
	}

	// 2. Encrypt request (similar to Login)
	encryptedData, rawKey, _, err := cygnetTool.encryptRequest(ctx, otpData)
	if err != nil {
		return nil, nil, false, err
	}

	// 3. Prepare final payload
	payload := map[string]interface{}{
		"data": encryptedData,
	}

	// 4. Send request
	url := fmt.Sprintf("%s/v0.1/customer/generateotp/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	// 5. Decrypt response using the NEW rawKey generated for this request
	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, rawKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["generateotp_result"] = decryptedResponse
	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (cygnetTool *CygnetTool) GenerateSession(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool GenerateSession - Start")

	// 1. Validate and prepare data
	username, ok := params["username"].(string)
	if !ok || username == "" {
		return nil, nil, false, fmt.Errorf("username is required and must be a non-empty string")
	}
	gstin, ok := params["gstin"].(string)
	if !ok || gstin == "" {
		return nil, nil, false, fmt.Errorf("gstin is required and must be a non-empty string")
	}
	otp, ok := params["otp"].(string)
	if !ok || otp == "" {
		return nil, nil, false, fmt.Errorf("otp is required and must be a non-empty string")
	}
	sessionData := map[string]interface{}{
		"username": username,
		"gstin":    gstin,
		"otp":      otp,
	}

	// 2. Encrypt request
	encryptedData, rawKey, _, err := cygnetTool.encryptRequest(ctx, sessionData)
	if err != nil {
		return nil, nil, false, err
	}

	// 3. Prepare final payload
	payload := map[string]interface{}{
		"data": encryptedData,
	}

	// 4. Send request
	url := fmt.Sprintf("%s/v0.1/customer/generatesession/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	// 5. Decrypt response
	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, rawKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["generatesession_result"] = decryptedResponse

	// If session was established successfully, trigger a download data request
	if responseMap, ok := decryptedResponse.(map[string]interface{}); ok {
		if statusCd, ok := responseMap["status_cd"]; ok {
			if statusCd.(float64) == 1 {
				downloadResult, _, _, downloadErr := cygnetTool.DownloadDataRequest(ctx, projectId, tenantId, map[string]interface{}{
					"gstin": gstin,
				})
				if downloadErr != nil {
					logs.WithContext(ctx).Error(fmt.Sprintf("DownloadDataRequest failed after GenerateSession: %s", downloadErr.Error()))
				} else {
					for k, v := range downloadResult {
						toolResult[k] = v
					}
				}
			}
		}
	}
	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (cygnetTool *CygnetTool) DownloadDataRequest(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool DownloadDataRequest - Start")

	// 1. Validate required fields
	gstin, ok := params["gstin"].(string)
	if !ok || gstin == "" {
		return nil, nil, false, fmt.Errorf("gstin is required and must be a non-empty string")
	}

	// 2. Apply defaults using current date
	now := time.Now()
	currentMonth := int(now.Month())
	currentYear := now.Year()

	toMnth := currentMonth
	if v, ok := params["to_mnth"]; ok {
		if vInt, ok := toInt(v); ok {
			toMnth = vInt
		}
	}

	toYr := currentYear
	if v, ok := params["to_yr"]; ok {
		if vInt, ok := toInt(v); ok {
			toYr = vInt
		}
	}

	frmMnth := 4
	if v, ok := params["frm_mnth"]; ok {
		if vInt, ok := toInt(v); ok {
			frmMnth = vInt
		}
	}

	frmYr := currentYear - 3
	if currentMonth < 4 {
		frmYr = currentYear - 4
	}
	if v, ok := params["frm_yr"]; ok {
		if vInt, ok := toInt(v); ok {
			frmYr = vInt
		}
	}

	cgdt := 1
	if v, ok := params["cgdt"]; ok {
		if vInt, ok := toInt(v); ok {
			cgdt = vInt
		}
	}

	// 3. Build and encrypt payload
	downloadData := map[string]interface{}{
		"gstin":    gstin,
		"to_mnth":  toMnth,
		"to_yr":    toYr,
		"frm_mnth": frmMnth,
		"frm_yr":   frmYr,
		"cgdt":     cgdt,
	}

	encryptedData, rawKey, _, err := cygnetTool.encryptRequest(ctx, downloadData)
	if err != nil {
		return nil, nil, false, err
	}

	// 4. Prepare final payload
	payload := map[string]interface{}{
		"data": encryptedData,
	}

	// 5. Send request
	url := fmt.Sprintf("%s/v0.1/customer/downloaddata/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	// 6. Decrypt response
	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, rawKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["downloaddatarequest_result"] = decryptedResponse
	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (cygnetTool *CygnetTool) CreateCustomer(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool CreateCustomer - Start")

	// 1. Validate and prepare data
	lgnm, ok := params["lgnm"].(string)
	if !ok || lgnm == "" {
		return nil, nil, false, fmt.Errorf("lgnm is required and must be a non-empty string")
	}
	pan, ok := params["pan"].(string)
	if !ok || pan == "" {
		return nil, nil, false, fmt.Errorf("pan is required and must be a non-empty string")
	}
	customerData := map[string]interface{}{
		"lgnm": lgnm,
		"pan":  pan,
		"ct":   1,
	}

	// 2. Encrypt request
	encryptedData, rawKey, encryptedAppKey, err := cygnetTool.encryptRequest(ctx, customerData)
	if err != nil {
		return nil, nil, false, err
	}

	// 3. Prepare final payload
	payload := map[string]interface{}{
		"data":    encryptedData,
		"app_key": encryptedAppKey,
	}

	// 4. Send request
	url := fmt.Sprintf("%s/v0.1/customer/create/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	// 5. Decrypt response
	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, rawKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["createcustomer_result"] = decryptedResponse
	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (cygnetTool *CygnetTool) GSTINSearch(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool GSTINSearch - Start")

	// 1. Validate required fields
	gstin := ""
	pan, ok := params["pan"].(string)
	if !ok || pan == "" {
		gstin, ok = params["gstin"].(string)
		if !ok || gstin == "" {
			return nil, nil, false, fmt.Errorf("pan or gstin is required and must be a non-empty string")
		}
	}

	// 2. Send GET request with pan as query param (no payload encryption)
	url := fmt.Sprintf("%s/v0.1/taxpayer/search/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	queryParams := map[string]string{}
	if pan != "" {
		queryParams["pan"] = pan
	} else if gstin != "" {
		queryParams["gstin"] = gstin
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	// 3. Decrypt response using stored AesKey
	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, cygnetTool.AesKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["gstinsearch_result"] = decryptedResponse
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (cygnetTool *CygnetTool) VerifySession(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool VerifySession - Start")

	gstin, ok := params["gstin"].(string)
	if !ok || gstin == "" {
		return nil, nil, false, fmt.Errorf("gstin is required and must be a non-empty string")
	}

	url := fmt.Sprintf("%s/v0.1/customer/verifysession/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	queryParams := map[string]string{"gstin": gstin}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, cygnetTool.AesKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["verifysession_result"] = decryptedResponse
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (cygnetTool *CygnetTool) VerifyConsent(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool VerifyConsent - Start")

	gstin, ok := params["gstin"].(string)
	if !ok || gstin == "" {
		return nil, nil, false, fmt.Errorf("gstin is required and must be a non-empty string")
	}

	url := fmt.Sprintf("%s/v0.1/customer/consentstatus/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	queryParams := map[string]string{"gstin": gstin}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, cygnetTool.AesKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["verifyconsent_result"] = decryptedResponse
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (cygnetTool *CygnetTool) VerifyDownloadRequestStatus(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool VerifyDownloadRequestStatus - Start")

	refId, ok := params["ref_id"].(string)
	if !ok || refId == "" {
		return nil, nil, false, fmt.Errorf("ref_id is required and must be a non-empty string")
	}

	url := fmt.Sprintf("%s/v0.1/customer/downloadstatus/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	queryParams := map[string]string{"ref_id": refId}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, cygnetTool.AesKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["verifydownloadrequeststatus_result"] = decryptedResponse
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}
func (cygnetTool *CygnetTool) BankStatementUpload(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool BankStatementUpload - Start")

	// 1. Prepare data with defaults
	cid := 30
	if v, ok := toInt(params["cid"]); ok {
		cid = v
	}

	bnkTyp := 3
	if v, ok := toInt(params["bnk_typ"]); ok {
		bnkTyp = v
	}

	bnkStmt, ok := params["bnk_stmt"].(string)
	if !ok || bnkStmt == "" {
		return nil, nil, false, fmt.Errorf("bnk_stmt is required and must be a non-empty base64 string")
	}

	bankData := map[string]interface{}{
		"cid":      cid,
		"bnk_typ":  bnkTyp,
		"bnk_stmt": bnkStmt,
	}

	if bnkPwd, ok := params["bnk_password"].(string); ok && bnkPwd != "" {
		bankData["bnk_password"] = bnkPwd
	}

	// 2. Encrypt request
	encryptedData, rawKey, _, err := cygnetTool.encryptRequest(ctx, bankData)
	if err != nil {
		return nil, nil, false, err
	}

	// 3. Prepare final payload
	payload := map[string]interface{}{
		"data": encryptedData,
	}

	// 4. Send request
	url := fmt.Sprintf("%s/v0.1/customer/bankstatement/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	// 5. Decrypt response
	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, rawKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["bankstatementupload_result"] = decryptedResponse
	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

func (cygnetTool *CygnetTool) BankStatementUploadStatus(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CygnetTool BankStatementUploadStatus - Start")

	// 1. Prepare data
	processId, ok := params["process_id"].(string)
	if !ok || processId == "" {
		return nil, nil, false, fmt.Errorf("process_id is required and must be a non-empty string")
	}

	statusData := map[string]interface{}{
		"process_id": processId,
	}

	// 2. Encrypt request
	encryptedData, rawKey, _, err := cygnetTool.encryptRequest(ctx, statusData)
	if err != nil {
		return nil, nil, false, err
	}

	// 3. Prepare final payload
	payload := map[string]interface{}{
		"data": encryptedData,
	}

	// 4. Send request
	url := fmt.Sprintf("%s/v0.1/customer/bankstatementstatus/", cygnetTool.BaseUrl)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	// 5. Decrypt response
	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, rawKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["bankstatementuploadstatus_result"] = decryptedResponse
	return toolResult, map[string]interface{}{"body": payload}, false, nil
}

// toInt is a helper to coerce interface{} param values to int (handles float64 from JSON unmarshaling).
func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	}
	return 0, false
}

func (cygnetTool *CygnetTool) encryptRequest(ctx context.Context, data interface{}) (encryptedData string, rawKey []byte, encryptedAppKey string, err error) {

	// Generate 32-byte AES key
	aesKey := cygnetTool.AesKey
	if aesKey == nil {
		generatedKey, genErr := aes.GenerateKey(ctx, 32)
		if genErr != nil {
			logs.WithContext(ctx).Error(genErr.Error())
			return "", nil, "", genErr
		}
		aesKey = generatedKey.Key
	}

	// 1. Marshal and Base64 encode the data
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", nil, "", err
	}
	b64JsonData := base64.StdEncoding.EncodeToString(jsonData)

	// 2. AES ECB Encrypt the Base64 string
	encryptedBytes, err := aes.EncryptECB(ctx, []byte(b64JsonData), aesKey)
	if err != nil {
		return "", nil, "", err
	}
	encryptedData = base64.StdEncoding.EncodeToString(encryptedBytes)

	// 3. Base64 encode the AES key
	b64AesKey := base64.StdEncoding.EncodeToString(aesKey)

	// 4. RSA Encrypt the Base64-encoded AES key using Cygnet's public key (assume it's a certificate PEM)
	// If CygnetPublicKey is a base64 encoded PEM, we need to decode it first based on the template logic H := b64Decode rasPubKey
	pubKeyPem := cygnetTool.CygnetPublicKey
	if decodedPubKey, err := base64.StdEncoding.DecodeString(cygnetTool.CygnetPublicKey); err == nil {
		pubKeyPem = string(decodedPubKey)
	}

	rsaEncryptedBytes, err := rsa.EncryptWithCert(ctx, []byte(b64AesKey), pubKeyPem)
	if err != nil {
		// Fallback to EncryptWithKey if it's not a certificate
		rsaEncryptedBytes, err = rsa.EncryptWithKey(ctx, []byte(b64AesKey), pubKeyPem)
		if err != nil {
			return "", nil, "", err
		}
	}
	encryptedAppKey = base64.StdEncoding.EncodeToString(rsaEncryptedBytes)

	return encryptedData, aesKey, encryptedAppKey, nil
}

func (cygnetTool *CygnetTool) decryptResponse(ctx context.Context, res interface{}, aesKey []byte) (interface{}, error) {
	resMap, ok := res.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	dataStr, ok := resMap["data"].(string)
	if !ok {
		return res, nil // Return as is if data is not present or not string
	}

	// 1. Base64 decode the data
	encryptedBytes, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, err
	}

	// 2. AES ECB Decrypt
	decryptedBytes, err := aes.DecryptECB(ctx, encryptedBytes, aesKey)
	if err != nil {
		return nil, err
	}

	// 3. The result is a Base64 encoded string of the actual JSON
	actualB64Str := string(decryptedBytes)
	actualJsonBytes, err := base64.StdEncoding.DecodeString(actualB64Str)
	if err != nil {
		// Maybe it was direct JSON? Let's check if it's already JSON
		var result interface{}
		if json.Unmarshal(decryptedBytes, &result) == nil {
			return result, nil
		}
		return nil, err
	}

	var result interface{}
	err = json.Unmarshal(actualJsonBytes, &result)
	if err != nil {
		return string(actualJsonBytes), nil
	}

	return result, nil
}

func (cygnetTool *CygnetTool) gstDataAction(ctx context.Context, actionName string, endpoint string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug(fmt.Sprintf("CygnetTool %s - Start", actionName))

	// 1. Map params to CygnetGstDataPayload
	var payload CygnetGstDataPayload
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, nil, false, err
	}
	if err := json.Unmarshal(paramBytes, &payload); err != nil {
		return nil, nil, false, err
	}

	// 2. Validate payload
	if err := utils.ValidateStruct(ctx, payload, ""); err != nil {
		return nil, nil, false, err
	}

	// 3. Prepare query params
	queryParams := map[string]string{
		"gstin": payload.GSTIN,
		"month": fmt.Sprintf("%02d", payload.Month),
		"year":  fmt.Sprintf("%04d", payload.Year),
	}

	// 4. Prepare headers
	url := fmt.Sprintf("%s%s", cygnetTool.BaseUrl, endpoint)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Response-Type", "application/json")
	headers.Set("Accept-Encoding", "identity")
	headers.Set("clientid", cygnetTool.ClientId)
	headers.Set("client-secret", cygnetTool.ClientSecret)
	headers.Set("txn", Txn)
	headers.Set("ip-usr", IpUse)
	headers.Set("auth-token", cygnetTool.AuthToken)

	// 5. Send request (GET)
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	// 6. Decrypt response
	decryptedResponse, err := cygnetTool.decryptResponse(ctx, res, cygnetTool.AesKey)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult[fmt.Sprintf("%s_result", actionName)] = decryptedResponse
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (cygnetTool *CygnetTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &CygnetTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}

func (cygnetTool *CygnetTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(cygnetTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}
