package emails

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
)

func (msEmailTool *MsEmailTool) ListCalendarEvents(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ListCalendarEvents - Start")
	url := fmt.Sprint(BaseUrl, "/v1.0/me/events")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	headers.Set("Content-Type", "application/json")
	strParams := make(map[string]string)
	for k, v := range params {
		strParams[k] = fmt.Sprint(v)
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, strParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult = map[string]interface{}{"events": res}
	return toolResult, false, nil
}

func (msEmailTool *MsEmailTool) GetCalendarEvent(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	url := fmt.Sprint(BaseUrl, "/v1.0/me/events/", eventId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult, ok = res.(map[string]interface{})
	if !ok {
		return nil, false, errors.New("unexpected response format")
	}
	return toolResult, false, nil
}

func (msEmailTool *MsEmailTool) CreateCalendarEvent(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CreateCalendarEvent - Start")
	url := fmt.Sprint(BaseUrl, "/v1.0/me/events")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
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

func (msEmailTool *MsEmailTool) UpdateCalendarEvent(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("UpdateCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	delete(params, "event_id")
	url := fmt.Sprint(BaseUrl, "/v1.0/me/events/", eventId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPatch, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	toolResult, ok = res.(map[string]interface{})
	if !ok {
		toolResult = map[string]interface{}{"status": "updated"}
	}
	return toolResult, false, nil
}

func (msEmailTool *MsEmailTool) DeleteCalendarEvent(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("DeleteCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	url := fmt.Sprint(BaseUrl, "/v1.0/me/events/", eventId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	_, _, _, _, err = utils.CallHttp(ctx, http.MethodDelete, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	return map[string]interface{}{"status": "deleted"}, false, nil
}

func (msEmailTool *MsEmailTool) respondToCalendarEvent(ctx context.Context, eventId string, action string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	url := fmt.Sprint(BaseUrl, "/v1.0/me/events/", eventId, "/", action)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	headers.Set("Content-Type", "application/json")
	_, _, _, _, err = utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	return map[string]interface{}{"status": action}, false, nil
}

func (msEmailTool *MsEmailTool) AcceptCalendarEvent(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("AcceptCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	delete(params, "event_id")
	return msEmailTool.respondToCalendarEvent(ctx, eventId, "accept", params)
}

func (msEmailTool *MsEmailTool) DeclineCalendarEvent(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("DeclineCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	delete(params, "event_id")
	return msEmailTool.respondToCalendarEvent(ctx, eventId, "decline", params)
}

func (msEmailTool *MsEmailTool) TentativeCalendarEvent(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("TentativeCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	delete(params, "event_id")
	return msEmailTool.respondToCalendarEvent(ctx, eventId, "tentativelyAccept", params)
}

func (msEmailTool *MsEmailTool) CancelCalendarEvent(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CancelCalendarEvent - Start")
	eventId, ok := params["event_id"].(string)
	if !ok || eventId == "" {
		return nil, false, errors.New("event_id is required")
	}
	delete(params, "event_id")
	return msEmailTool.respondToCalendarEvent(ctx, eventId, "cancel", params)
}

func (msEmailTool *MsEmailTool) ListCalendars(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ListCalendars - Start")
	url := fmt.Sprint(BaseUrl, "/v1.0/me/calendars")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	headers.Set("Content-Type", "application/json")
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

func (msEmailTool *MsEmailTool) SubscribeCalendar(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SubscribeCalendar - Start")
	url := fmt.Sprint(BaseUrl, "/v1.0/subscriptions")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", msEmailTool.EmailAccount.AccessToken))
	headers.Set("Content-Type", "application/json")
	expirationTime := time.Now().UTC().Add(50 * time.Hour).Format(time.RFC3339)
	body := map[string]interface{}{
		"changeType":         "created,updated,deleted",
		"notificationUrl":    msEmailTool.GetToolCbUrl(projectId, tenantId),
		"resource":           "me/events",
		"expirationDateTime": expirationTime,
		"clientState":        tenantId,
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
	if id, ok := subResult["id"].(string); ok {
		msEmailTool.EmailAccount.SubscriptionId = id
		msEmailTool.EmailAccount.SubscriptionExpirationDateTime = expirationTime
		persistStore = true
	}
	return map[string]interface{}{"subscription_status": "success", "subscription_id": subResult["id"]}, persistStore, nil
}
