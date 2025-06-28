package emails

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	INSERT_FUNC_ASYNC = "insert into eruai_cb_msemail (project_id, tenant_id, request_body, request_params) values ($1, $2, $3, $4)"
)

type MsNotificationCollection struct {
	Value []MsNotification `json:"value"`
}

type MsNotification struct {
	ChangeType                     string         `json:"changeType"`
	ClientState                    string         `json:"clientState"`
	Resource                       string         `json:"resource"`
	ResourceData                   MsResourceData `json:"resourceData"`
	SubscriptionId                 string         `json:"subscriptionId"`
	Id                             string         `json:"id"`
	LifecycleEvent                 string         `json:"lifecycleEvent"`
	SubscriptionExpirationDateTime string         `json:"subscriptionExpirationDateTime"`
	TenantId                       string         `json:"tenantId"`
}

type MsTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExtExpiresIn int    `json:"ext_expires_in"`
}

type MsResourceData struct {
	Id        string `json:"id"`
	ODataId   string `json:"@odata.id"`
	ODataType string `json:"@odata.type"`
	ODataEtag string `json:"@odata.etag"`
}

type MsEmailTool struct {
	tools.Tool
	EmailAccount EmailAccount `json:"email_account"`
	AuthName     string       `json:"auth_name"`
}
type msEmailToolWithToken struct {
	tools.Tool
	EmailAccount emailAccountWithToken
	AuthName     string
}

const (
	BaseUrl = "https://graph.microsoft.com"
)

func (msEmailTool *MsEmailTool) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, ReadEmail)
	actions = append(actions, SendEmail)
	actions = append(actions, SubscribeEmail)
	actions = append(actions, ReadMessage)
	actions = append(actions, Callback)
	actions = append(actions, GetSsoUrl)
	actions = append(actions, Login)
	actions = append(actions, RenewToken)
	actions = append(actions, RenewSubscription)
	actions = append(actions, StopAutoRenew)
	actions = append(actions, StopSubscription)
	return actions
}

func (msEmailTool *MsEmailTool) GetMcpTools() []tools.McpToolList {
	mcpTools := []tools.McpToolList{}
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        ReadEmail,
		ToolDescription: "Read Emails from your Microsoft 365 account",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", ReadEmail),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        SendEmail,
		ToolDescription: "Send Emails from your Microsoft 365 account",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", SendEmail),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        SubscribeEmail,
		ToolDescription: "Subscribe to your Microsoft 365 account",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", SubscribeEmail),
	})
	return mcpTools
}

func (msEmailTool *MsEmailTool) GetSpec() tools.Tooling {
	return msEmailTool
}

