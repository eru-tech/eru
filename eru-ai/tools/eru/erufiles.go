package eru

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
	"google.golang.org/api/idtoken"
)

const (
	ErufilesUpload            = "upload"
	ErufilesUploadB64         = "upload_b64"
	ErufilesDownload          = "download"
	ErufilesSubscribe         = "subscribe"
	ErufilesRenewSubscription = "renew_subscription"
	ErufilesStopSubscription  = "stop_subscription"
	ErufilesStopAutoRenew     = "stop_auto_renew"
	ErufilesCallback          = "callback"
	ErufilesLogin             = "login"
	ErufilesRenewToken        = "renew_token"
	ErufilesGetSsoUrl         = "get_sso_url"

	erufilesGdrive   = "GDRIVE"
	erufilesOneDrive = "ONEDRIVE"

	gdriveApiBase    = "https://www.googleapis.com/drive/v3"
	graphDriveBase   = "https://graph.microsoft.com/v1.0/me/drive"
	graphSubsBase    = "https://graph.microsoft.com/v1.0/subscriptions"
)

type ErufilesUploadParams struct {
	FolderPath  string `json:"folder_path" desc:"target folder path inside the storage"`
	FileName    string `json:"file_name" eru:"required" desc:"file name to save"`
	DocType     string `json:"doc_type" desc:"optional document type tag"`
	ContentType string `json:"content_type" desc:"MIME type for the upload"`
	Data        string `json:"data" eru:"required" desc:"raw or base64 encoded payload depending on action"`
}

type ErufilesDownloadParams struct {
	FolderPath string `json:"folder_path" desc:"folder path inside the storage"`
	FileName   string `json:"file_name" eru:"required" desc:"file name to download"`
}

type ErufilesSubscribeParams struct {
	TargetPath string `json:"target_path" desc:"optional folder/file path to scope the subscription"`
}

type ErufilesUnsubscribeParams struct {
}

type ErufilesTool struct {
	tools.Tool
	StorageName string `json:"storage_name" eru:"required"`
	StorageType string `json:"storage_type" eru:"required"`
	AuthName    string `json:"auth_name"`
	EventName   string `json:"event_name"`
	TargetPath  string `json:"target_path"`

	SubscriptionId                 string `json:"subscription_id"`
	SubscriptionExpirationDateTime string `json:"subscription_expiration_date_time"`
	StartPageToken                 string `json:"start_page_token,omitempty"`
	DeltaLink                      string `json:"delta_link,omitempty"`
	TopicName                      string `json:"topic_name,omitempty"`
	OidcAudience                   string `json:"oidc_audience,omitempty"`
	OidcServiceAccount             string `json:"oidc_service_account,omitempty"`
}

