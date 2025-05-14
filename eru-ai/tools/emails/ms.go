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

type MsResourceData struct {
	Id        string `json:"id"`
	ODataId   string `json:"@odata.id"`
	ODataType string `json:"@odata.type"`
	ODataEtag string `json:"@odata.etag"`
}

type MsEmailTool struct {
	tools.Tool
	EmailAccount EmailAccount `json:"email_account"`
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
	logs.WithContext(context.Background()).Info(fmt.Sprintf("Actions List: %v", actions))
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

func (msEmailTool *MsEmailTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("MsEmailTool Execute - Start")
	switch actionName {
	case ReadEmail:
		return msEmailTool.ReadEmail(ctx, params)
	case SendEmail:
		return msEmailTool.SendEmail(ctx, params)
	case SubscribeEmail:
		return msEmailTool.SubscribeEmail(ctx, projectId, tenantId, params)
	case ReadMessage:
		return msEmailTool.ReadMessage(ctx, params)
	default:
		return nil, fmt.Errorf("action %s not found", actionName)
	}
}

func (msEmailTool *MsEmailTool) ReadEmail(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ReadEmail Execute - Start")
	url := fmt.Sprint(BaseUrl, "/v1.0/me/messages")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.SecretName))
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	resbytes, _ := json.Marshal(res)

	logs.WithContext(ctx).Info(string(resbytes))

	logs.WithContext(ctx).Info(msEmailTool.EmailAccount.SecretName)
	_ = url
	return nil, nil
}

func (msEmailTool *MsEmailTool) ReadMessage(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("ReadMessage Execute - Start")
	messageId := params["message_id"].(string)
	url := fmt.Sprint(BaseUrl, "/v1.0/me/messages/", messageId, "?$expand=attachments")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.SecretName))
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	toolResultOk := false
	toolResult, toolResultOk = res.(map[string]interface{})
	if !toolResultOk {
		err = errors.New("toolResult is not a map")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	logs.WithContext(ctx).Info(fmt.Sprint(toolResult))
	return toolResult, nil
}

func (msEmailTool *MsEmailTool) SubscribeEmail(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("SubscribeEmail Execute - Start")
	url := fmt.Sprint(BaseUrl, "/v1.0/subscriptions")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	// Calculate expiration time (current time + 24 hours)
	expirationTime := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	subPost := map[string]interface{}{
		"changeType":         "created",
		"notificationUrl":    msEmailTool.GetToolCbUrl(projectId, tenantId),
		"resource":           "me/mailFolders('Inbox')/messages",
		"expirationDateTime": expirationTime,
		"clientState":        tenantId,
	}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.SecretName))
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, subPost)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	resbytes, _ := json.Marshal(res)

	logs.WithContext(ctx).Info(string(resbytes))

	logs.WithContext(ctx).Info(msEmailTool.EmailAccount.SecretName)
	_ = url
	return nil, nil
}

func (msEmailTool *MsEmailTool) SendEmail(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("SendEmail Execute - Start")
	url := fmt.Sprint("/v1.0/me/sendMail")
	_ = url
	return nil, nil
}

func (msEmailTool *MsEmailTool) GetToolCallback() tools.ToolCallback {
	return tools.ToolCallback{
		ResponseContentType: "plain/text",
	}
}

func (msEmailTool *MsEmailTool) Callback(ctx context.Context, projectId string, tenantId string, actionName string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, err error) {
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
				readMsg, err = msEmailTool.ReadMessage(bgCtx, map[string]interface{}{"message_id": messageId})
				if err != nil {
					logs.WithContext(bgCtx).Error(err.Error())
					return
				}
			}

			if processMsg {
				hookResult, err := msEmailTool.ExecuteCallbackHook(bgCtx, projectId, tenantId, readMsg, params)
				if err != nil {
					logs.WithContext(bgCtx).Error(err.Error())
					return
				}
				logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
			}
		}
	}()

	return validationString, nil
}

func (msEmailTool *MsEmailTool) GetToolCbUrl(projectId string, tenantId string) string {
	return fmt.Sprint(msEmailTool.CallbackBaseUrl, "/", projectId, "/", tenantId, "/callback/tool/", msEmailTool.ToolName)
}
