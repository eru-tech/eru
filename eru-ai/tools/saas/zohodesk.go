package saas

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

type ZohoTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type ZohoAccount struct {
	OrgId                   string `json:"org_id"`
	AccessToken             string `json:"-"`
	RefreshToken            string `json:"-"`
	TokenExpirationDateTime string `json:"token_expiration_date_time"`
}

type zohoAccountWithToken struct {
	OrgId                   string `json:"org_id"`
	AccessToken             string `json:"access_token"`
	RefreshToken            string `json:"refresh_token"`
	TokenExpirationDateTime string `json:"token_expiration_date_time"`
}

const (
	GetTickets          = "get_tickets"
	GetOrganizations    = "get_organizations"
	GetTicketThread     = "get_ticket_thread"
	GetTicketContent    = "get_ticket_content"
	GetTicketAttachment = "get_ticket_attachment"
	Login               = "login"
	RenewToken          = "renew_token"
	GetSsoUrl           = "get_sso_url"
	StopAutoRenew       = "stop_auto_renew"
)

const (
	DeskBaseUrl = "https://desk.zoho.in/api/v1"
)

type ZohoDeskTool struct {
	tools.Tool
	ZohoAccount ZohoAccount `json:"zoho_account"`
	AuthName    string      `json:"auth_name"`
}

type zohoDeskToolWithToken struct {
	tools.Tool
	ZohoAccount zohoAccountWithToken
	AuthName    string
}

func (zohoDeskTool *ZohoDeskTool) GetActionsList() []tools.ActionInfo {
	return []tools.ActionInfo{
		{Name: GetTickets},
		{Name: GetOrganizations},
		{Name: GetTicketThread},
		{Name: GetTicketContent},
		{Name: GetTicketAttachment},
		{Name: Login},
		{Name: RenewToken},
		{Name: GetSsoUrl},
		{Name: StopAutoRenew},
	}
}

func (zohoDeskTool *ZohoDeskTool) GetSpec() tools.Tooling {
	return zohoDeskTool
}

