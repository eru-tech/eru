package messengers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	INSERT_FUNC_ASYNC_SLACK = "insert into eruai_cb_slack (project_id, tenant_id, request_body, request_params) values ($1, $2, $3, $4)"
	SLACK_BASE_URL          = "https://slack.com/api"
	JoinChannel             = "join_channel"
)

type SlackTool struct {
	tools.Tool
	SlackAccount SlackAccount `json:"slack_account"`
	AuthName     string       `json:"auth_name"`
}

type slackToolWithToken struct {
	tools.Tool
	SlackAccount slackAccountWithToken
	AuthName     string
}

func (slackTool *SlackTool) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, SendMessage, ReadMessages, Login, GetSsoUrl, SubscribeWebhooks,
		ListChannels, ListUsers, CreateChannel, InviteToChannel, JoinChannel, UploadMedia, Callback)
	return actions
}

func (slackTool *SlackTool) GetMcpTools() []tools.McpToolList {
	mcpTools := []tools.McpToolList{}
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        SendMessage,
		ToolDescription: "Send messages to Slack channels via Web API",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", SendMessage),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        ReadMessages,
		ToolDescription: "Read message history from Slack channels",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", ReadMessages),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        ListChannels,
		ToolDescription: "List all channels in Slack workspace",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", ListChannels),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        ListUsers,
		ToolDescription: "List all users in Slack workspace",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", ListUsers),
	})
	return mcpTools
}

func (slackTool *SlackTool) GetSpec() tools.Tooling {
	return slackTool
}