func (msEmailTool *MsEmailTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &msEmailTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (msEmailTool *MsEmailTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("MsEmailTool Execute - Start")
	switch actionName {
	case ReadEmail:
		return msEmailTool.ReadEmail(ctx, params)
	case SendEmail:
		return msEmailTool.SendEmail(ctx, params)
	case SubscribeEmail:
		return msEmailTool.SubscribeEmail(ctx, projectId, tenantId, params, "", false)
	case ReadMessage:
		return msEmailTool.ReadMessage(ctx, params)
	case GetSsoUrl:
		return msEmailTool.GetSsoUrl(ctx, projectId, tenantId, params)
	case Login:
		return msEmailTool.Login(ctx, projectId, tenantId, params, "")
	case RenewToken:
		return msEmailTool.RenewToken(ctx, projectId, tenantId, params)
	case RenewSubscription:
		return msEmailTool.RenewSubscription(ctx, projectId, tenantId, params)
	case StopAutoRenew:
		return msEmailTool.StopAutoRenew(ctx, projectId, tenantId, params)
	case StopSubscription:
		return msEmailTool.StopSubscription(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (msEmailTool *MsEmailTool) ReadEmail(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ReadEmail Execute - Start")
	url := fmt.Sprint(BaseUrl, "/v1.0/me/messages")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	resbytes, _ := json.Marshal(res)

	logs.WithContext(ctx).Info(string(resbytes))

	logs.WithContext(ctx).Info(msEmailTool.EmailAccount.AccessToken)
	_ = url
	toolResult = make(map[string]interface{})
	toolResult["emails"] = res
	return toolResult, false, nil
}

func (msEmailTool *MsEmailTool) ReadMessage(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ReadMessage Execute - Start")
	messageId := params["message_id"].(string)
	url := fmt.Sprint(BaseUrl, "/v1.0/me/messages/", messageId, "?$expand=attachments")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResultOk := false
	toolResult, toolResultOk = res.(map[string]interface{})
	if !toolResultOk {
		err = errors.New("toolResult is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	return toolResult, false, nil
}

func (msEmailTool *MsEmailTool) SubscribeEmail(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, subscriptionId string, unsubscribe bool) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SubscribeEmail Execute - Start")
	url := fmt.Sprint(BaseUrl, "/v1.0/subscriptions", subscriptionId)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	// Calculate expiration time (current time + 24 hours)
	expirationTime := time.Now().UTC().Add(50 * time.Hour).Format(time.RFC3339)

	subPost := make(map[string]interface{})
	httpMethod := http.MethodPost
	if unsubscribe {
		httpMethod = http.MethodDelete
		subPost = nil
		subscriptionId = ""
	} else if subscriptionId == "" {
		subPost = map[string]interface{}{
			"changeType":         "created",
			"notificationUrl":    msEmailTool.GetToolCbUrl(projectId, tenantId),
			"resource":           "me/mailFolders('Inbox')/messages",
			"expirationDateTime": expirationTime,
			"clientState":        tenantId,
		}
	} else {
		httpMethod = http.MethodPatch
		subPost = map[string]interface{}{
			"expirationDateTime": expirationTime,
		}
	}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	res, _, _, _, err := utils.CallHttp(ctx, httpMethod, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, subPost)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	logs.WithContext(ctx).Info(fmt.Sprint(res))

	if unsubscribe {
		jobName := fmt.Sprint(msEmailTool.Tool.Hooks.ARSU, "_", tenantId)
		err = msEmailTool.Scheduler.Unschedule(ctx, "", jobName)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			//continue even if unschedule fails
		}
		toolResult = make(map[string]interface{})
		msEmailTool.EmailAccount.SubscriptionId = ""
		msEmailTool.EmailAccount.SubscriptionExpirationDateTime = ""
		persistStore = true
		toolResult["unsubscription_status"] = "success"
		return toolResult, persistStore, nil
	}

	subResultOk := false
	subResult, subResultOk := res.(map[string]interface{})
	if !subResultOk {
		err = errors.New("subResult is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	subcriptionId, subcriptionIdOk := subResult["id"]
	if !subcriptionIdOk {
		err = errors.New("subcription id not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	msEmailTool.EmailAccount.SubscriptionId = subcriptionId.(string)
	msEmailTool.EmailAccount.SubscriptionExpirationDateTime = expirationTime
	persistStore = true

	if msEmailTool.Tool.Hooks.ARSU != "" {
		hookBody := map[string]interface{}{
			"Vars": map[string]interface{}{
				"Body": map[string]interface{}{
					"tool_name": msEmailTool.ToolName,
					"tenant_id": tenantId,
				},
				"OrgBody": map[string]interface{}{
					"tool_name": msEmailTool.ToolName,
					"tenant_id": tenantId,
				},
			},
			"ReqVars": map[string]interface{}{},
			"ResVars": map[string]interface{}{},
		}
		hookBodyBytes, err := json.Marshal(hookBody)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, persistStore, err
		}
		jobName := fmt.Sprint(msEmailTool.Tool.Hooks.ARSU, "_", tenantId)
		err = msEmailTool.Scheduler.Unschedule(ctx, "", jobName)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			//continue even if unschedule fails
		}
		cronStr := utils.GetCronStr(ctx, time.Now().UTC().Add(48*time.Hour))
		schedulerCommand := fmt.Sprint("CALL schedule_procedure('", msEmailTool.Tool.Hooks.ARSU, "','", string(hookBodyBytes), "','", msEmailTool.Scheduler.GetSchedulerName(), "')")
		jobId, err := msEmailTool.Scheduler.Schedule(ctx, jobName, schedulerCommand, cronStr)
		if err != nil {
			return nil, persistStore, err
		}
		logs.WithContext(ctx).Info(fmt.Sprint("jobId: ", jobId))
	}
	toolResult = make(map[string]interface{})
	toolResult["subscription_status"] = "success"
	toolResult["subscription_id"] = msEmailTool.EmailAccount.SubscriptionId
	_ = url
	return toolResult, persistStore, nil
}

func (msEmailTool *MsEmailTool) SendEmail(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SendEmail Execute - Start")
	url := fmt.Sprint("/v1.0/me/sendMail")
	_ = url
	return nil, false, nil
}

func (msEmailTool *MsEmailTool) GetToolCallback() tools.ToolCallback {
	return tools.ToolCallback{
		ResponseContentType: "plain/text",
	}
}

func (msEmailTool *MsEmailTool) Callback(ctx context.Context, projectId string, tenantId string, actionName string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Callback Execute - Start")
	_ = actionName
	_ = body
	_ = params

	validationString := ""
	if vToken, vTokenOk := params["validationToken"]; vTokenOk {
		validationString = vToken[0]
	}

	// Process the message in a separate goroutine with a new context
	go func() {
		// Create a new background context for the goroutine
		bgCtx := context.Background()
		// Copy any important values from the original context if needed
		if eruFuncBaseUrl, ok := ctx.Value("Erufuncbaseurl").(string); ok {
			bgCtx = context.WithValue(bgCtx, "Erufuncbaseurl", eruFuncBaseUrl)
		}

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}

		paramBytes, err := json.Marshal(params)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}

		var insertQueries []*models.Queries
		insertQueryFuncAsync := models.Queries{}
		insertQueryFuncAsync.Query = msEmailTool.ToolDb.GetDbQuery(bgCtx, INSERT_FUNC_ASYNC)
		insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, projectId, tenantId, string(bodyBytes), string(paramBytes))
		insertQueryFuncAsync.Rank = 1
		insertQueries = append(insertQueries, &insertQueryFuncAsync)
		_, insertOutputErr := utils.ExecuteDbSave(bgCtx, msEmailTool.ToolDb.GetConn(), insertQueries)
		if insertOutputErr != nil {
			logs.WithContext(bgCtx).Error(insertOutputErr.Error())
			return
		}

		var notificationCollection MsNotificationCollection
		err = json.Unmarshal(bodyBytes, &notificationCollection)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}

		for _, notification := range notificationCollection.Value {
			processMsg := true

			if notification.ClientState != tenantId {
				processMsg = false
				logs.WithContext(bgCtx).Info(fmt.Sprint("notification client state does not match tenant id : ", tenantId, " : ", notification.ClientState))
				break
			}
			readMsg := make(map[string]interface{})
			if notification.ResourceData.ODataType != "microsoft.graph.message" && notification.ChangeType == "created" {
				messageId := notification.ResourceData.Id
				readMsg, _, err = msEmailTool.ReadMessage(bgCtx, map[string]interface{}{"message_id": messageId})
				if err != nil {
					logs.WithContext(bgCtx).Error(err.Error())
					return
				}
			}

			if processMsg {
				body := map[string]interface{}{
					"mail":      readMsg,
					"tenant_id": tenantId,
				}
				hookResult, err := msEmailTool.ExecuteCallbackHook(bgCtx, projectId, tenantId, body, params)
				if err != nil {
					logs.WithContext(bgCtx).Error(err.Error())
					return
				}
				logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
			}
		}
	}()

	return validationString, false, nil
}

func (msEmailTool *MsEmailTool) GetToolCbUrl(projectId string, tenantId string) string {
	return fmt.Sprint(msEmailTool.CallbackBaseUrl, "/", projectId, "/", tenantId, "/callback/tool/", msEmailTool.ToolName)
}

func (msEmailTool *MsEmailTool) GetSsoUrl(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetSsoUrl Execute - Start")
	if msEmailTool.AuthName == "" {
		err = errors.New("auth name is required")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", msEmailTool.AuthName, "/getssourl")
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
		return nil, false, err
	}
	toolResultOk := false
	toolResult, toolResultOk = res.(map[string]interface{})
	if !toolResultOk {
		err = errors.New("toolResult is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	logs.WithContext(ctx).Info(fmt.Sprint("toolResult: ", toolResult))
	return toolResult, false, nil
}
func (msEmailTool *MsEmailTool) RenewToken(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	params["refresh_token"] = msEmailTool.EmailAccount.RefreshToken
	return msEmailTool.Login(ctx, projectId, tenantId, params, "/renew")
}
func (msEmailTool *MsEmailTool) RenewSubscription(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("RenewSubscription Execute - Start")
	subscriptionId := msEmailTool.EmailAccount.SubscriptionId
	if subscriptionId == "" {
		err = errors.New("subscription id is required")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	return msEmailTool.SubscribeEmail(ctx, projectId, tenantId, params, fmt.Sprint("/", subscriptionId), false)
}
func (msEmailTool *MsEmailTool) Login(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, renewStr string) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Login Execute - Start")
	if msEmailTool.AuthName == "" {
		err = errors.New("auth name is required")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", msEmailTool.AuthName, "/idptoken", renewStr)
	logs.WithContext(ctx).Info(fmt.Sprint("url: ", url))
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	var msTokens MsTokens
	resBytes, _ := json.Marshal(res)
	err = json.Unmarshal(resBytes, &msTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = json.Unmarshal(resBytes, &msTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = msEmailTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(msEmailTool.AuthName, "_access_token"), msTokens.AccessToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = msEmailTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(msEmailTool.AuthName, "_refresh_token"), msTokens.RefreshToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = msEmailTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(msEmailTool.AuthName, "_id_token"), msTokens.IdToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	msEmailTool.EmailAccount.TokenExpirationDateTime = time.Now().UTC().Add(time.Duration(msTokens.ExpiresIn) * time.Second).Format(time.RFC3339)
	persistStore = true

	if msEmailTool.Tool.Hooks.ARRT != "" {
		hookBody := map[string]interface{}{
			"Vars": map[string]interface{}{
				"Body": map[string]interface{}{
					"tool_name": msEmailTool.ToolName,
					"tenant_id": tenantId,
				},
				"OrgBody": map[string]interface{}{
					"tool_name": msEmailTool.ToolName,
					"tenant_id": tenantId,
				},
			},
			"ReqVars": map[string]interface{}{},
			"ResVars": map[string]interface{}{},
		}
		hookBodyBytes, err := json.Marshal(hookBody)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, persistStore, err
		}
		jobName := fmt.Sprint(msEmailTool.Tool.Hooks.ARRT, "_", tenantId)
		err = msEmailTool.Scheduler.Unschedule(ctx, "", jobName)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			//continue even if unschedule fails
		}

		schedulerCommand := fmt.Sprint("CALL schedule_procedure('", msEmailTool.Tool.Hooks.ARRT, "','", string(hookBodyBytes), "','", msEmailTool.Scheduler.GetSchedulerName(), "')")

		cronStr := utils.GetCronStr(ctx, time.Now().UTC().Add(1*time.Hour))
		jobId, err := msEmailTool.Scheduler.Schedule(ctx, jobName, schedulerCommand, cronStr)
		if err != nil {
			return nil, persistStore, err
		}
		logs.WithContext(ctx).Info(fmt.Sprint("jobId: ", jobId))
	}
	toolResult = make(map[string]interface{})
	toolResult["login_status"] = "success"
	return toolResult, persistStore, nil
}

func (msEmailTool *MsEmailTool) SetPrivateAttributes(ctx context.Context, realTool tools.Tooling) (err error) {
	msEmailTool.EmailAccount.AccessToken = "$SECRET_msmail_access_token"
	msEmailTool.EmailAccount.RefreshToken = "$SECRET_msmail_refresh_token"
	return nil
}

func (msEmailTool *MsEmailTool) GetBytes(ctx context.Context) ([]byte, error) {

	msEmailToolWithToken := msEmailToolWithToken{
		Tool: msEmailTool.Tool,
		EmailAccount: emailAccountWithToken{
			DisplayName:                    msEmailTool.EmailAccount.DisplayName,
			AccessToken:                    msEmailTool.EmailAccount.AccessToken,
			RefreshToken:                   msEmailTool.EmailAccount.RefreshToken,
			SubscriptionId:                 msEmailTool.EmailAccount.SubscriptionId,
			SubscriptionExpirationDateTime: msEmailTool.EmailAccount.SubscriptionExpirationDateTime,
			TokenExpirationDateTime:        msEmailTool.EmailAccount.TokenExpirationDateTime,
		},
		AuthName: msEmailTool.AuthName,
	}

	toolJson, err := json.Marshal(msEmailToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}
func (msEmailTool *MsEmailTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	msEmailToolWithToken := msEmailToolWithToken{}
	err := json.Unmarshal(toolObjJson, &msEmailToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	msEmailTool = &MsEmailTool{
		Tool: msEmailToolWithToken.Tool,
		EmailAccount: EmailAccount{
			DisplayName:                    msEmailToolWithToken.EmailAccount.DisplayName,
			AccessToken:                    msEmailToolWithToken.EmailAccount.AccessToken,
			RefreshToken:                   msEmailToolWithToken.EmailAccount.RefreshToken,
			SubscriptionId:                 msEmailToolWithToken.EmailAccount.SubscriptionId,
			SubscriptionExpirationDateTime: msEmailToolWithToken.EmailAccount.SubscriptionExpirationDateTime,
			TokenExpirationDateTime:        msEmailToolWithToken.EmailAccount.TokenExpirationDateTime,
		},
		AuthName: msEmailToolWithToken.AuthName,
	}
	return msEmailTool, nil
}
func (msEmailTool *MsEmailTool) StopAutoRenew(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	if msEmailTool.Scheduler == nil {
		err = errors.New("scheduler not defined")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	msEmailTool.Scheduler.Unschedule(ctx, "", fmt.Sprint(msEmailTool.Tool.Hooks.ARRT, "_", tenantId))
	toolResult = make(map[string]interface{})
	toolResult["stop_auto_renew_status"] = "success"
	msEmailTool.EmailAccount.TokenExpirationDateTime = ""
	persistStore = true
	return toolResult, persistStore, nil
}
func (msEmailTool *MsEmailTool) StopSubscription(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("StopSubscription Execute - Start")
	subscriptionId := msEmailTool.EmailAccount.SubscriptionId
	if subscriptionId == "" {
		err = errors.New("subscription id is required")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	return msEmailTool.SubscribeEmail(ctx, projectId, tenantId, params, fmt.Sprint("/", subscriptionId), true)
}
