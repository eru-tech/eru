package emails

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	INSERT_FUNC_ASYNC = "insert into eruai_cb_msemail (project_id, tenant_id, request_body, request_params) values ($1, $2, $3, $4)"
)

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
		return msEmailTool.SubscribeEmail(ctx, params)
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

func (msEmailTool *MsEmailTool) SubscribeEmail(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("SubscribeEmail Execute - Start")
	url := fmt.Sprint(BaseUrl, "/v1.0/subscriptions")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	subPost := map[string]interface{}{
		"changeType":         "created",
		"notificationUrl":    "https://erufunc.dev.processo.io/processo/func/slack_callback",
		"resource":           "me/mailFolders('Inbox')/messages",
		"expirationDateTime": "2025-04-30T00:00:00Z",
		"clientState":        "39acd634-577e-41ba-aa56-df5695208696",
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

	logs.WithContext(ctx).Info(fmt.Sprint(body))
	logs.WithContext(ctx).Info(fmt.Sprint(params))
	logs.WithContext(ctx).Info(fmt.Sprint(actionName))
	validationString := params["validationToken"][0]

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	paramBytes, err := json.Marshal(params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	var insertQueries []*models.Queries
	insertQueryFuncAsync := models.Queries{}
	insertQueryFuncAsync.Query = msEmailTool.ToolDb.GetDbQuery(ctx, INSERT_FUNC_ASYNC)
	insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, projectId, tenantId, string(bodyBytes), string(paramBytes))
	insertQueryFuncAsync.Rank = 1
	insertQueries = append(insertQueries, &insertQueryFuncAsync)
	_, insertOutputErr := utils.ExecuteDbSave(ctx, msEmailTool.ToolDb.GetConn(), insertQueries)
	if insertOutputErr != nil {
		err = insertOutputErr
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	hookResult, err := msEmailTool.ExecuteCallbackHook(ctx, projectId, tenantId, body, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	logs.WithContext(ctx).Info(fmt.Sprint(hookResult))
	return validationString, nil
}

func (msEmailTool *MsEmailTool) GetToolCbUrl(r *http.Request, projectId string, tenantId string) string {
	scheme := r.URL.Scheme
	host := r.Host
	if scheme == "" {
		return ""
	}
	return fmt.Sprint(scheme, "://", host, "/", projectId, "/", tenantId, "/callback/tool/", msEmailTool.ToolName)
}