var erufilesToolActions = []tools.ToolAction{
	{
		ActionName:   ErufilesUpload,
		Description:  "Upload a file to the configured eru-files storage (binary content, multipart form via eru-files).",
		SystemPrompt: "Upload a binary file to the storage. data must be base64-encoded.",
		OutputSchema: eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufilesUploadParams{}), []string{})
		},
	},
	{
		ActionName:   ErufilesUploadB64,
		Description:  "Upload a base64-encoded file payload to the configured eru-files storage.",
		SystemPrompt: "Upload a base64-encoded payload as a file to the storage.",
		OutputSchema: eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufilesUploadParams{}), []string{})
		},
	},
	{
		ActionName:   ErufilesDownload,
		Description:  "Download a file from the configured eru-files storage. Returns base64-encoded content.",
		SystemPrompt: "Download a file from the storage by folder and name.",
		OutputSchema: eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufilesDownloadParams{}), []string{})
		},
	},
	{
		ActionName:   ErufilesSubscribe,
		Description:  "Subscribe to changes on a file or folder in the configured storage. Triggers callbacks via the tool's CLBK hook on change.",
		SystemPrompt: "Subscribe to a file or folder in the storage and receive change notifications.",
		OutputSchema: eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufilesSubscribeParams{}), []string{})
		},
	},
	{
		ActionName:   ErufilesRenewSubscription,
		Description:  "Renew an existing change subscription before it expires.",
		SystemPrompt: "Renew the active subscription on this storage.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
	{
		ActionName:   ErufilesStopSubscription,
		Description:  "Stop the active change subscription on this storage.",
		SystemPrompt: "Stop the active subscription on this storage.",
		OutputSchema: eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(ErufilesUnsubscribeParams{}), []string{})
		},
	},
	{
		ActionName:   ErufilesStopAutoRenew,
		Description:  "Stop the auto-renew job for this subscription.",
		SystemPrompt: "Stop the scheduled auto-renew of the subscription.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
	{
		ActionName:   ErufilesGetSsoUrl,
		Description:  "Get the IdP SSO URL via eru-auth for the configured auth_name (used by the consent/redirect flow).",
		SystemPrompt: "Return the SSO URL for user consent.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
	{
		ActionName:   ErufilesLogin,
		Description:  "Exchange an OAuth authorization code for tokens via eru-auth (idptoken). Tokens are persisted by eru-auth when persist_token=true.",
		SystemPrompt: "Convert an OAuth authorization code into tokens.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
	{
		ActionName:   ErufilesRenewToken,
		Description:  "Renew the IdP token explicitly via eru-auth (idptoken/renew). Normally not needed - gettoken auto-refreshes.",
		SystemPrompt: "Renew the IdP token using its refresh_token.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
	},
}

func (t *ErufilesTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(erufilesToolActions))
	for i, a := range erufilesToolActions {
		infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
	}
	return infos
}

func (t *ErufilesTool) GetActions() []tools.ToolAction {
	return erufilesToolActions
}

func (t *ErufilesTool) GetSpec() tools.Tooling {
	return t
}

func (t *ErufilesTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("ErufilesTool MakeFromJson - Start")
	if err := json.Unmarshal(*rj, t); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	t.StorageType = strings.ToUpper(t.StorageType)
	return nil
}

func (t *ErufilesTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &ErufilesTool{}
	if err := json.Unmarshal(toolObjJson, newTool); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return newTool, nil
}

func (t *ErufilesTool) GetBytes(ctx context.Context) ([]byte, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return b, nil
}

func (t *ErufilesTool) SetToolAction(actionName string) {
	for _, a := range erufilesToolActions {
		if a.ActionName == actionName {
			t.ToolAction = a
			return
		}
	}
	t.ToolAction = tools.ToolAction{}
}

func (t *ErufilesTool) GetAttribute(ctx context.Context, attributeName string) (interface{}, error) {
	switch attributeName {
	case "tool_name":
		return t.ToolName, nil
	case "tool_type":
		return t.ToolType, nil
	case "storage_name":
		return t.StorageName, nil
	case "storage_type":
		return t.StorageType, nil
	case "auth_name":
		return t.AuthName, nil
	case "event_name":
		return t.EventName, nil
	case "target_path":
		return t.TargetPath, nil
	case "system_prompt":
		return t.SystemPrompt, nil
	case "output_schema":
		return t.OutputSchema, nil
	case "parameters":
		return t.Parameters, nil
	case "description":
		return t.Description, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (t *ErufilesTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) error {
	switch attributeName {
	case "tool_name":
		t.ToolName, _ = attributeValue.(string)
	case "tool_type":
		t.ToolType, _ = attributeValue.(string)
	case "storage_name":
		t.StorageName, _ = attributeValue.(string)
	case "storage_type":
		s, _ := attributeValue.(string)
		t.StorageType = strings.ToUpper(s)
	case "auth_name":
		t.AuthName, _ = attributeValue.(string)
	case "event_name":
		t.EventName, _ = attributeValue.(string)
	case "target_path":
		t.TargetPath, _ = attributeValue.(string)
	case "system_prompt":
		t.SystemPrompt, _ = attributeValue.(string)
	case "output_schema":
		t.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		t.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		t.Description, _ = attributeValue.(string)
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (t *ErufilesTool) GetToolCbUrl(projectId string, tenantId string) string {
	return fmt.Sprint(t.CallbackBaseUrl, "/", projectId, "/callback/", tenantId, "/tool/", t.ToolName)
}

func (t *ErufilesTool) GetToolCallback() tools.ToolCallback {
	return tools.ToolCallback{
		ResponseContentType: "application/json",
	}
}

func erufilesBaseUrl(ctx context.Context) (string, error) {
	if v, ok := ctx.Value("erufilesbaseurl").(string); ok && v != "" {
		return strings.TrimRight(v, "/"), nil
	}
	if v := os.Getenv("ERUFILES_BASEURL"); v != "" {
		return strings.TrimRight(v, "/"), nil
	}
	return "", errors.New("erufilesbaseurl not found in context or env")
}

func (t *ErufilesTool) buildHeaders(ctx context.Context) http.Header {
	h := http.Header{}
	if claims := ctx.Value("claims"); claims != nil {
		h.Set("claims", fmt.Sprint(claims))
	}
	if t.ToolName != "" {
		h.Set("X-Token-Key-Prefix", t.ToolName)
	}
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "application/json")
	return h
}

func (t *ErufilesTool) getAccessToken(ctx context.Context, projectId string) (string, error) {
	base, err := erufilesBaseUrl(ctx)
	if err != nil {
		return "", err
	}
	tokenUrl := fmt.Sprintf("%s/files/%s/%s/gettoken", base, projectId, t.StorageName)
	res, _, _, status, err := utils.CallHttp(ctx, http.MethodGet, tokenUrl, t.buildHeaders(ctx), map[string]string{}, nil, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("erufiles gettoken failed (status %d): %s", status, err.Error()))
		return "", err
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		return "", errors.New("erufiles gettoken response not an object")
	}
	at, _ := m["access_token"].(string)
	if at == "" {
		return "", errors.New("erufiles gettoken response missing access_token")
	}
	return at, nil
}

func (t *ErufilesTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ErufilesTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case ErufilesUpload:
		toolResult, toolRequest, persistStore, err = t.Upload(ctx, projectId, params, false)
	case ErufilesUploadB64:
		toolResult, toolRequest, persistStore, err = t.Upload(ctx, projectId, params, true)
	case ErufilesDownload:
		toolResult, toolRequest, persistStore, err = t.Download(ctx, projectId, params)
	case ErufilesSubscribe:
		toolResult, toolRequest, persistStore, err = t.Subscribe(ctx, projectId, tenantId, params, false)
	case ErufilesRenewSubscription:
		toolResult, toolRequest, persistStore, err = t.Subscribe(ctx, projectId, tenantId, params, false)
	case ErufilesStopSubscription:
		toolResult, toolRequest, persistStore, err = t.Subscribe(ctx, projectId, tenantId, params, true)
	case ErufilesStopAutoRenew:
		toolResult, toolRequest, persistStore, err = t.StopAutoRenew(ctx, projectId, tenantId, params)
	case ErufilesGetSsoUrl:
		toolResult, toolRequest, persistStore, err = t.GetSsoUrl(ctx, projectId, tenantId, params)
	case ErufilesLogin:
		toolResult, toolRequest, persistStore, err = t.Login(ctx, projectId, tenantId, params, "")
	case ErufilesRenewToken:
		toolResult, toolRequest, persistStore, err = t.RenewToken(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}

	gm := server.GetGlobalGoroutineManager(ctx)
	gm.SafeGoWithRestartBehavior("erufilestool-post-execute-hook", func(bgCtx context.Context) {
		if claims := ctx.Value("claims"); claims != nil {
			bgCtx = context.WithValue(bgCtx, "claims", claims)
		}
		if efurl := ctx.Value(tools.EruFuncBaseUrlKey); efurl != nil {
			if s, ok := efurl.(string); ok {
				bgCtx = context.WithValue(bgCtx, tools.EruFuncBaseUrlKey, s)
			}
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
		if hookResult, hookErr := t.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil); hookErr != nil {
			logs.WithContext(bgCtx).Error(hookErr.Error())
		} else {
			logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
		}
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func unmarshalParams(ctx context.Context, params map[string]interface{}, target interface{}) error {
	b, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("error marshalling params: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return logs.Err(ctx, err, "")
	}
	return nil
}

func eruauthBaseUrl(ctx context.Context) (string, error) {
	if v, ok := ctx.Value("eruauthbaseurl").(string); ok && v != "" {
		return strings.TrimRight(v, "/"), nil
	}
	if v := os.Getenv("ERUAUTH_BASEURL"); v != "" {
		return strings.TrimRight(v, "/"), nil
	}
	return "", errors.New("eruauthbaseurl not found in context or env")
}

func (t *ErufilesTool) GetSsoUrl(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ErufilesTool GetSsoUrl - Start")
	if t.AuthName == "" {
		err = errors.New("auth_name is required on the tool config")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	base, err := eruauthBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	ssoUrl := fmt.Sprint(base, "/", projectId, "/", t.AuthName, "/getssourl")
	qParams := map[string]string{}
	if params["state"] != nil {
		qParams["state"], _ = params["state"].(string)
	}
	res, _, _, status, callErr := utils.CallHttp(ctx, http.MethodGet, ssoUrl, t.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, qParams, nil)
	if callErr != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("erufiles getssourl failed (status %d): %s", status, callErr.Error()))
		return nil, nil, false, callErr
	}
	rm, ok := res.(map[string]interface{})
	if !ok {
		err = errors.New("getssourl response is not an object")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	return rm, map[string]interface{}{"query": qParams}, false, nil
}

func (t *ErufilesTool) Login(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, renewStr string) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ErufilesTool Login - Start")
	if t.AuthName == "" {
		err = errors.New("auth_name is required on the tool config")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	base, err := eruauthBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	idpUrl := fmt.Sprint(base, "/", projectId, "/", t.AuthName, "/idptoken", renewStr)
	if params == nil {
		params = map[string]interface{}{}
	}
	if t.ToolName != "" {
		params["token_key_prefix"] = t.ToolName
	}
	res, _, _, status, callErr := utils.CallHttp(ctx, http.MethodPost, idpUrl, t.buildHeaders(ctx), map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if callErr != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("erufiles idptoken failed (status %d): %s", status, callErr.Error()))
		return nil, nil, false, callErr
	}
	rm, ok := res.(map[string]interface{})
	if !ok {
		err = errors.New("idptoken response is not an object")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	return rm, params, false, nil
}

func (t *ErufilesTool) RenewToken(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	return t.Login(ctx, projectId, tenantId, params, "/renew")
}

func (t *ErufilesTool) Upload(ctx context.Context, projectId string, params map[string]interface{}, b64Action bool) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	p := ErufilesUploadParams{}
	if err = unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.FileName == "" || p.Data == "" {
		err = errors.New("file_name and data are required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	base, err := erufilesBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}

	if b64Action {
		uploadUrl := fmt.Sprintf("%s/files/%s/%s/uploadb64", base, projectId, t.StorageName)
		body := map[string]interface{}{
			"folderpath": p.FolderPath,
			"file_name":  p.FileName,
			"doctype":    p.DocType,
			"file":       p.Data,
		}
		res, _, _, status, callErr := utils.CallHttp(ctx, http.MethodPost, uploadUrl, t.buildHeaders(ctx), map[string]string{}, nil, map[string]string{}, body)
		if callErr != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("erufiles uploadb64 failed (status %d): %s", status, callErr.Error()))
			return nil, nil, false, callErr
		}
		toolResult = map[string]interface{}{"result": res}
		return toolResult, body, false, nil
	}

	raw, decErr := base64.StdEncoding.DecodeString(p.Data)
	if decErr != nil {
		raw, decErr = base64.URLEncoding.DecodeString(p.Data)
		if decErr != nil {
			err = errors.New("data must be base64-encoded for upload action")
			logs.WithContext(ctx).Error(err.Error())
			return nil, nil, false, err
		}
	}
	uploadUrl := fmt.Sprintf("%s/files/%s/%s/upload", base, projectId, t.StorageName)
	var bodyBuf bytes.Buffer
	mw := multipart.NewWriter(&bodyBuf)
	_ = mw.WriteField("folderpath", p.FolderPath)
	_ = mw.WriteField("doctype", fmt.Sprintf("%s:%s", p.DocType, p.FileName))
	contentType := p.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	mh := make(map[string][]string)
	mh["Content-Disposition"] = []string{fmt.Sprintf("form-data; name=\"files\"; filename=%q", p.FileName)}
	mh["Content-Type"] = []string{contentType}
	pw, fwErr := mw.CreatePart(mh)
	if fwErr != nil {
		return nil, nil, false, fwErr
	}
	if _, err = pw.Write(raw); err != nil {
		return nil, nil, false, err
	}
	if err = mw.Close(); err != nil {
		return nil, nil, false, err
	}
	req, rErr := http.NewRequestWithContext(ctx, http.MethodPost, uploadUrl, &bodyBuf)
	if rErr != nil {
		return nil, nil, false, rErr
	}
	req.Header = t.buildHeaders(ctx)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, rErr := http.DefaultClient.Do(req)
	if rErr != nil {
		return nil, nil, false, rErr
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		err = fmt.Errorf("erufiles upload failed: status %d body %s", resp.StatusCode, string(respBytes))
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	var rm map[string]interface{}
	_ = json.Unmarshal(respBytes, &rm)
	toolResult = rm
	if toolResult == nil {
		toolResult = map[string]interface{}{"raw": string(respBytes)}
	}
	return toolResult, map[string]interface{}{"folder_path": p.FolderPath, "file_name": p.FileName, "doc_type": p.DocType}, false, nil
}

func (t *ErufilesTool) Download(ctx context.Context, projectId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	p := ErufilesDownloadParams{}
	if err = unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if p.FileName == "" {
		err = errors.New("file_name is required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	base, err := erufilesBaseUrl(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	dlUrl := fmt.Sprintf("%s/files/%s/%s/downloadb64/%s/%s", base, projectId, t.StorageName, url.PathEscape(p.FolderPath), url.PathEscape(p.FileName))
	res, _, _, status, callErr := utils.CallHttp(ctx, http.MethodGet, dlUrl, t.buildHeaders(ctx), map[string]string{}, nil, map[string]string{}, nil)
	if callErr != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("erufiles downloadb64 failed (status %d): %s", status, callErr.Error()))
		return nil, nil, false, callErr
	}
	rm, _ := res.(map[string]interface{})
	if rm == nil {
		rm = map[string]interface{}{"raw": res}
	}
	return rm, map[string]interface{}{"folder_path": p.FolderPath, "file_name": p.FileName}, false, nil
}

func (t *ErufilesTool) Subscribe(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, unsubscribe bool) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ErufilesTool Subscribe - Start")
	p := ErufilesSubscribeParams{}
	if perr := unmarshalParams(ctx, params, &p); perr == nil && p.TargetPath != "" {
		t.TargetPath = p.TargetPath
	}

	switch t.StorageType {
	case erufilesGdrive:
		toolResult, toolRequest, persistStore, err = t.subscribeGdrive(ctx, projectId, tenantId, unsubscribe)
	case erufilesOneDrive:
		toolResult, toolRequest, persistStore, err = t.subscribeOneDrive(ctx, projectId, tenantId, unsubscribe)
	default:
		err = fmt.Errorf("subscribe not supported for storage_type %q", t.StorageType)
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	if err != nil {
		return
	}

	if !unsubscribe && t.Hooks.ARSU != "" && t.Scheduler != nil {
		hookBody := map[string]interface{}{
			"Vars": map[string]interface{}{
				"Body":    map[string]interface{}{"tool_name": t.ToolName, "tenant_id": tenantId},
				"OrgBody": map[string]interface{}{"tool_name": t.ToolName, "tenant_id": tenantId},
			},
			"ReqVars": map[string]interface{}{},
			"ResVars": map[string]interface{}{},
		}
		hookBodyBytes, mErr := json.Marshal(hookBody)
		if mErr != nil {
			logs.WithContext(ctx).Error(mErr.Error())
			return toolResult, toolRequest, persistStore, mErr
		}
		jobName := fmt.Sprint(t.ToolName, "_", t.Hooks.ARSU, "_", tenantId)
		_ = t.Scheduler.Unschedule(ctx, "", jobName)
		schedulerCommand := fmt.Sprint("CALL schedule_procedure('", t.Hooks.ARSU, "','", string(hookBodyBytes), "','", t.Scheduler.GetSchedulerName(), "')")
		renewAfter := 24 * time.Hour
		if t.StorageType == erufilesOneDrive {
			renewAfter = 60 * time.Hour
		}
		cronStr := utils.GetCronStr(ctx, time.Now().UTC().Add(renewAfter))
		jobId, sErr := t.Scheduler.Schedule(ctx, jobName, schedulerCommand, cronStr)
		if sErr != nil {
			return toolResult, toolRequest, persistStore, sErr
		}
		logs.WithContext(ctx).Info(fmt.Sprint("erufiles subscribe jobId: ", jobId))
	}
	return
}

func gdriveGcpSubscriptionId(projectId string, tenantId string, toolName string) string {
	id := fmt.Sprintf("eru-%s-%s-%s-sub", projectId, tenantId, toolName)
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.', r == '~', r == '+', r == '%':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := b.String()
	if len(out) > 255 {
		out = out[:255]
	}
	return out
}

func gdriveCallEventSubscribe(ctx context.Context, projectId string, eventName string, subscriptionId string, pushEndpoint string) (map[string]interface{}, error) {
	efurl, _ := ctx.Value(tools.EruFuncBaseUrlKey).(string)
	if efurl == "" {
		return nil, errors.New("erufuncbaseurl not found in context")
	}
	subUrl := fmt.Sprint(efurl, "/store/", projectId, "/event/subscribe/", eventName)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	body := map[string]interface{}{
		"subscription_id": subscriptionId,
		"push_endpoint":   pushEndpoint,
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, subUrl, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		return nil, err
	}
	rm, ok := res.(map[string]interface{})
	if !ok {
		return nil, errors.New("subscribe response is not a map")
	}
	return rm, nil
}

func gdriveCallEventUnsubscribe(ctx context.Context, projectId string, eventName string, subscriptionId string) error {
	efurl, _ := ctx.Value(tools.EruFuncBaseUrlKey).(string)
	if efurl == "" {
		return errors.New("erufuncbaseurl not found in context")
	}
	subUrl := fmt.Sprint(efurl, "/store/", projectId, "/event/unsubscribe/", eventName, "/", subscriptionId)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	_, _, _, _, err := utils.CallHttp(ctx, http.MethodDelete, subUrl, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	return err
}

func (t *ErufilesTool) subscribeGdrive(ctx context.Context, projectId string, tenantId string, unsubscribe bool) (map[string]interface{}, interface{}, bool, error) {
	if t.EventName == "" {
		err := errors.New("event_name is required for GDRIVE subscribe (must point to a GCP_PUBSUB event)")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	subscriptionId := gdriveGcpSubscriptionId(projectId, tenantId, t.ToolName)

	if unsubscribe {
		if err := gdriveCallEventUnsubscribe(ctx, projectId, t.EventName, subscriptionId); err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		if t.SubscriptionId != "" {
			tok, terr := t.getAccessToken(ctx, projectId)
			if terr == nil {
				stopUrl := fmt.Sprint(gdriveApiBase, "/channels/stop")
				h := http.Header{}
				h.Set("Authorization", "Bearer "+tok)
				h.Set("Content-Type", "application/json")
				body := map[string]interface{}{"id": t.SubscriptionId, "resourceId": t.TopicName}
				_, _, _, _, _ = utils.CallHttp(ctx, http.MethodPost, stopUrl, h, map[string]string{}, nil, map[string]string{}, body)
			}
		}
		jobName := fmt.Sprint(t.ToolName, "_", t.Hooks.ARSU, "_", tenantId)
		if t.Scheduler != nil {
			_ = t.Scheduler.Unschedule(ctx, "", jobName)
		}
		t.SubscriptionId = ""
		t.SubscriptionExpirationDateTime = ""
		t.StartPageToken = ""
		t.TopicName = ""
		t.OidcAudience = ""
		t.OidcServiceAccount = ""
		return map[string]interface{}{"unsubscription_status": "success"}, nil, true, nil
	}

	pushEndpoint := t.GetToolCbUrl(projectId, tenantId)
	subRes, err := gdriveCallEventSubscribe(ctx, projectId, t.EventName, subscriptionId, pushEndpoint)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	topicName, _ := subRes["topic_name"].(string)
	if topicName == "" {
		err = errors.New("topic_name not returned from event subscribe")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	t.TopicName = topicName
	if v, ok := subRes["oidc_audience"].(string); ok {
		t.OidcAudience = v
	}
	if v, ok := subRes["oidc_service_account"].(string); ok {
		t.OidcServiceAccount = v
	}

	tok, err := t.getAccessToken(ctx, projectId)
	if err != nil {
		return nil, nil, false, err
	}

	startTok, sptErr := gdriveStartPageToken(ctx, tok)
	if sptErr != nil {
		logs.WithContext(ctx).Error(sptErr.Error())
		return nil, nil, false, sptErr
	}
	t.StartPageToken = startTok

	channelId := subscriptionId
	expirationMs := time.Now().UTC().Add(7 * 24 * time.Hour).UnixMilli()
	watchBody := map[string]interface{}{
		"id":         channelId,
		"type":       "web_hook",
		"address":    pushEndpoint,
		"expiration": fmt.Sprint(expirationMs),
	}
	watchUrl := fmt.Sprintf("%s/changes/watch?pageToken=%s&supportsAllDrives=true", gdriveApiBase, url.QueryEscape(startTok))
	wh := http.Header{}
	wh.Set("Authorization", "Bearer "+tok)
	wh.Set("Content-Type", "application/json")
	wRes, _, _, status, err := utils.CallHttp(ctx, http.MethodPost, watchUrl, wh, map[string]string{}, nil, map[string]string{}, watchBody)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("drive changes.watch failed (status %d): %s", status, err.Error()))
		return nil, nil, false, err
	}
	wm, _ := wRes.(map[string]interface{})
	t.SubscriptionId = channelId
	if exp, ok := wm["expiration"].(string); ok {
		if expMs, perr := strconv.ParseInt(exp, 10, 64); perr == nil {
			t.SubscriptionExpirationDateTime = time.Unix(0, expMs*int64(time.Millisecond)).UTC().Format(time.RFC3339)
		}
	}
	return map[string]interface{}{
		"subscription_status":              "success",
		"subscription_id":                  t.SubscriptionId,
		"subscription_expiration_datetime": t.SubscriptionExpirationDateTime,
		"start_page_token":                 t.StartPageToken,
	}, watchBody, true, nil
}

func gdriveStartPageToken(ctx context.Context, accessToken string) (string, error) {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+accessToken)
	h.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, gdriveApiBase+"/changes/startPageToken", h, map[string]string{}, nil, map[string]string{"supportsAllDrives": "true"}, nil)
	if err != nil {
		return "", err
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		return "", errors.New("startPageToken response not an object")
	}
	tok, _ := m["startPageToken"].(string)
	if tok == "" {
		return "", errors.New("startPageToken response missing token")
	}
	return tok, nil
}

func (t *ErufilesTool) subscribeOneDrive(ctx context.Context, projectId string, tenantId string, unsubscribe bool) (map[string]interface{}, interface{}, bool, error) {
	tok, err := t.getAccessToken(ctx, projectId)
	if err != nil {
		return nil, nil, false, err
	}
	if unsubscribe {
		if t.SubscriptionId != "" {
			delUrl := fmt.Sprintf("%s/%s", graphSubsBase, t.SubscriptionId)
			h := http.Header{}
			h.Set("Authorization", "Bearer "+tok)
			_, _, _, _, _ = utils.CallHttp(ctx, http.MethodDelete, delUrl, h, map[string]string{}, nil, map[string]string{}, nil)
		}
		jobName := fmt.Sprint(t.ToolName, "_", t.Hooks.ARSU, "_", tenantId)
		if t.Scheduler != nil {
			_ = t.Scheduler.Unschedule(ctx, "", jobName)
		}
		t.SubscriptionId = ""
		t.SubscriptionExpirationDateTime = ""
		t.DeltaLink = ""
		return map[string]interface{}{"unsubscription_status": "success"}, nil, true, nil
	}

	resource := "/me/drive/root"
	if t.TargetPath != "" {
		resource = fmt.Sprintf("/me/drive/root:/%s", strings.TrimLeft(t.TargetPath, "/"))
	}
	expiry := time.Now().UTC().Add(60 * time.Hour).Format(time.RFC3339)
	subBody := map[string]interface{}{
		"changeType":         "updated",
		"notificationUrl":    t.GetToolCbUrl(projectId, tenantId),
		"resource":           resource,
		"expirationDateTime": expiry,
		"clientState":        fmt.Sprint(projectId, "/", tenantId, "/", t.ToolName),
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+tok)
	h.Set("Content-Type", "application/json")
	res, _, _, status, err := utils.CallHttp(ctx, http.MethodPost, graphSubsBase, h, map[string]string{}, nil, map[string]string{}, subBody)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("graph subscriptions create failed (status %d): %s", status, err.Error()))
		return nil, nil, false, err
	}
	rm, _ := res.(map[string]interface{})
	if id, ok := rm["id"].(string); ok {
		t.SubscriptionId = id
	}
	if exp, ok := rm["expirationDateTime"].(string); ok {
		t.SubscriptionExpirationDateTime = exp
	}
	return map[string]interface{}{
		"subscription_status":              "success",
		"subscription_id":                  t.SubscriptionId,
		"subscription_expiration_datetime": t.SubscriptionExpirationDateTime,
	}, subBody, true, nil
}

func (t *ErufilesTool) StopAutoRenew(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	if t.Scheduler == nil {
		err = errors.New("scheduler not defined")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	jobName := fmt.Sprint(t.ToolName, "_", t.Hooks.ARSU, "_", tenantId)
	_ = t.Scheduler.Unschedule(ctx, "", jobName)
	return map[string]interface{}{"stop_auto_renew_status": "success"}, map[string]interface{}{"body": params}, true, nil
}

type gdrivePubSubPush struct {
	Message struct {
		Data        string            `json:"data"`
		MessageId   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
		Attributes  map[string]string `json:"attributes,omitempty"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

var (
	gdriveIdTokenValidator     *idtoken.Validator
	gdriveIdTokenValidatorOnce reflect.Value // unused; placeholder kept for parity if needed
)

func (t *ErufilesTool) verifyGdriveOidcPush(ctx context.Context) error {
	_ = gdriveIdTokenValidatorOnce
	if strings.EqualFold(os.Getenv("ERUAI_SKIP_OIDC_VERIFY"), "true") {
		return nil
	}
	if t.OidcServiceAccount == "" {
		return nil
	}
	authVal, _ := ctx.Value(tools.RequestAuthorizationKey).(string)
	if authVal == "" {
		return errors.New("missing Authorization header on push")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authVal, "Bearer "))
	if token == "" {
		return errors.New("empty bearer token on push")
	}
	if gdriveIdTokenValidator == nil {
		v, vErr := idtoken.NewValidator(ctx)
		if vErr != nil {
			return fmt.Errorf("idtoken validator init failed: %w", vErr)
		}
		gdriveIdTokenValidator = v
	}
	payload, vErr := gdriveIdTokenValidator.Validate(ctx, token, t.OidcAudience)
	if vErr != nil {
		return fmt.Errorf("oidc token validation failed: %w", vErr)
	}
	email, _ := payload.Claims["email"].(string)
	if email != t.OidcServiceAccount {
		return fmt.Errorf("oidc email mismatch: got %q want %q", email, t.OidcServiceAccount)
	}
	return nil
}

func (t *ErufilesTool) Callback(ctx context.Context, projectId string, tenantId string, actionName string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ErufilesTool Callback - Start")
	switch t.StorageType {
	case erufilesGdrive:
		return t.callbackGdrive(ctx, projectId, tenantId, body, params)
	case erufilesOneDrive:
		return t.callbackOneDrive(ctx, projectId, tenantId, body, params)
	default:
		err = fmt.Errorf("callback not supported for storage_type %q", t.StorageType)
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
}

func (t *ErufilesTool) callbackGdrive(ctx context.Context, projectId string, tenantId string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, persistStore bool, err error) {
	if verr := t.verifyGdriveOidcPush(ctx); verr != nil {
		logs.WithContext(ctx).Error(fmt.Sprint("oidc verify failed: ", verr.Error()))
		return "", false, nil
	}

	gm := server.GetGlobalGoroutineManager(ctx)
	gm.SafeGoWithRestartBehavior("erufiles-gdrive-callback", func(bgCtx context.Context) {
		if rid := ctx.Value("request_id"); rid != nil {
			bgCtx = context.WithValue(bgCtx, "request_id", rid)
		}
		if efurl := ctx.Value(tools.EruFuncBaseUrlKey); efurl != nil {
			if s, ok := efurl.(string); ok {
				bgCtx = context.WithValue(bgCtx, tools.EruFuncBaseUrlKey, s)
			}
		}
		bodyBytes, mErr := json.Marshal(body)
		if mErr != nil {
			logs.WithContext(bgCtx).Error(mErr.Error())
			return
		}
		var push gdrivePubSubPush
		if uErr := json.Unmarshal(bodyBytes, &push); uErr != nil {
			logs.WithContext(bgCtx).Error(uErr.Error())
			return
		}
		tok, tErr := t.getAccessToken(bgCtx, projectId)
		if tErr != nil {
			logs.WithContext(bgCtx).Error(tErr.Error())
			return
		}
		pageToken := t.StartPageToken
		if pageToken == "" {
			logs.WithContext(bgCtx).Info("erufiles gdrive callback: no startPageToken stored, skipping change fetch")
			return
		}
		listUrl := gdriveApiBase + "/changes"
		h := http.Header{}
		h.Set("Authorization", "Bearer "+tok)
		h.Set("Content-Type", "application/json")
		listParams := map[string]string{
			"pageToken":                 pageToken,
			"includeRemoved":            "true",
			"includeItemsFromAllDrives": "true",
			"supportsAllDrives":         "true",
			"fields":                    "newStartPageToken,nextPageToken,changes(fileId,removed,file(id,name,parents,mimeType,modifiedTime))",
		}
		res, _, _, _, lErr := utils.CallHttp(bgCtx, http.MethodGet, listUrl, h, map[string]string{}, nil, listParams, nil)
		if lErr != nil {
			logs.WithContext(bgCtx).Error(lErr.Error())
			return
		}
		rm, _ := res.(map[string]interface{})
		changes, _ := rm["changes"].([]interface{})
		for _, c := range changes {
			cm, _ := c.(map[string]interface{})
			hookBody := map[string]interface{}{
				"change":       cm,
				"storage_name": t.StorageName,
				"tenant_id":    tenantId,
			}
			if hookRes, hErr := t.ExecuteHook(bgCtx, "clbk", "", projectId, tenantId, hookBody, params); hErr != nil {
				logs.WithContext(bgCtx).Error(hErr.Error())
			} else {
				logs.WithContext(bgCtx).Info(fmt.Sprint(hookRes))
			}
		}
		if newTok, ok := rm["newStartPageToken"].(string); ok && newTok != "" {
			t.StartPageToken = newTok
		} else if next, ok := rm["nextPageToken"].(string); ok && next != "" {
			t.StartPageToken = next
		}
	}, server.ContinueOnMaxRetries)
	return "", false, nil
}

func (t *ErufilesTool) callbackOneDrive(ctx context.Context, projectId string, tenantId string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, persistStore bool, err error) {
	if vt, ok := params["validationToken"]; ok && len(vt) > 0 {
		return vt[0], false, nil
	}

	gm := server.GetGlobalGoroutineManager(ctx)
	gm.SafeGoWithRestartBehavior("erufiles-onedrive-callback", func(bgCtx context.Context) {
		if rid := ctx.Value("request_id"); rid != nil {
			bgCtx = context.WithValue(bgCtx, "request_id", rid)
		}
		if efurl := ctx.Value(tools.EruFuncBaseUrlKey); efurl != nil {
			if s, ok := efurl.(string); ok {
				bgCtx = context.WithValue(bgCtx, tools.EruFuncBaseUrlKey, s)
			}
		}
		tok, tErr := t.getAccessToken(bgCtx, projectId)
		if tErr != nil {
			logs.WithContext(bgCtx).Error(tErr.Error())
			return
		}
		deltaUrl := t.DeltaLink
		if deltaUrl == "" {
			deltaUrl = graphDriveBase + "/root/delta"
			if t.TargetPath != "" {
				deltaUrl = fmt.Sprintf("%s/root:/%s:/delta", graphDriveBase, strings.TrimLeft(t.TargetPath, "/"))
			}
		}
		h := http.Header{}
		h.Set("Authorization", "Bearer "+tok)
		h.Set("Content-Type", "application/json")
		for deltaUrl != "" {
			res, _, _, _, lErr := utils.CallHttp(bgCtx, http.MethodGet, deltaUrl, h, map[string]string{}, nil, map[string]string{}, nil)
			if lErr != nil {
				logs.WithContext(bgCtx).Error(lErr.Error())
				return
			}
			rm, _ := res.(map[string]interface{})
			items, _ := rm["value"].([]interface{})
			for _, it := range items {
				im, _ := it.(map[string]interface{})
				hookBody := map[string]interface{}{
					"item":         im,
					"storage_name": t.StorageName,
					"tenant_id":    tenantId,
				}
				if hookRes, hErr := t.ExecuteHook(bgCtx, "clbk", "", projectId, tenantId, hookBody, params); hErr != nil {
					logs.WithContext(bgCtx).Error(hErr.Error())
				} else {
					logs.WithContext(bgCtx).Info(fmt.Sprint(hookRes))
				}
			}
			if next, ok := rm["@odata.nextLink"].(string); ok && next != "" {
				deltaUrl = next
				continue
			}
			if dl, ok := rm["@odata.deltaLink"].(string); ok && dl != "" {
				t.DeltaLink = dl
			}
			deltaUrl = ""
		}
	}, server.ContinueOnMaxRetries)
	return "", false, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		ToolType:    "ERUFILES",
		Category:    "Storage",
		Description: "Eru-files storage tool for upload, download and change subscription on cloud-backed storages (GDRIVE / ONEDRIVE).",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(erufilesToolActions))
			for i, a := range erufilesToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: true,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(ErufilesTool{}), []string{}),
	})
}
