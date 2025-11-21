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
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	INSERT_FUNC_ASYNC_WHATSAPP = "insert into eruai_cb_whatsapp (project_id, tenant_id, request_body, request_params) values ($1, $2, $3, $4)"
	WHATSAPP_BASE_URL          = "https://graph.facebook.com"
)

type WhatsAppTool struct {
	tools.Tool
	WhatsAppAccount WhatsAppAccount `json:"whatsapp_account"`
}

type WhatsAppAccount struct {
	PhoneNumberId     string `json:"phone_number_id"`
	BusinessAccountId string `json:"business_account_id"`
	ApiKey            string `json:"api_key"`
	WebhookUrl        string `json:"webhook_url"`
	ApiVersion        string `json:"api_version"`
}

func (whatsAppTool *WhatsAppTool) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, SendMessage)
	actions = append(actions, SubscribeWebhooks)
	actions = append(actions, GetMessageStatus)
	actions = append(actions, UploadMedia)
	actions = append(actions, GetBusinessProfile)
	actions = append(actions, GetMessageTemplates)
	actions = append(actions, Callback)
	return actions
}

func (whatsAppTool *WhatsAppTool) GetMcpTools() []tools.McpToolList {
	mcpTools := []tools.McpToolList{}
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        SendMessage,
		ToolDescription: "Send messages via WhatsApp Cloud API",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", SendMessage),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        SubscribeWebhooks,
		ToolDescription: "Subscribe to WhatsApp webhook notifications",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", SubscribeWebhooks),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        UploadMedia,
		ToolDescription: "Upload media files to WhatsApp Cloud API",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", UploadMedia),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        GetMessageTemplates,
		ToolDescription: "Retrieve all approved message templates",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", GetMessageTemplates),
	})
	return mcpTools
}

func (whatsAppTool *WhatsAppTool) GetSpec() tools.Tooling {
	return whatsAppTool
}