func (slackTool *SlackTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &slackTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (slackTool *SlackTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SlackTool Execute - Start")
	switch actionName {
	case SendMessage:
		return slackTool.SendMessage(ctx, params)
	case ReadMessages:
		return slackTool.ReadMessages(ctx, params)
	case SubscribeWebhooks:
		return slackTool.SubscribeWebhooks(ctx, projectId, tenantId, params)
	case ListChannels:
		return slackTool.ListChannels(ctx, params)
	case ListUsers:
		return slackTool.ListUsers(ctx, params)
	case CreateChannel:
		return slackTool.CreateChannel(ctx, params)
	case InviteToChannel:
		return slackTool.InviteToChannel(ctx, params)
	case JoinChannel:
		return slackTool.JoinChannel(ctx, params)
	case UploadMedia:
		return slackTool.UploadMedia(ctx, params)
	case Login:
		return slackTool.Login(ctx, projectId, tenantId, params, "")
	case GetSsoUrl:
		return slackTool.GetSsoUrl(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}
func (slackTool *SlackTool) getAccessToken(ctx context.Context, params map[string]interface{}) (token string) {
	token = ""
	if tokenType, tokenTypeOk := params["token_type"]; tokenTypeOk {
		if tokenTypeStr, tokenTypeStrOk := tokenType.(string); tokenTypeStrOk {
			switch tokenTypeStr {
			case "bot":
				logs.WithContext(ctx).Info("Using bot token")
				token = slackTool.SlackAccount.BotAccessToken
			case "user":
				logs.WithContext(ctx).Info("Using user token")
				token = slackTool.SlackAccount.AuthedUserAccessToken
			}
		}
	} else {
		//if token_type is not provided, use bot token by default
		logs.WithContext(ctx).Info("Using bot token by default")
		token = slackTool.SlackAccount.BotAccessToken
	}
	return token
}
func (slackTool *SlackTool) GetSsoUrl(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetSsoUrl Execute - Start")
	if slackTool.AuthName == "" {
		err = errors.New("auth name is required")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", slackTool.AuthName, "/getssourl")
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
func (slackTool *SlackTool) Login(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, renewStr string) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Login Execute - Start")
	if slackTool.AuthName == "" {
		err = errors.New("auth name is required")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", slackTool.AuthName, "/idptoken", renewStr)
	logs.WithContext(ctx).Info(fmt.Sprint("url: ", url))
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	var slackTokens SlackTokens
	resBytes, _ := json.Marshal(res)
	err = json.Unmarshal(resBytes, &slackTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	if !slackTokens.Ok {
		err = logs.Err(ctx, errors.New(slackTokens.Error), "")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	err = slackTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprintf("%s_authed_user_access_token", slackTool.ToolName), slackTokens.AuthedUser.AccessToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = slackTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprintf("%s_authed_user_id", slackTool.ToolName), slackTokens.AuthedUser.Id)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = slackTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprintf("%s_bot_access_token", slackTool.ToolName), slackTokens.AccessToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = slackTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprintf("%s_bot_user_id", slackTool.ToolName), slackTokens.BotUserId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	//slackTool.EmailAccount.TokenExpirationDateTime = time.Now().UTC().Add(time.Duration(msTokens.ExpiresIn) * time.Second).Format(time.RFC3339)
	persistStore = true

	toolResult = make(map[string]interface{})
	toolResult["login_status"] = "success"
	return toolResult, persistStore, nil
}

func (slackTool *SlackTool) SendMessage(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SendMessage Execute - Start")

	messagePayload, messagePayloadOk := params["message_payload"]
	if !messagePayloadOk {
		err = errors.New("message_payload parameter is required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	var messagePayloadStruct SlackMessagePayload
	messagePayloadBytes, err := json.Marshal(messagePayload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = json.Unmarshal(messagePayloadBytes, &messagePayloadStruct)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = utils.ValidateStruct(ctx, messagePayloadStruct, "")
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	logs.WithContext(ctx).Info("Sending Slack message")

	url := fmt.Sprintf("%s/chat.postMessage", SLACK_BASE_URL)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", slackTool.getAccessToken(ctx, params)))
	headers.Set("Content-Type", "application/json")

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, messagePayloadStruct)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, resMapOk := res.(map[string]interface{}); resMapOk {
		toolResult["response"] = resMap
		if ok, okExists := resMap["ok"]; okExists && ok.(bool) {
			toolResult["status"] = "sent"
			if ts, tsExists := resMap["ts"]; tsExists {
				toolResult["message_ts"] = ts
			}
			if channel, channelExists := resMap["channel"]; channelExists {
				toolResult["channel"] = channel
			}
		} else {
			toolResult["status"] = "failed"
			if errorMsg, errorExists := resMap["error"]; errorExists {
				toolResult["error"] = errorMsg
			}
		}
	} else {
		toolResult["response"] = res
		toolResult["status"] = "sent"
	}

	return toolResult, false, nil
}

func (slackTool *SlackTool) ReadMessages(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ReadMessages Execute - Start")

	// Validate required channel parameter
	channel, channelOk := params["channel"]
	if !channelOk || channel == "" {
		err = errors.New("channel parameter is required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	url := fmt.Sprintf("%s/conversations.history", SLACK_BASE_URL)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", slackTool.getAccessToken(ctx, params)))
	headers.Set("Content-Type", "application/json")

	queryParams := map[string]string{
		"channel": fmt.Sprintf("%v", channel),
	}

	// Optional parameters
	if limit, limitOk := params["limit"]; limitOk {
		queryParams["limit"] = fmt.Sprintf("%v", limit)
	}
	if latest, latestOk := params["latest"]; latestOk {
		queryParams["latest"] = fmt.Sprintf("%v", latest)
	}
	if oldest, oldestOk := params["oldest"]; oldestOk {
		queryParams["oldest"] = fmt.Sprintf("%v", oldest)
	}
	if inclusive, inclusiveOk := params["inclusive"]; inclusiveOk {
		queryParams["inclusive"] = fmt.Sprintf("%v", inclusive)
	}
	if cursor, cursorOk := params["cursor"]; cursorOk {
		queryParams["cursor"] = fmt.Sprintf("%v", cursor)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	// Check if we got a "not_in_channel" error
	if resMap, resMapOk := res.(map[string]interface{}); resMapOk {
		if ok, okExists := resMap["ok"]; okExists && !ok.(bool) {
			if errorMsg, errorExists := resMap["error"]; errorExists && errorMsg == "not_in_channel" {
				logs.WithContext(ctx).Info("Not in channel, attempting to join and retry")

				// Try to join the channel
				joinResult, _, joinErr := slackTool.JoinChannel(ctx, params)
				if joinErr != nil {
					logs.WithContext(ctx).Error(fmt.Sprintf("Failed to join channel: %v", joinErr))
					return nil, false, fmt.Errorf("failed to join channel: %v", joinErr)
				}

				// Check if join was successful
				if joinResMap, joinResMapOk := joinResult["result"].(map[string]interface{}); joinResMapOk {
					if joinOk, joinOkExists := joinResMap["ok"]; joinOkExists && !joinOk.(bool) {
						logs.WithContext(ctx).Error("Failed to join channel")
						return nil, false, errors.New("failed to join channel")
					}
				}

				// Retry reading messages
				logs.WithContext(ctx).Info("Retrying to read messages after joining channel")
				res, _, _, _, err = utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return nil, false, err
				}
			}
		}
	}

	toolResult = make(map[string]interface{})
	toolResult["messages"] = res

	return toolResult, false, nil
}

func (slackTool *SlackTool) JoinChannel(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("JoinChannel Execute - Start")

	// Validate required channel parameter
	channel, channelOk := params["channel"]
	if !channelOk || channel == "" {
		err = errors.New("channel parameter is required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	url := fmt.Sprintf("%s/conversations.join", SLACK_BASE_URL)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", slackTool.getAccessToken(ctx, params)))
	headers.Set("Content-Type", "application/json")

	payload := map[string]interface{}{
		"channel": channel,
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["result"] = res

	return toolResult, false, nil
}

func (slackTool *SlackTool) SubscribeWebhooks(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SubscribeWebhooks Execute - Start")

	webhookUrl := slackTool.GetToolCbUrl(projectId, tenantId)
	logs.WithContext(ctx).Info(fmt.Sprintf("Webhook URL: %s", webhookUrl))

	toolResult = make(map[string]interface{})
	toolResult["webhook_url"] = webhookUrl
	//toolResult["verification_token"] = slackTool.SlackAccount.WebhookVerifyToken
	toolResult["status"] = "configured"
	toolResult["instructions"] = "Configure this webhook URL in your Slack app's Event Subscriptions with the provided verification token"

	return toolResult, false, nil
}

func (slackTool *SlackTool) ListChannels(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ListChannels Execute - Start")

	url := fmt.Sprintf("%s/conversations.list", SLACK_BASE_URL)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", slackTool.getAccessToken(ctx, params)))
	headers.Set("Content-Type", "application/json")
	queryParams := map[string]string{}
	if limit, limitOk := params["limit"]; limitOk {
		queryParams["limit"] = fmt.Sprintf("%v", limit)
	}
	if types, typesOk := params["types"]; typesOk {
		queryParams["types"] = fmt.Sprintf("%v", types)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["channels"] = res

	return toolResult, false, nil
}

func (slackTool *SlackTool) ListUsers(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ListUsers Execute - Start")

	url := fmt.Sprintf("%s/users.list", SLACK_BASE_URL)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", slackTool.getAccessToken(ctx, params)))
	headers.Set("Content-Type", "application/json")

	queryParams := map[string]string{}
	if limit, limitOk := params["limit"]; limitOk {
		queryParams["limit"] = fmt.Sprintf("%v", limit)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["users"] = res

	return toolResult, false, nil
}

func (slackTool *SlackTool) CreateChannel(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CreateChannel Execute - Start")

	channelPayload, channelPayloadOk := params["channel_payload"]
	if !channelPayloadOk {
		err = errors.New("channel_payload parameter is required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	url := fmt.Sprintf("%s/conversations.create", SLACK_BASE_URL)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", slackTool.getAccessToken(ctx, params)))
	headers.Set("Content-Type", "application/json")

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, channelPayload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["result"] = res

	return toolResult, false, nil
}

func (slackTool *SlackTool) InviteToChannel(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("InviteToChannel Execute - Start")

	invitePayload, invitePayloadOk := params["invite_payload"]
	if !invitePayloadOk {
		err = errors.New("invite_payload parameter is required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	url := fmt.Sprintf("%s/conversations.invite", SLACK_BASE_URL)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", slackTool.getAccessToken(ctx, params)))
	headers.Set("Content-Type", "application/json")

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, invitePayload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["result"] = res

	return toolResult, false, nil
}

func (slackTool *SlackTool) UploadMedia(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("UploadMedia Execute - Start")

	err = errors.New("UploadMedia not implemented yet - requires multipart form upload")
	logs.WithContext(ctx).Error(err.Error())
	return nil, false, err
}

func (slackTool *SlackTool) GetToolCallback() tools.ToolCallback {
	return tools.ToolCallback{
		ResponseContentType: "application/json",
	}
}

func (slackTool *SlackTool) Callback(ctx context.Context, projectId string, tenantId string, actionName string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Callback Execute - Start")
	// This callback handles:
	// 1. URL verification challenges from Slack Events API
	// 2. Message events (message.channels, message.groups, message.im, message.mpim)
	// 3. App mention events (app_mention)
	// 4. Reaction events (reaction_added, reaction_removed)
	// 5. File events (file_created, file_deleted, file_shared)
	// 6. User/team events (team_join, user_change)
	// All events are stored in database and forwarded to eru-functions for processing

	// Handle Slack URL verification challenge
	if challenge, challengeOk := body["challenge"]; challengeOk {
		if challengeType, typeOk := body["type"]; typeOk && challengeType == "url_verification" {
			logs.WithContext(ctx).Info("Slack URL verification challenge received")
			return challenge, false, nil
		}
	}

	// Handle event callback
	if eventType, typeOk := body["type"]; typeOk && eventType == "event_callback" {
		go func() {
			bgCtx := context.Background()
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
			insertQueryFuncAsync.Query = slackTool.ToolDb.GetDbQuery(bgCtx, INSERT_FUNC_ASYNC_SLACK)
			insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, projectId, tenantId, string(bodyBytes), string(paramBytes))
			insertQueryFuncAsync.Rank = 1
			insertQueries = append(insertQueries, &insertQueryFuncAsync)

			_, insertOutputErr := utils.ExecuteDbSave(bgCtx, slackTool.ToolDb.GetConn(), insertQueries)
			if insertOutputErr != nil {
				logs.WithContext(bgCtx).Error(insertOutputErr.Error())
				return
			}

			var eventPayload SlackEventPayload
			err = json.Unmarshal(bodyBytes, &eventPayload)
			if err != nil {
				logs.WithContext(bgCtx).Error(err.Error())
				return
			}

			// Process different event types
			if eventPayload.Event.Type != "" {
				logs.WithContext(bgCtx).Info(fmt.Sprintf("Received Slack event: %s in channel: %s", eventPayload.Event.Type, eventPayload.Event.Channel))

				// Structure event data based on type for consistent processing
				eventDetails := map[string]interface{}{
					"event_type":   eventPayload.Event.Type,
					"event_ts":     eventPayload.Event.EventTs,
					"user":         eventPayload.Event.User,
					"channel":      eventPayload.Event.Channel,
					"channel_type": eventPayload.Event.ChannelType,
					"team_id":      eventPayload.TeamId,
					"api_app_id":   eventPayload.ApiAppId,
					"event_id":     eventPayload.EventId,
					"event_time":   eventPayload.EventTime,
					"tenant_id":    tenantId,
					"project_id":   projectId,
				}

				// Add event-specific data based on event type
				switch eventPayload.Event.Type {
				case "message":
					eventDetails["message"] = map[string]interface{}{
						"text":          eventPayload.Event.Text,
						"ts":            eventPayload.Event.Ts,
						"client_msg_id": eventPayload.Event.ClientMsgId,
						"thread_ts":     eventPayload.Event.Thread_ts,
						"blocks":        eventPayload.Event.Blocks,
					}
					logs.WithContext(bgCtx).Info(fmt.Sprintf("Message from user %s: %s", eventPayload.Event.User, eventPayload.Event.Text))

				case "app_mention":
					eventDetails["mention"] = map[string]interface{}{
						"text":      eventPayload.Event.Text,
						"ts":        eventPayload.Event.Ts,
						"thread_ts": eventPayload.Event.Thread_ts,
						"blocks":    eventPayload.Event.Blocks,
					}
					logs.WithContext(bgCtx).Info(fmt.Sprintf("App mentioned by user %s: %s", eventPayload.Event.User, eventPayload.Event.Text))

				case "reaction_added", "reaction_removed":
					eventDetails["reaction"] = map[string]interface{}{
						"reaction":     eventPayload.Event.Reaction,
						"item_type":    eventPayload.Event.Item.Type,
						"item_channel": eventPayload.Event.Item.Channel,
						"item_ts":      eventPayload.Event.Item.Ts,
					}
					logs.WithContext(bgCtx).Info(fmt.Sprintf("Reaction %s %s by user %s", eventPayload.Event.Reaction, eventPayload.Event.Type, eventPayload.Event.User))

				case "file_created", "file_deleted", "file_shared":
					eventDetails["file"] = map[string]interface{}{
						"file_id":   eventPayload.Event.FileId,
						"file_name": eventPayload.Event.File.Name,
						"file_type": eventPayload.Event.File.Id,
					}
					logs.WithContext(bgCtx).Info(fmt.Sprintf("File event %s by user %s", eventPayload.Event.Type, eventPayload.Event.User))

				case "team_join":
					eventDetails["new_user"] = map[string]interface{}{
						"user_id": eventPayload.Event.User,
					}
					logs.WithContext(bgCtx).Info(fmt.Sprintf("New user joined team: %s", eventPayload.Event.User))

				default:
					// For other event types, include the full event payload
					eventDetails["raw_event"] = eventPayload.Event
					logs.WithContext(bgCtx).Info(fmt.Sprintf("Other event type: %s", eventPayload.Event.Type))
				}

				hookBody := map[string]interface{}{
					"type":       "slack_event",
					"event":      eventDetails,
					"tenant_id":  tenantId,
					"event_time": eventPayload.EventTime,
				}

				hookResult, err := slackTool.ExecuteCallbackHook(bgCtx, projectId, tenantId, hookBody, params)
				if err != nil {
					logs.WithContext(bgCtx).Error(err.Error())
					return
				}
				logs.WithContext(bgCtx).Info(fmt.Sprint("Slack event callback result: ", hookResult))
			}
		}()
	}

	return "OK", false, nil
}

func (slackTool *SlackTool) GetToolCbUrl(projectId string, tenantId string) string {
	return fmt.Sprint(slackTool.CallbackBaseUrl, "/", projectId, "/", tenantId, "/callback/tool/", slackTool.ToolName)
}

func (slackTool *SlackTool) SetPrivateAttributes(ctx context.Context, realTool tools.Tooling) (err error) {
	slackTool.SlackAccount.AuthedUserAccessToken = fmt.Sprintf("$SECRET_%s_authed_user_access_token", slackTool.ToolName)
	slackTool.SlackAccount.AuthedUserId = fmt.Sprintf("$SECRET_%s_authed_user_id", slackTool.ToolName)
	slackTool.SlackAccount.BotAccessToken = fmt.Sprintf("$SECRET_%s_bot_access_token", slackTool.ToolName)
	slackTool.SlackAccount.BotUserId = fmt.Sprintf("$SECRET_%s_bot_user_id", slackTool.ToolName)

	return nil
}

func (slackTool *SlackTool) GetBytes(ctx context.Context) ([]byte, error) {
	slackToolWithToken := slackToolWithToken{
		Tool: slackTool.Tool,
		SlackAccount: slackAccountWithToken{
			TeamId:                slackTool.SlackAccount.TeamId,
			BotUserId:             slackTool.SlackAccount.BotUserId,
			AuthedUserAccessToken: slackTool.SlackAccount.AuthedUserAccessToken,
			BotAccessToken:        slackTool.SlackAccount.BotAccessToken,
			AuthedUserId:          slackTool.SlackAccount.AuthedUserId,
			TeamName:              slackTool.SlackAccount.TeamName,
			Enterprise:            slackTool.SlackAccount.Enterprise,
			IsEnterpriseInstall:   slackTool.SlackAccount.IsEnterpriseInstall,
			AppId:                 slackTool.SlackAccount.AppId,
		},
		AuthName: slackTool.AuthName,
	}

	toolJson, err := json.Marshal(slackToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (slackTool *SlackTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	slackToolWithToken := slackToolWithToken{}
	err := json.Unmarshal(toolObjJson, &slackToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}

	slackTool = &SlackTool{
		Tool: slackToolWithToken.Tool,
		SlackAccount: SlackAccount{
			TeamId:                slackToolWithToken.SlackAccount.TeamId,
			BotUserId:             slackToolWithToken.SlackAccount.BotUserId,
			AuthedUserAccessToken: slackToolWithToken.SlackAccount.AuthedUserAccessToken,
			BotAccessToken:        slackToolWithToken.SlackAccount.BotAccessToken,
			AuthedUserId:          slackToolWithToken.SlackAccount.AuthedUserId,
			TeamName:              slackToolWithToken.SlackAccount.TeamName,
			Enterprise:            slackToolWithToken.SlackAccount.Enterprise,
			IsEnterpriseInstall:   slackToolWithToken.SlackAccount.IsEnterpriseInstall,
			AppId:                 slackToolWithToken.SlackAccount.AppId,
		},
		AuthName: slackToolWithToken.AuthName,
	}
	return slackTool, nil
}