func (zohoDeskTool *ZohoDeskTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &zohoDeskTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (zohoDeskTool *ZohoDeskTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ZohoDeskTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case GetTickets:
		toolResult, toolRequest, persistStore, err = zohoDeskTool.GetTickets(ctx, params)
	case GetOrganizations:
		toolResult, toolRequest, persistStore, err = zohoDeskTool.GetOrganizations(ctx, params)
	case GetTicketThread:
		toolResult, toolRequest, persistStore, err = zohoDeskTool.GetTicketThread(ctx, params)
	case GetTicketContent:
		toolResult, toolRequest, persistStore, err = zohoDeskTool.GetTicketContent(ctx, params)
	case GetTicketAttachment:
		toolResult, toolRequest, persistStore, err = zohoDeskTool.GetTicketAttachment(ctx, params)
	case Login:
		toolResult, toolRequest, persistStore, err = zohoDeskTool.Login(ctx, projectId, tenantId, params, "")
	case RenewToken:
		toolResult, toolRequest, persistStore, err = zohoDeskTool.RenewToken(ctx, projectId, tenantId, params)
	case GetSsoUrl:
		toolResult, toolRequest, persistStore, err = zohoDeskTool.GetSsoUrl(ctx, projectId, tenantId, params)
	case StopAutoRenew:
		toolResult, toolRequest, persistStore, err = zohoDeskTool.StopAutoRenew(ctx, projectId, tenantId, params)
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

		hookResult, err := zohoDeskTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (zohoDeskTool *ZohoDeskTool) GetTickets(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetTickets Execute - Start")

	url := fmt.Sprint(DeskBaseUrl, "/tickets")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprint("Zoho-oauthtoken ", zohoDeskTool.ZohoAccount.AccessToken))
	headers.Set("orgId", zohoDeskTool.ZohoAccount.OrgId)
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		queryParams[k] = fmt.Sprint(v)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["data"] = res
	}

	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (zohoDeskTool *ZohoDeskTool) GetOrganizations(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetOrganizations Execute - Start")

	url := fmt.Sprint(DeskBaseUrl, "/organizations")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprint("Zoho-oauthtoken ", zohoDeskTool.ZohoAccount.AccessToken))
	headers.Set("orgId", zohoDeskTool.ZohoAccount.OrgId)
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		queryParams[k] = fmt.Sprint(v)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["data"] = res
	}

	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (zohoDeskTool *ZohoDeskTool) GetTicketThread(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetTicketThread Execute - Start")

	ticketId, ok := params["ticket_id"].(string)
	if !ok {
		err = errors.New("ticket_id is required")
		logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	url := fmt.Sprint(DeskBaseUrl, "/tickets/", ticketId, "/threads")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprint("Zoho-oauthtoken ", zohoDeskTool.ZohoAccount.AccessToken))
	headers.Set("orgId", zohoDeskTool.ZohoAccount.OrgId)
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		if k != "ticket_id" {
			queryParams[k] = fmt.Sprint(v)
		}
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	var threads []interface{}
	if resMap, ok := res.(map[string]interface{}); ok {
		if data, exists := resMap["data"]; exists {
			if threadsArray, ok := data.([]interface{}); ok {
				threads = threadsArray
			} else {
				threads = []interface{}{data}
			}
		} else {
			toolResult = resMap
			return toolResult, map[string]interface{}{"query": queryParams}, false, nil
		}
	} else if threadsArray, ok := res.([]interface{}); ok {
		threads = threadsArray
	} else {
		toolResult = make(map[string]interface{})
		toolResult["data"] = res
		return toolResult, map[string]interface{}{"query": queryParams}, false, nil
	}

	enrichedThreads := make([]interface{}, 0, len(threads))
	for _, thread := range threads {
		threadMap, ok := thread.(map[string]interface{})
		if !ok {
			enrichedThreads = append(enrichedThreads, thread)
			continue
		}

		threadId, exists := threadMap["id"].(string)
		if !exists || threadId == "" {
			enrichedThreads = append(enrichedThreads, thread)
			continue
		}

		contentParams := make(map[string]interface{})
		contentParams["ticket_id"] = ticketId
		contentParams["thread_id"] = threadId
		for k, v := range params {
			if k != "ticket_id" {
				contentParams[k] = v
			}
		}

		contentResult, _, _, err := zohoDeskTool.GetTicketContent(ctx, contentParams)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprint("Error getting content for thread ", threadId, ": ", err.Error()))
			enrichedThreads = append(enrichedThreads, thread)
			continue
		}

		if contentData, ok := contentResult["data"].(map[string]interface{}); ok {
			if attachments, exists := contentData["attachments"]; exists {
				threadMap["attachments"] = attachments
			}
			if content, exists := contentData["content"]; exists {
				threadMap["content"] = content
			}
		} else {
			if attachments, exists := contentResult["attachments"]; exists {
				threadMap["attachments"] = attachments
			}
			if content, exists := contentResult["content"]; exists {
				threadMap["content"] = content
			}
		}

		enrichedThreads = append(enrichedThreads, threadMap)
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
		toolResult["data"] = enrichedThreads
	} else {
		toolResult["data"] = enrichedThreads
	}

	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (zohoDeskTool *ZohoDeskTool) GetTicketContent(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetTicketContent Execute - Start")

	ticketId, ok := params["ticket_id"].(string)
	if !ok {
		err = errors.New("ticket_id is required")
		logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	threadId, ok := params["thread_id"].(string)
	if !ok {
		err = errors.New("thread_id is required")
		logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	url := fmt.Sprint(DeskBaseUrl, "/tickets/", ticketId, "/threads/", threadId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprint("Zoho-oauthtoken ", zohoDeskTool.ZohoAccount.AccessToken))
	headers.Set("orgId", zohoDeskTool.ZohoAccount.OrgId)
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		if k != "ticket_id" && k != "thread_id" {
			queryParams[k] = fmt.Sprint(v)
		}
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["data"] = res
	}

	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (zohoDeskTool *ZohoDeskTool) GetTicketAttachment(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetTicketAttachment Execute - Start")

	ticketId, ok := params["ticket_id"].(string)
	if !ok {
		err = errors.New("ticket_id is required")
		logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	threadId, ok := params["thread_id"].(string)
	if !ok {
		err = errors.New("thread_id is required")
		logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	attachmentId, ok := params["attachment_id"].(string)
	if !ok {
		err = errors.New("attachment_id is required")
		logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	url := fmt.Sprint(DeskBaseUrl, "/tickets/", ticketId, "/threads/", threadId, "/attachments/", attachmentId, "/content")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprint("Zoho-oauthtoken ", zohoDeskTool.ZohoAccount.AccessToken))
	headers.Set("orgId", zohoDeskTool.ZohoAccount.OrgId)
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		if k != "ticket_id" && k != "thread_id" && k != "attachment_id" {
			queryParams[k] = fmt.Sprint(v)
		}
	}

	res, respHeaders, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	var fileContent []byte
	if resMap, ok := res.(map[string]interface{}); ok {
		if body, exists := resMap["body"].(string); exists {
			fileContent = []byte(body)
		} else {
			err = errors.New("response body not found")
			logs.Err(ctx, err, "")
			return nil, nil, false, err
		}
	} else {
		err = errors.New("unexpected response format")
		logs.Err(ctx, err, "")
		return nil, nil, false, err
	}

	mimeType := respHeaders.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	base64Content := base64.StdEncoding.EncodeToString(fileContent)

	toolResult = make(map[string]interface{})
	toolResult["file"] = base64Content
	toolResult["mime_type"] = mimeType

	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (zohoDeskTool *ZohoDeskTool) GetSsoUrl(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetSsoUrl Execute - Start")

	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", zohoDeskTool.AuthName, "/getssourl")
	logs.WithContext(ctx).Info(fmt.Sprint("url: ", url))
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	qParams := make(map[string]string)
	if params["state"] != nil {
		qParams["state"] = params["state"].(string)
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, qParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResultOk := false
	toolResult, toolResultOk = res.(map[string]interface{})
	if !toolResultOk {
		err = errors.New("toolResult is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	logs.WithContext(ctx).Info(fmt.Sprint("toolResult: ", toolResult))
	return toolResult, map[string]interface{}{"query": qParams}, false, nil
}

func (zohoDeskTool *ZohoDeskTool) RenewToken(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	params["refresh_token"] = zohoDeskTool.ZohoAccount.RefreshToken
	return zohoDeskTool.Login(ctx, projectId, tenantId, params, "/renew")
}

func (zohoDeskTool *ZohoDeskTool) Login(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, renewStr string) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Login Execute - Start")
	if zohoDeskTool.AuthName == "" {
		err = errors.New("auth name is required")
		logs.Err(ctx, err, "")
		return nil, nil, false, err
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", zohoDeskTool.AuthName, "/idptoken", renewStr)
	logs.WithContext(ctx).Info(fmt.Sprint("url: ", url))
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	var zohoTokens ZohoTokens
	resBytes, _ := json.Marshal(res)
	err = json.Unmarshal(resBytes, &zohoTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	err = json.Unmarshal(resBytes, &zohoTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	if zohoTokens.RefreshToken == "" {
		zohoTokens.RefreshToken = params["refresh_token"].(string)
	}
	err = zohoDeskTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(zohoDeskTool.ToolName, "_access_token"), zohoTokens.AccessToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	err = zohoDeskTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(zohoDeskTool.ToolName, "_refresh_token"), zohoTokens.RefreshToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	zohoDeskTool.ZohoAccount.TokenExpirationDateTime = time.Now().UTC().Add(time.Duration(zohoTokens.ExpiresIn) * time.Second).Format(time.RFC3339)
	persistStore = true

	if zohoDeskTool.Tool.Hooks.ARRT != "" {
		hookBody := map[string]interface{}{
			"Vars": map[string]interface{}{
				"Body": map[string]interface{}{
					"tool_name": zohoDeskTool.ToolName,
					"tenant_id": tenantId,
				},
				"OrgBody": map[string]interface{}{
					"tool_name": zohoDeskTool.ToolName,
					"tenant_id": tenantId,
				},
			},
			"ReqVars": map[string]interface{}{},
			"ResVars": map[string]interface{}{},
		}
		hookBodyBytes, err := json.Marshal(hookBody)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, nil, persistStore, err
		}
		jobName := fmt.Sprint(zohoDeskTool.ToolName, "_", zohoDeskTool.Tool.Hooks.ARRT, "_", tenantId)
		err = zohoDeskTool.Scheduler.Unschedule(ctx, "", jobName)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}

		schedulerCommand := fmt.Sprint("CALL schedule_procedure('", zohoDeskTool.Tool.Hooks.ARRT, "','", string(hookBodyBytes), "','", zohoDeskTool.Scheduler.GetSchedulerName(), "')")

		cronStr := utils.GetCronStr(ctx, time.Now().UTC().Add(1*time.Hour))
		jobId, err := zohoDeskTool.Scheduler.Schedule(ctx, jobName, schedulerCommand, cronStr)
		if err != nil {
			return nil, nil, persistStore, err
		}
		logs.WithContext(ctx).Info(fmt.Sprint("jobId: ", jobId))
	}
	toolResult = make(map[string]interface{})
	toolResult["login_status"] = "success"
	toolResult["access_token"] = zohoTokens.AccessToken
	toolResult["refresh_token"] = zohoTokens.RefreshToken
	toolResult["expires_in"] = zohoTokens.ExpiresIn
	toolResult["token_type"] = zohoTokens.TokenType
	return toolResult, map[string]interface{}{"body": params}, persistStore, nil
}

func (zohoDeskTool *ZohoDeskTool) SetPrivateAttributes(ctx context.Context, realTool tools.Tooling) (err error) {
	zohoDeskTool.ZohoAccount.AccessToken = fmt.Sprint("$SECRET_", zohoDeskTool.ToolName, "_access_token")
	zohoDeskTool.ZohoAccount.RefreshToken = fmt.Sprint("$SECRET_", zohoDeskTool.ToolName, "_refresh_token")
	return nil
}

func (zohoDeskTool *ZohoDeskTool) GetBytes(ctx context.Context) ([]byte, error) {

	zohoDeskToolWithToken := zohoDeskToolWithToken{
		Tool: zohoDeskTool.Tool,
		ZohoAccount: zohoAccountWithToken{
			OrgId:                   zohoDeskTool.ZohoAccount.OrgId,
			AccessToken:             zohoDeskTool.ZohoAccount.AccessToken,
			RefreshToken:            zohoDeskTool.ZohoAccount.RefreshToken,
			TokenExpirationDateTime: zohoDeskTool.ZohoAccount.TokenExpirationDateTime,
		},
		AuthName: zohoDeskTool.AuthName,
	}

	toolJson, err := json.Marshal(zohoDeskToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (zohoDeskTool *ZohoDeskTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	zohoDeskToolWithToken := zohoDeskToolWithToken{}
	err := json.Unmarshal(toolObjJson, &zohoDeskToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	zohoDeskTool = &ZohoDeskTool{
		Tool: zohoDeskToolWithToken.Tool,
		ZohoAccount: ZohoAccount{
			OrgId:                   zohoDeskToolWithToken.ZohoAccount.OrgId,
			AccessToken:             zohoDeskToolWithToken.ZohoAccount.AccessToken,
			RefreshToken:            zohoDeskToolWithToken.ZohoAccount.RefreshToken,
			TokenExpirationDateTime: zohoDeskToolWithToken.ZohoAccount.TokenExpirationDateTime,
		},
		AuthName: zohoDeskToolWithToken.AuthName,
	}
	return zohoDeskTool, nil
}

func (zohoDeskTool *ZohoDeskTool) StopAutoRenew(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	if zohoDeskTool.Scheduler == nil {
		err = errors.New("scheduler not defined")
		logs.Err(ctx, err, "")
		return nil, nil, false, err
	}
	zohoDeskTool.Scheduler.Unschedule(ctx, "", fmt.Sprint(zohoDeskTool.ToolName, "_", zohoDeskTool.Tool.Hooks.ARRT, "_", tenantId))
	toolResult = make(map[string]interface{})
	toolResult["stop_auto_renew_status"] = "success"
	zohoDeskTool.ZohoAccount.TokenExpirationDateTime = ""
	persistStore = true
	return toolResult, nil, persistStore, nil
}