func (whatsAppTool *WhatsAppTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &whatsAppTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (whatsAppTool *WhatsAppTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("WhatsAppTool Execute - Start")
	switch actionName {
	case SendMessage:
		return whatsAppTool.SendMessage(ctx, params)
	case SubscribeWebhooks:
		return whatsAppTool.SubscribeWebhooks(ctx, projectId, tenantId, params)
	case GetMessageStatus:
		return whatsAppTool.GetMessageStatus(ctx, params)
	case UploadMedia:
		return whatsAppTool.UploadMedia(ctx, params)
	case GetBusinessProfile:
		return whatsAppTool.GetBusinessProfile(ctx, params)
	case GetMessageTemplates:
		return whatsAppTool.GetMessageTemplates(ctx, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (whatsAppTool *WhatsAppTool) SendMessage(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SendMessage Execute - Start")

	messageType, messageTypeOk := params["message_type"]
	if !messageTypeOk {
		err = logs.Err(ctx, errors.New("message_type parameter is required"), "message_type parameter is required")
		return nil, false, err
	}
	messageTypeStr, messageTypeStrOk := messageType.(string)
	if !messageTypeStrOk {
		err = logs.Err(ctx, errors.New("message_type must be a string"), "message_type must be a string")
		return nil, false, err
	}

	to, toOk := params["to"]
	if !toOk {
		err = logs.Err(ctx, errors.New("recipient phone number is required"), "recipient phone number is required")
		return nil, false, err
	}
	toStr, toStrOk := to.(string)
	if !toStrOk {
		err = logs.Err(ctx, errors.New("recipient phone number must be a string"), "recipient phone number must be a string")
		return nil, false, err
	}
	if toStr == "" {
		err = logs.Err(ctx, errors.New("recipient phone number cannot be empty"), "recipient phone number cannot be empty")
		return nil, false, err
	}

	messagePayloadParams, messagePayloadParamsOk := params["message_payload"]
	if !messagePayloadParamsOk {
		err = logs.Err(ctx, errors.New("message_payload parameter is required"), "message_payload parameter is required")
		return nil, false, err
	}

	messagePayloadParamsBytes, err := json.Marshal(messagePayloadParams)
	if err != nil {
		err = logs.Err(ctx, errors.New("failed to marshal message payload"), "failed to marshal message payload")
		return nil, false, err
	}

	whatsAppMessagePayload := WhatsAppMessagePayload{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               toStr,
		Type:             messageTypeStr,
	}
	switch messageTypeStr {
	case "template":
		messagePayload := WhatsAppTemplateMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, errors.New("failed to unmarshal template message payload"), "failed to unmarshal template message payload")
			return nil, false, err
		}
		whatsAppMessagePayload.Template = &messagePayload
		if whatsAppMessagePayload.Template.Name == "" || whatsAppMessagePayload.Template.Language.Code == "" {
			err = logs.Err(ctx, errors.New("incorrect template payload"), "incorrect template payload")
			return nil, false, err
		}
	case "text":
		messagePayload := WhatsAppTextMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, errors.New("failed to unmarshal text message payload"), "failed to unmarshal text message payload")
			return nil, false, err
		}
		whatsAppMessagePayload.Text = &messagePayload
		if whatsAppMessagePayload.Text.Body == "" {
			err = logs.Err(ctx, errors.New("incorrect text payload"), "incorrect text payload")
			return nil, false, err
		}
	case "reaction":
		messagePayload := WhatsAppReactionMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, errors.New("failed to unmarshal reaction message payload"), "failed to unmarshal reaction message payload")
			return nil, false, err
		}
		whatsAppMessagePayload.Reaction = &messagePayload
		if whatsAppMessagePayload.Reaction.MessageId == "" || whatsAppMessagePayload.Reaction.Emoji == "" {
			err = logs.Err(ctx, errors.New("incorrect reaction payload"), "incorrect reaction payload")
			return nil, false, err
		}
	case "image":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, errors.New("failed to unmarshal image message payload"), "failed to unmarshal image message payload")
			return nil, false, err
		}
		whatsAppMessagePayload.Image = &messagePayload
		if (whatsAppMessagePayload.Image.Id == "" && whatsAppMessagePayload.Image.Link == "") || (whatsAppMessagePayload.Image.Id != "" && whatsAppMessagePayload.Image.Link != "") {
			err = logs.Err(ctx, errors.New("either id or link is required in image payload"), "either id or link is required in image payload")
			return nil, false, err
		}
	case "video":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, errors.New("failed to unmarshal video message payload"), "failed to unmarshal video message payload")
			return nil, false, err
		}
		whatsAppMessagePayload.Video = &messagePayload
		if (whatsAppMessagePayload.Video.Id == "" && whatsAppMessagePayload.Video.Link == "") || (whatsAppMessagePayload.Video.Id != "" && whatsAppMessagePayload.Video.Link != "") {
			err = logs.Err(ctx, errors.New("either id or link is required in video payload"), "either id or link is required in video payload")
			return nil, false, err
		}
	case "audio":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, errors.New("failed to unmarshal audio message payload"), "failed to unmarshal audio message payload")
			return nil, false, err
		}
		whatsAppMessagePayload.Audio = &messagePayload
		if (whatsAppMessagePayload.Audio.Id == "" && whatsAppMessagePayload.Audio.Link == "") || (whatsAppMessagePayload.Audio.Id != "" && whatsAppMessagePayload.Audio.Link != "") {
			err = logs.Err(ctx, errors.New("either id or link is required in audio payload"), "either id or link is required in audio payload")
			return nil, false, err
		}
	case "document":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, errors.New("failed to unmarshal document message payload"), "failed to unmarshal document message payload")
			return nil, false, err
		}
		whatsAppMessagePayload.Document = &messagePayload
		if (whatsAppMessagePayload.Document.Id == "" && whatsAppMessagePayload.Document.Link == "") || (whatsAppMessagePayload.Document.Id != "" && whatsAppMessagePayload.Document.Link != "") {
			err = logs.Err(ctx, errors.New("either id or link is required in document payload"), "either id or link is required in document payload")
			return nil, false, err
		}
	case "sticker":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, errors.New("failed to unmarshal sticker message payload"), "failed to unmarshal sticker message payload")
			return nil, false, err
		}
		whatsAppMessagePayload.Sticker = &messagePayload
		if (whatsAppMessagePayload.Sticker.Id == "" && whatsAppMessagePayload.Sticker.Link == "") || (whatsAppMessagePayload.Sticker.Id != "" && whatsAppMessagePayload.Sticker.Link != "") {
			err = logs.Err(ctx, errors.New("either id or link is required in sticker payload"), "either id or link is required in sticker payload")
			return nil, false, err
		}
	case "contacts":
		messagePayload := []WhatsAppContactMessagePayload{}

		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal contact message payload: %s", err.Error()), "failed to unmarshal contact message payload")
			return nil, false, err
		}

		whatsAppMessagePayload.Contacts = messagePayload
		for _, contact := range whatsAppMessagePayload.Contacts {
			if contact.Name.FormattedName == "" {
				err = logs.Err(ctx, errors.New("formatted name is required in contact payload"), "formatted name is required in contact payload")
				return nil, false, err
			}
		}
	}
	err = utils.ValidateStruct(ctx, whatsAppMessagePayload, "")
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
		return nil, false, err
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("Sending message type: %s", messageTypeStr))

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/messages", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, whatsAppMessagePayload)
	if err != nil {
		err = logs.Err(ctx, errors.New("failed to send message"), "failed to send message")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, resMapOk := res.(map[string]interface{}); resMapOk {
		toolResult["response"] = resMap
		toolResult["status"] = "sent"

		if messages, messagesOk := resMap["messages"].([]interface{}); messagesOk && len(messages) > 0 {
			if messageMap, messageMapOk := messages[0].(map[string]interface{}); messageMapOk {
				if messageId, messageIdOk := messageMap["id"]; messageIdOk {
					toolResult["message_id"] = messageId
				}
			}
		}
	} else {
		toolResult["response"] = res
		toolResult["status"] = "sent"
	}

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) SubscribeWebhooks(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SubscribeWebhooks Execute - Start")

	webhookUrl := whatsAppTool.GetToolCbUrl(projectId, tenantId)
	logs.WithContext(ctx).Info(fmt.Sprintf("Webhook URL: %s", webhookUrl))

	toolResult = make(map[string]interface{})
	toolResult["webhook_url"] = webhookUrl
	toolResult["verification_token"] = whatsAppTool.WhatsAppAccount.WebhookUrl
	toolResult["status"] = "configured"
	toolResult["instructions"] = "Configure this webhook URL in your WhatsApp Business API settings with the provided verification token"

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) GetMessageStatus(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetMessageStatus Execute - Start")

	messageId, messageIdOk := params["message_id"]
	if !messageIdOk {
		err = logs.Err(ctx, errors.New("message_id parameter is required"), "message_id parameter is required")
		return nil, false, err
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s", WHATSAPP_BASE_URL, apiVersion, messageId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		err = logs.Err(ctx, errors.New("failed to get message status"), "failed to get message status")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["message_status"] = res

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) UploadMedia(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("UploadMedia Execute - Start")

	mediaType, mediaTypeOk := params["type"]
	if !mediaTypeOk {
		err = logs.Err(ctx, errors.New("type parameter is required (image, video, audio, document)"), "type parameter is required (image, video, audio, document)")
		return nil, false, err
	}

	mediaFile, mediaFileOk := params["file"]
	if !mediaFileOk {
		err = logs.Err(ctx, errors.New("file parameter is required (base64 encoded content or file URL)"), "file parameter is required (base64 encoded content or file URL)")
		return nil, false, err
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/media", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))

	// For now, we'll accept a simplified payload structure
	// In a full implementation, this would handle multipart/form-data uploads
	uploadPayload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"type":              mediaType,
		"file":              mediaFile,
	}

	// Note: This is a simplified implementation. WhatsApp API actually requires multipart/form-data
	// For proper implementation, would need to construct multipart form with file binary data
	err = errors.New("UploadMedia requires multipart/form-data implementation with actual file handling")
	err = logs.Err(ctx, errors.New("UploadMedia requires multipart/form-data implementation with actual file handling"), "UploadMedia requires multipart/form-data implementation with actual file handling")
	logs.WithContext(ctx).Info(fmt.Sprintf("Upload payload structure: %+v", uploadPayload))
	logs.WithContext(ctx).Info(fmt.Sprintf("Use URL: %s", url))

	toolResult = make(map[string]interface{})
	toolResult["error"] = "Media upload requires multipart/form-data implementation"
	toolResult["upload_url"] = url
	toolResult["instructions"] = "Implement multipart form upload with file binary data for actual media upload"

	return toolResult, false, err
}

func (whatsAppTool *WhatsAppTool) GetBusinessProfile(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetBusinessProfile Execute - Start")

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/whatsapp_business_profile", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		err = logs.Err(ctx, errors.New("failed to get business profile"), "failed to get business profile")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["business_profile"] = res

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) GetMessageTemplates(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetMessageTemplates Execute - Start")

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	// WhatsApp Business Account ID is required for templates endpoint
	businessAccountId := whatsAppTool.WhatsAppAccount.BusinessAccountId
	if businessAccountId == "" {
		err = logs.Err(ctx, errors.New("business_account_id is required to retrieve message templates"), "business_account_id is required to retrieve message templates")
		return nil, false, err
	}

	url := fmt.Sprintf("%s/%s/%s/message_templates", WHATSAPP_BASE_URL, apiVersion, businessAccountId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))

	queryParams := map[string]string{}

	// Optional parameters
	if limit, limitOk := params["limit"]; limitOk {
		queryParams["limit"] = fmt.Sprintf("%v", limit)
	}
	if status, statusOk := params["status"]; statusOk {
		queryParams["status"] = fmt.Sprintf("%v", status) // APPROVED, PENDING, REJECTED
	}
	if category, categoryOk := params["category"]; categoryOk {
		queryParams["category"] = fmt.Sprintf("%v", category) // AUTHENTICATION, MARKETING, UTILITY
	}
	if language, languageOk := params["language"]; languageOk {
		queryParams["language"] = fmt.Sprintf("%v", language) // e.g., en_US, es_ES
	}
	if name, nameOk := params["name"]; nameOk {
		queryParams["name"] = fmt.Sprintf("%v", name) // Template name filter
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		err = logs.Err(ctx, errors.New("failed to get message templates"), "failed to get message templates")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["message_templates"] = res

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) GetToolCallback() tools.ToolCallback {
	return tools.ToolCallback{
		ResponseContentType: "application/json",
	}
}

func (whatsAppTool *WhatsAppTool) Callback(ctx context.Context, projectId string, tenantId string, actionName string, body map[string]interface{}, params map[string][]string) (callbackResult interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Callback Execute - Start")
	// This callback handles:
	// 1. Webhook verification challenges from WhatsApp
	// 2. Incoming message notifications
	// 3. Message status updates (sent, delivered, read, failed) for tracking delivery
	// All callbacks are stored in database and forwarded to eru-functions for processing

	hubMode := ""
	hubChallenge := ""
	hubVerifyToken := ""

	if mode, modeOk := params["hub.mode"]; modeOk && len(mode) > 0 {
		hubMode = mode[0]
	}
	if challenge, challengeOk := params["hub.challenge"]; challengeOk && len(challenge) > 0 {
		hubChallenge = challenge[0]
	}
	if verifyToken, verifyTokenOk := params["hub.verify_token"]; verifyTokenOk && len(verifyToken) > 0 {
		hubVerifyToken = verifyToken[0]
	}

	if hubMode == "subscribe" && hubVerifyToken == whatsAppTool.WhatsAppAccount.WebhookUrl {
		logs.WithContext(ctx).Info("Webhook verification successful")
		return hubChallenge, false, nil
	}

	gm := server.GetGlobalGoroutineManager(ctx)
	gm.SafeGoWithRestartBehavior("whatsapp-webhook-callback", func(bgCtx context.Context) {
		if eruFuncBaseUrl, ok := ctx.Value("Erufuncbaseurl").(string); ok {
			bgCtx = context.WithValue(bgCtx, "Erufuncbaseurl", eruFuncBaseUrl)
		}

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			err = logs.Err(bgCtx, errors.New("failed to marshal body"), "failed to marshal body")
			return
		}

		paramBytes, err := json.Marshal(params)
		if err != nil {
			err = logs.Err(bgCtx, errors.New("failed to marshal params"), "failed to marshal params")
			return
		}

		var insertQueries []*models.Queries
		insertQueryFuncAsync := models.Queries{}
		insertQueryFuncAsync.Query = whatsAppTool.ToolDb.GetDbQuery(bgCtx, INSERT_FUNC_ASYNC_WHATSAPP)
		insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, projectId, tenantId, string(bodyBytes), string(paramBytes))
		insertQueryFuncAsync.Rank = 1
		insertQueries = append(insertQueries, &insertQueryFuncAsync)

		_, insertOutputErr := utils.ExecuteDbSave(bgCtx, whatsAppTool.ToolDb.GetConn(), insertQueries)
		if insertOutputErr != nil {
			err = logs.Err(bgCtx, errors.New("failed to insert query"), "failed to insert query")
			return
		}

		var webhookPayload WhatsAppWebhookPayload
		err = json.Unmarshal(bodyBytes, &webhookPayload)
		if err != nil {
			err = logs.Err(bgCtx, errors.New("failed to unmarshal webhook payload"), "failed to unmarshal webhook payload")
			return
		}

		if webhookPayload.Object == "whatsapp_business_account" {
			for _, entry := range webhookPayload.Entry {
				for _, change := range entry.Changes {
					if change.Field == "messages" {
						processMessages := true

						if len(change.Value.Messages) > 0 {
							for _, message := range change.Value.Messages {
								logs.WithContext(bgCtx).Info(fmt.Sprintf("Received message from %s: %s (Type: %s)", message.From, message.Id, message.Type))

								if processMessages {
									// Structure message data for consistent processing
									messageDetails := map[string]interface{}{
										"message_id":      message.Id,
										"from":            message.From,
										"timestamp":       message.Timestamp,
										"type":            message.Type,
										"tenant_id":       tenantId,
										"project_id":      projectId,
										"phone_number_id": change.Value.Metadata.PhoneNumberId,
									}

									// Add message content based on type
									switch message.Type {
									case "text":
										messageDetails["text"] = message.Text.Body
									case "image":
										messageDetails["image"] = map[string]interface{}{
											"id":        message.Image.Id,
											"caption":   message.Image.Caption,
											"mime_type": message.Image.MimeType,
											"sha256":    message.Image.Sha256,
										}
									case "audio":
										messageDetails["audio"] = map[string]interface{}{
											"id":        message.Audio.Id,
											"mime_type": message.Audio.MimeType,
										}
									case "video":
										messageDetails["video"] = map[string]interface{}{
											"id":        message.Video.Id,
											"caption":   message.Video.Caption,
											"filename":  message.Video.Filename,
											"mime_type": message.Video.MimeType,
										}
									case "document":
										messageDetails["document"] = map[string]interface{}{
											"id":        message.Document.Id,
											"caption":   message.Document.Caption,
											"filename":  message.Document.Filename,
											"mime_type": message.Document.MimeType,
										}
									case "location":
										messageDetails["location"] = map[string]interface{}{
											"latitude":  message.Location.Latitude,
											"longitude": message.Location.Longitude,
											"name":      message.Location.Name,
											"address":   message.Location.Address,
										}
									}

									hookBody := map[string]interface{}{
										"type":       "incoming_message",
										"message":    messageDetails,
										"metadata":   change.Value.Metadata,
										"contacts":   change.Value.Contacts,
										"tenant_id":  tenantId,
										"event_time": message.Timestamp,
									}

									hookResult, err := whatsAppTool.ExecuteCallbackHook(bgCtx, projectId, tenantId, hookBody, params)
									if err != nil {
										err = logs.Err(bgCtx, errors.New("failed to execute callback hook"), "failed to execute callback hook")
										return
									}
									logs.WithContext(bgCtx).Info(fmt.Sprint("Message callback result: ", hookResult))
								}
							}
						}

						if len(change.Value.Statuses) > 0 {
							for _, status := range change.Value.Statuses {
								logs.WithContext(bgCtx).Info(fmt.Sprintf("Message status update: %s - %s for recipient %s", status.Id, status.Status, status.RecipientId))

								// Store detailed status information for tracking
								statusDetails := map[string]interface{}{
									"message_id":      status.Id,
									"status":          status.Status, // sent, delivered, read, failed
									"timestamp":       status.Timestamp,
									"recipient_id":    status.RecipientId,
									"tenant_id":       tenantId,
									"project_id":      projectId,
									"phone_number_id": change.Value.Metadata.PhoneNumberId,
								}

								// Add conversation and pricing info if available
								if status.Conversation.Id != "" {
									statusDetails["conversation_id"] = status.Conversation.Id
									statusDetails["conversation_origin"] = status.Conversation.Origin.Type
									if status.Conversation.ExpirationTimestamp != "" {
										statusDetails["conversation_expiration"] = status.Conversation.ExpirationTimestamp
									}
								}

								if status.Pricing.PricingModel != "" {
									statusDetails["pricing_billable"] = status.Pricing.Billable
									statusDetails["pricing_model"] = status.Pricing.PricingModel
									statusDetails["pricing_category"] = status.Pricing.Category
								}

								hookBody := map[string]interface{}{
									"type":       "message_status",
									"status":     statusDetails,
									"metadata":   change.Value.Metadata,
									"tenant_id":  tenantId,
									"event_time": status.Timestamp,
								}

								hookResult, err := whatsAppTool.ExecuteCallbackHook(bgCtx, projectId, tenantId, hookBody, params)
								if err != nil {
									err = logs.Err(bgCtx, errors.New("failed to execute callback hook"), "failed to execute callback hook")
									return
								}
								logs.WithContext(bgCtx).Info(fmt.Sprint("Status callback result: ", hookResult))
							}
						}
					}
				}
			}
		}
	}, server.ContinueOnMaxRetries)

	return "OK", false, nil
}

func (whatsAppTool *WhatsAppTool) GetToolCbUrl(projectId string, tenantId string) string {
	return fmt.Sprint(whatsAppTool.CallbackBaseUrl, "/", projectId, "/", tenantId, "/callback/tool/", whatsAppTool.ToolName)
}

func (whatsAppTool *WhatsAppTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(whatsAppTool)
	if err != nil {
		err = logs.Err(ctx, errors.New("failed to marshal tool"), "failed to marshal tool")
		return nil, err
	}
	return toolJson, nil
}

func (whatsAppTool *WhatsAppTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &whatsAppTool)
	if err != nil {
		err = logs.Err(ctx, err, "failed to unmarshal tool")
		return nil, err
	}
	return whatsAppTool, nil
}
