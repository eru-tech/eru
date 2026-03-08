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
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	GoogleGmailBaseUrl    = "https://gmail.googleapis.com/gmail/v1/users/me"
	GoogleCalendarBaseUrl = "https://www.googleapis.com/calendar/v3"
)

type GoogleMailTool struct {
	tools.Tool
	EmailAccount EmailAccount `json:"email_account"`
	AuthName     string       `json:"auth_name"`
}

type googleMailToolWithToken struct {
	tools.Tool
	EmailAccount emailAccountWithToken `json:"email_account"`
	AuthName     string                `json:"auth_name"`
}

func (g *GoogleMailTool) GetSpec() tools.Tooling {
	return g
}

func (g *GoogleMailTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("GoogleMailTool MakeFromJson - Start")
	if err := json.Unmarshal(*rj, g); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (g *GoogleMailTool) GetActionsList() []string {
	return []string{
		ReadEmail, SendEmail, ReadMessage,
		GetSsoUrl, Login, RenewToken,
		ListCalendarEvents, GetCalendarEvent, CreateCalendarEvent,
		UpdateCalendarEvent, DeleteCalendarEvent, AcceptCalendarEvent,
		DeclineCalendarEvent, ListCalendars, SubscribeCalendar,
	}
}

func (g *GoogleMailTool) GetMcpTools() []tools.McpToolList {
	return []tools.McpToolList{
		{ToolName: ReadEmail, ToolDescription: "Read emails from your Gmail account", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", ReadEmail)},
		{ToolName: SendEmail, ToolDescription: "Send an email from your Gmail account", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", SendEmail)},
		{ToolName: ReadMessage, ToolDescription: "Read a specific Gmail message with attachments", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", ReadMessage)},
		{ToolName: ListCalendarEvents, ToolDescription: "List Google Calendar events", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", ListCalendarEvents)},
		{ToolName: GetCalendarEvent, ToolDescription: "Get a specific Google Calendar event", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", GetCalendarEvent)},
		{ToolName: CreateCalendarEvent, ToolDescription: "Create a Google Calendar event", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", CreateCalendarEvent)},
		{ToolName: UpdateCalendarEvent, ToolDescription: "Update a Google Calendar event", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", UpdateCalendarEvent)},
		{ToolName: DeleteCalendarEvent, ToolDescription: "Delete a Google Calendar event", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", DeleteCalendarEvent)},
		{ToolName: AcceptCalendarEvent, ToolDescription: "Accept a Google Calendar event invitation", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", AcceptCalendarEvent)},
		{ToolName: DeclineCalendarEvent, ToolDescription: "Decline a Google Calendar event invitation", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", DeclineCalendarEvent)},
		{ToolName: ListCalendars, ToolDescription: "List all Google Calendars", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", ListCalendars)},
		{ToolName: SubscribeCalendar, ToolDescription: "Subscribe to Google Calendar change notifications", ComponentUrl: fmt.Sprintf("/tools/%s/component.json", SubscribeCalendar)},
	}
}

func (g *GoogleMailTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GoogleMailTool Execute - Start")
	switch actionName {
	case ReadEmail:
		return g.ReadEmail(ctx, params)
	case SendEmail:
		return g.SendEmail(ctx, params)
	case ReadMessage:
		return g.ReadMessage(ctx, params)
	case GetSsoUrl:
		return g.GetSsoUrl(ctx, projectId, tenantId, params)
	case Login:
		return g.Login(ctx, projectId, tenantId, params, "")
	case RenewToken:
		return g.RenewToken(ctx, projectId, tenantId, params)
	case ListCalendarEvents:
		return g.ListCalendarEvents(ctx, params)
	case GetCalendarEvent:
		return g.GetCalendarEvent(ctx, params)
	case CreateCalendarEvent:
		return g.CreateCalendarEvent(ctx, params)
	case UpdateCalendarEvent:
		return g.UpdateCalendarEvent(ctx, params)
	case DeleteCalendarEvent:
		return g.DeleteCalendarEvent(ctx, params)
	case AcceptCalendarEvent:
		return g.AcceptCalendarEvent(ctx, params)
	case DeclineCalendarEvent:
		return g.DeclineCalendarEvent(ctx, params)
	case ListCalendars:
		return g.ListCalendars(ctx, params)
	case SubscribeCalendar:
		return g.SubscribeCalendar(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (g *GoogleMailTool) SetPrivateAttributes(ctx context.Context, realTool tools.Tooling) error {
	g.EmailAccount.AccessToken = "$SECRET_google_access_token"
	g.EmailAccount.RefreshToken = "$SECRET_google_refresh_token"
	return nil
}

func (g *GoogleMailTool) GetBytes(ctx context.Context) ([]byte, error) {
	withToken := googleMailToolWithToken{
		Tool: g.Tool,
		EmailAccount: emailAccountWithToken{
			DisplayName:                    g.EmailAccount.DisplayName,
			AccessToken:                    g.EmailAccount.AccessToken,
			RefreshToken:                   g.EmailAccount.RefreshToken,
			SubscriptionId:                 g.EmailAccount.SubscriptionId,
			SubscriptionExpirationDateTime: g.EmailAccount.SubscriptionExpirationDateTime,
			TokenExpirationDateTime:        g.EmailAccount.TokenExpirationDateTime,
		},
		AuthName: g.AuthName,
	}
	b, err := json.Marshal(withToken)
	if err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return b, nil
}

func (g *GoogleMailTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	withToken := googleMailToolWithToken{}
	if err := json.Unmarshal(toolObjJson, &withToken); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return &GoogleMailTool{
		Tool: withToken.Tool,
		EmailAccount: EmailAccount{
			DisplayName:                    withToken.EmailAccount.DisplayName,
			AccessToken:                    withToken.EmailAccount.AccessToken,
			RefreshToken:                   withToken.EmailAccount.RefreshToken,
			SubscriptionId:                 withToken.EmailAccount.SubscriptionId,
			SubscriptionExpirationDateTime: withToken.EmailAccount.SubscriptionExpirationDateTime,
			TokenExpirationDateTime:        withToken.EmailAccount.TokenExpirationDateTime,
		},
		AuthName: withToken.AuthName,
	}, nil
}

// --- Gmail ---

func (g *GoogleMailTool) authHeader() string {
	return fmt.Sprintf("Bearer %s", g.EmailAccount.AccessToken)
}

func (g *GoogleMailTool) ReadEmail(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool ReadEmail - Start")
	url := fmt.Sprint(GoogleGmailBaseUrl, "/messages")
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	strParams := make(map[string]string)
	for k, v := range params {
		strParams[k] = fmt.Sprint(v)
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, strParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	return map[string]interface{}{"emails": res}, false, nil
}

func (g *GoogleMailTool) ReadMessage(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool ReadMessage - Start")
	messageId, ok := params["message_id"].(string)
	if !ok || messageId == "" {
		return nil, false, errors.New("message_id is required")
	}
	url := fmt.Sprint(GoogleGmailBaseUrl, "/messages/", messageId)
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	strParams := map[string]string{"format": "full"}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, strParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult, ok := res.(map[string]interface{})
	if !ok {
		return nil, false, errors.New("unexpected response format")
	}
	return toolResult, false, nil
}

func (g *GoogleMailTool) SendEmail(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool SendEmail - Start")
	url := fmt.Sprint(GoogleGmailBaseUrl, "/messages/send")
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult, ok := res.(map[string]interface{})
	if !ok {
		return map[string]interface{}{"status": "sent"}, false, nil
	}
	return toolResult, false, nil
}

// --- Auth (same pattern as MS) ---

func (g *GoogleMailTool) GetSsoUrl(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool GetSsoUrl - Start")
	if g.AuthName == "" {
		return nil, false, errors.New("auth_name is required")
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", g.AuthName, "/getssourl")
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
	toolResult, ok := res.(map[string]interface{})
	if !ok {
		return nil, false, errors.New("unexpected response format")
	}
	return toolResult, false, nil
}

func (g *GoogleMailTool) RenewToken(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	params["refresh_token"] = g.EmailAccount.RefreshToken
	return g.Login(ctx, projectId, tenantId, params, "/renew")
}

func (g *GoogleMailTool) Login(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, renewStr string) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool Login - Start")
	if g.AuthName == "" {
		return nil, false, errors.New("auth_name is required")
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", g.AuthName, "/idptoken", renewStr)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	var msTokens MsTokens
	resBytes, _ := json.Marshal(res)
	if err := json.Unmarshal(resBytes, &msTokens); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	if err := g.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(g.AuthName, "_access_token"), msTokens.AccessToken); err != nil {
		return nil, false, err
	}
	if err := g.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(g.AuthName, "_refresh_token"), msTokens.RefreshToken); err != nil {
		return nil, false, err
	}
	g.EmailAccount.TokenExpirationDateTime = time.Now().UTC().Add(time.Duration(msTokens.ExpiresIn) * time.Second).Format(time.RFC3339)
	return map[string]interface{}{"login_status": "success"}, true, nil
}

// --- Google Calendar ---

func (g *GoogleMailTool) calendarUrl(calendarId string) string {
	if calendarId == "" {
		calendarId = "primary"
	}
	return fmt.Sprint(GoogleCalendarBaseUrl, "/calendars/", calendarId)
}

func (g *GoogleMailTool) ListCalendarEvents(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool ListCalendarEvents - Start")
	calendarId, _ := params["calendar_id"].(string)
	url := fmt.Sprint(g.calendarUrl(calendarId), "/events")
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	strParams := make(map[string]string)
	for k, v := range params {
		if k != "calendar_id" {
			strParams[k] = fmt.Sprint(v)
		}
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, strParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	return map[string]interface{}{"events": res}, false, nil
}

func (g *GoogleMailTool) GetCalendarEvent(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool GetCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	calendarId, _ := params["calendar_id"].(string)
	url := fmt.Sprint(g.calendarUrl(calendarId), "/events/", eventId)
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult, ok := res.(map[string]interface{})
	if !ok {
		return nil, false, errors.New("unexpected response format")
	}
	return toolResult, false, nil
}

func (g *GoogleMailTool) CreateCalendarEvent(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool CreateCalendarEvent - Start")
	calendarId, _ := params["calendar_id"].(string)
	delete(params, "calendar_id")
	url := fmt.Sprint(g.calendarUrl(calendarId), "/events")
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult, ok := res.(map[string]interface{})
	if !ok {
		return nil, false, errors.New("unexpected response format")
	}
	return toolResult, false, nil
}

func (g *GoogleMailTool) UpdateCalendarEvent(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool UpdateCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	calendarId, _ := params["calendar_id"].(string)
	delete(params, "event_id")
	delete(params, "calendar_id")
	url := fmt.Sprint(g.calendarUrl(calendarId), "/events/", eventId)
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPut, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult, ok := res.(map[string]interface{})
	if !ok {
		return map[string]interface{}{"status": "updated"}, false, nil
	}
	return toolResult, false, nil
}

func (g *GoogleMailTool) DeleteCalendarEvent(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool DeleteCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	calendarId, _ := params["calendar_id"].(string)
	url := fmt.Sprint(g.calendarUrl(calendarId), "/events/", eventId)
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	_, _, _, _, err := utils.CallHttp(ctx, http.MethodDelete, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	return map[string]interface{}{"status": "deleted"}, false, nil
}

func (g *GoogleMailTool) AcceptCalendarEvent(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool AcceptCalendarEvent - Start")
	return g.patchEventStatus(ctx, params, "accepted")
}

func (g *GoogleMailTool) DeclineCalendarEvent(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool DeclineCalendarEvent - Start")
	return g.patchEventStatus(ctx, params, "declined")
}

func (g *GoogleMailTool) patchEventStatus(ctx context.Context, params map[string]interface{}, status string) (map[string]interface{}, bool, error) {
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	calendarId, _ := params["calendar_id"].(string)
	selfEmail, _ := params["self_email"].(string)
	url := fmt.Sprint(g.calendarUrl(calendarId), "/events/", eventId)
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	headers.Set("Content-Type", "application/json")
	body := map[string]interface{}{
		"attendees": []map[string]interface{}{
			{"email": selfEmail, "responseStatus": status},
		},
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPatch, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult, ok := res.(map[string]interface{})
	if !ok {
		return map[string]interface{}{"status": status}, false, nil
	}
	return toolResult, false, nil
}

func (g *GoogleMailTool) ListCalendars(ctx context.Context, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool ListCalendars - Start")
	url := fmt.Sprint(GoogleCalendarBaseUrl, "/users/me/calendarList")
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	strParams := make(map[string]string)
	for k, v := range params {
		strParams[k] = fmt.Sprint(v)
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, strParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	return map[string]interface{}{"calendars": res}, false, nil
}

func (g *GoogleMailTool) SubscribeCalendar(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, bool, error) {
	logs.WithContext(ctx).Debug("GoogleMailTool SubscribeCalendar - Start")
	calendarId, _ := params["calendar_id"].(string)
	url := fmt.Sprint(g.calendarUrl(calendarId), "/events/watch")
	headers := http.Header{}
	headers.Set("Authorization", g.authHeader())
	headers.Set("Content-Type", "application/json")
	channelId := fmt.Sprint(tenantId, "-calendar-", time.Now().Unix())
	body := map[string]interface{}{
		"id":      channelId,
		"type":    "web_hook",
		"address": g.GetToolCbUrl(projectId, tenantId),
		"params":  map[string]interface{}{"ttl": "86400"},
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	subResult, ok := res.(map[string]interface{})
	if !ok {
		return nil, false, errors.New("unexpected subscription response")
	}
	if id, ok := subResult["resourceId"].(string); ok {
		g.EmailAccount.SubscriptionId = id
		g.EmailAccount.SubscriptionExpirationDateTime = fmt.Sprint(subResult["expiration"])
	}
	return map[string]interface{}{"subscription_status": "success", "channel_id": channelId, "resource_id": subResult["resourceId"]}, true, nil
}

func (g *GoogleMailTool) GetToolCbUrl(projectId string, tenantId string) string {
	return fmt.Sprint(g.CallbackBaseUrl, "/", projectId, "/callback/", tenantId, "/tool/", g.ToolName)
}
