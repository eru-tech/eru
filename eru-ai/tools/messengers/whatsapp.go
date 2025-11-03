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
		err = errors.New("message_type parameter is required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	messagePayload, messagePayloadOk := params["message_payload"]
	if !messagePayloadOk {
		err = errors.New("message_payload parameter is required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("Sending message type: %s", messageType))

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/messages", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", whatsAppTool.WhatsAppAccount.ApiKey)
	headers.Set("Content-Type", "application/json")

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, messagePayload)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
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
		err = errors.New("message_id parameter is required")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s", WHATSAPP_BASE_URL, apiVersion, messageId)
	headers := http.Header{}
	headers.Set("Authorization", whatsAppTool.WhatsAppAccount.ApiKey)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
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
		err = errors.New("type parameter is required (image, video, audio, document)")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	mediaFile, mediaFileOk := params["file"]
	if !mediaFileOk {
		err = errors.New("file parameter is required (base64 encoded content or file URL)")
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/media", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", whatsAppTool.WhatsAppAccount.ApiKey)

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
	logs.WithContext(ctx).Error(err.Error())
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
	headers.Set("Authorization", whatsAppTool.WhatsAppAccount.ApiKey)

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
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
		err = errors.New("business_account_id is required to retrieve message templates")
		logs.WithContext(ctx).Error(err.Error())
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
		logs.WithContext(ctx).Error(err.Error())
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
		insertQueryFuncAsync.Query = whatsAppTool.ToolDb.GetDbQuery(bgCtx, INSERT_FUNC_ASYNC_WHATSAPP)
		insertQueryFuncAsync.Vals = append(insertQueryFuncAsync.Vals, projectId, tenantId, string(bodyBytes), string(paramBytes))
		insertQueryFuncAsync.Rank = 1
		insertQueries = append(insertQueries, &insertQueryFuncAsync)

		_, insertOutputErr := utils.ExecuteDbSave(bgCtx, whatsAppTool.ToolDb.GetConn(), insertQueries)
		if insertOutputErr != nil {
			logs.WithContext(bgCtx).Error(insertOutputErr.Error())
			return
		}

		var webhookPayload WhatsAppWebhookPayload
		err = json.Unmarshal(bodyBytes, &webhookPayload)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
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
										logs.WithContext(bgCtx).Error(err.Error())
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
									logs.WithContext(bgCtx).Error(err.Error())
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
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (whatsAppTool *WhatsAppTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &whatsAppTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return whatsAppTool, nil
}
