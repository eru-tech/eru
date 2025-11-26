package messengers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/gabriel-vasile/mimetype"
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
	actions = append(actions, RetrieveMedia)
	actions = append(actions, DeleteMedia)
	actions = append(actions, GetMediaUrl)
	actions = append(actions, GetBusinessProfile)
	actions = append(actions, GetMessageTemplates)
	actions = append(actions, MarkMessageAsRead)
	actions = append(actions, SendTypingIndicator)
	actions = append(actions, GetThroughput)
	actions = append(actions, CreateGroup)
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
		ToolName:        RetrieveMedia,
		ToolDescription: "Retrieve/download media files from WhatsApp Cloud API",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", RetrieveMedia),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        DeleteMedia,
		ToolDescription: "Delete media files from WhatsApp Cloud API",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", DeleteMedia),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        GetMediaUrl,
		ToolDescription: "Get media URL for downloading from WhatsApp Cloud API",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", GetMediaUrl),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        GetMessageTemplates,
		ToolDescription: "Retrieve all approved message templates",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", GetMessageTemplates),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        MarkMessageAsRead,
		ToolDescription: "Mark incoming WhatsApp messages as read",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", MarkMessageAsRead),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        SendTypingIndicator,
		ToolDescription: "Send typing indicator to show you are preparing a response",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", SendTypingIndicator),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        GetThroughput,
		ToolDescription: "Get throughput information for WhatsApp messaging",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", GetThroughput),
	})
	mcpTools = append(mcpTools, tools.McpToolList{
		ToolName:        CreateGroup,
		ToolDescription: "Create a WhatsApp group",
		ComponentUrl:    fmt.Sprintf("/tools/%s/component.json", CreateGroup),
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
	case RetrieveMedia:
		return whatsAppTool.RetrieveMedia(ctx, params)
	case DeleteMedia:
		return whatsAppTool.DeleteMedia(ctx, params)
	case GetMediaUrl:
		return whatsAppTool.GetMediaUrl(ctx, params)
	case GetBusinessProfile:
		return whatsAppTool.GetBusinessProfile(ctx, params)
	case GetMessageTemplates:
		return whatsAppTool.GetMessageTemplates(ctx, params)
	case MarkMessageAsRead:
		return whatsAppTool.MarkMessageAsRead(ctx, params)
	case SendTypingIndicator:
		return whatsAppTool.SendTypingIndicator(ctx, params)
	case GetThroughput:
		return whatsAppTool.GetThroughput(ctx, params)
	case CreateGroup:
		return whatsAppTool.CreateGroup(ctx, params)
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

	messageSubType, messageSubTypeOk := params["message_sub_type"]
	messageSubTypeStr := ""
	if messageSubTypeOk {
		messageSubTypeStr, _ = messageSubType.(string)
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
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal template message payload: %s", err.Error()), "failed to unmarshal template message payload")
			return nil, false, err
		}
		err = utils.ValidateStruct(ctx, messagePayload, "")
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
			return nil, false, err
		}
		whatsAppMessagePayload.Template = &messagePayload
		if whatsAppMessagePayload.Template.Name == "" || whatsAppMessagePayload.Template.Language.Code == "" {
			err = logs.Err(ctx, fmt.Errorf("incorrect template payload"), fmt.Sprintf("incorrect template payload: %s", err.Error()))
			return nil, false, err
		}
	case "text":
		messagePayload := WhatsAppTextMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal text message payload: %s", err.Error()), "failed to unmarshal text message payload")
			return nil, false, err
		}
		err = utils.ValidateStruct(ctx, messagePayload, "")
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
			return nil, false, err
		}
		whatsAppMessagePayload.Text = &messagePayload
		if whatsAppMessagePayload.Text.Body == "" {
			err = logs.Err(ctx, fmt.Errorf("incorrect text payload"), fmt.Sprintf("incorrect text payload: %s", err.Error()))
			return nil, false, err
		}
	case "reaction":
		messagePayload := WhatsAppReactionMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal reaction message payload: %s", err.Error()), "failed to unmarshal reaction message payload")
			return nil, false, err
		}
		err = utils.ValidateStruct(ctx, messagePayload, "")
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
			return nil, false, err
		}
		whatsAppMessagePayload.Reaction = &messagePayload
		if whatsAppMessagePayload.Reaction.MessageId == "" || whatsAppMessagePayload.Reaction.Emoji == "" {
			err = logs.Err(ctx, fmt.Errorf("incorrect reaction payload"), fmt.Sprintf("incorrect reaction payload: %s", err.Error()))
			return nil, false, err
		}
	case "image":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal image message payload: %s", err.Error()), "failed to unmarshal image message payload")
			return nil, false, err
		}
		err = utils.ValidateStruct(ctx, messagePayload, "")
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
			return nil, false, err
		}
		whatsAppMessagePayload.Image = &messagePayload
		if (whatsAppMessagePayload.Image.Id == "" && whatsAppMessagePayload.Image.Link == "") || (whatsAppMessagePayload.Image.Id != "" && whatsAppMessagePayload.Image.Link != "") {
			err = logs.Err(ctx, fmt.Errorf("either id or link is required in image payload"), fmt.Sprintf("either id or link is required in image payload: %s", err.Error()))
			return nil, false, err
		}
	case "video":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal video message payload: %s", err.Error()), "failed to unmarshal video message payload")
			return nil, false, err
		}
		err = utils.ValidateStruct(ctx, messagePayload, "")
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
			return nil, false, err
		}
		whatsAppMessagePayload.Video = &messagePayload
		if (whatsAppMessagePayload.Video.Id == "" && whatsAppMessagePayload.Video.Link == "") || (whatsAppMessagePayload.Video.Id != "" && whatsAppMessagePayload.Video.Link != "") {
			err = logs.Err(ctx, fmt.Errorf("either id or link is required in video payload"), fmt.Sprintf("either id or link is required in video payload: %s", err.Error()))
			return nil, false, err
		}
	case "audio":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal audio message payload: %s", err.Error()), "failed to unmarshal audio message payload")
			return nil, false, err
		}
		err = utils.ValidateStruct(ctx, messagePayload, "")
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
			return nil, false, err
		}
		whatsAppMessagePayload.Audio = &messagePayload
		if (whatsAppMessagePayload.Audio.Id == "" && whatsAppMessagePayload.Audio.Link == "") || (whatsAppMessagePayload.Audio.Id != "" && whatsAppMessagePayload.Audio.Link != "") {
			err = logs.Err(ctx, fmt.Errorf("either id or link is required in audio payload"), fmt.Sprintf("either id or link is required in audio payload: %s", err.Error()))
			return nil, false, err
		}
	case "document":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal document message payload: %s", err.Error()), "failed to unmarshal document message payload")
			return nil, false, err
		}
		err = utils.ValidateStruct(ctx, messagePayload, "")
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
			return nil, false, err
		}
		whatsAppMessagePayload.Document = &messagePayload
		if (whatsAppMessagePayload.Document.Id == "" && whatsAppMessagePayload.Document.Link == "") || (whatsAppMessagePayload.Document.Id != "" && whatsAppMessagePayload.Document.Link != "") {
			err = logs.Err(ctx, fmt.Errorf("either id or link is required in document payload"), fmt.Sprintf("either id or link is required in document payload: %s", err.Error()))
			return nil, false, err
		}
	case "sticker":
		messagePayload := WhatsAppMediaMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal sticker message payload: %s", err.Error()), "failed to unmarshal sticker message payload")
			return nil, false, err
		}
		whatsAppMessagePayload.Sticker = &messagePayload
		if (whatsAppMessagePayload.Sticker.Id == "" && whatsAppMessagePayload.Sticker.Link == "") || (whatsAppMessagePayload.Sticker.Id != "" && whatsAppMessagePayload.Sticker.Link != "") {
			err = logs.Err(ctx, fmt.Errorf("either id or link is required in sticker payload"), fmt.Sprintf("either id or link is required in sticker payload: %s", err.Error()))
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
				err = logs.Err(ctx, fmt.Errorf("formatted name is required in contact payload"), fmt.Sprintf("formatted name is required in contact payload: %s", err.Error()))
				return nil, false, err
			}
		}
	case "location":
		messagePayload := WhatsAppLocationMessagePayload{}
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal location message payload: %s", err.Error()), "failed to unmarshal location message payload")
			return nil, false, err
		}
		err = utils.ValidateStruct(ctx, messagePayload, "")
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
			return nil, false, err
		}
		whatsAppMessagePayload.Location = &messagePayload

	case "interactive":
		if messageSubTypeStr == "" {
			err = logs.Err(ctx, fmt.Errorf("message_sub_type parameter is required"), "message_sub_type parameter is required")
			return nil, false, err
		}
		var messagePayload WhatsAppInteractiveMessagePayload
		err = json.Unmarshal(messagePayloadParamsBytes, &messagePayload)
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("failed to unmarshal interactive message payload: %s", err.Error()), "failed to unmarshal interactive message payload")
			return nil, false, err
		}
		err = utils.ValidateStruct(ctx, messagePayload, "")
		if err != nil {
			err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
			return nil, false, err
		}
		whatsAppMessagePayload.Interactive = &messagePayload
		switch messageSubTypeStr {
		case "cta_url":
			if whatsAppMessagePayload.Interactive.Action.Name == "" || whatsAppMessagePayload.Interactive.Action.Parameters == nil || whatsAppMessagePayload.Interactive.Action.Parameters.DisplayText == "" || whatsAppMessagePayload.Interactive.Action.Parameters.Url == "" {
				err = logs.Err(ctx, fmt.Errorf("name , display text and url are required in action payload"), "name , display text and url are required in action payload")
				return nil, false, err
			}
		case "list":
			if whatsAppMessagePayload.Interactive.Action.Button == "" {
				err = logs.Err(ctx, fmt.Errorf("button is required in action payload"), "button is required in action payload")
				return nil, false, err
			}
			if len(whatsAppMessagePayload.Interactive.Action.Sections) == 0 {
				err = logs.Err(ctx, fmt.Errorf("sections are required in action payload"), "sections are required in action payload")
				return nil, false, err
			}
			for _, section := range whatsAppMessagePayload.Interactive.Action.Sections {
				if section.Title == "" {
					err = logs.Err(ctx, fmt.Errorf("title is required in section payload"), "title is required in section payload")
					return nil, false, err
				}
				if len(section.Rows) == 0 {
					err = logs.Err(ctx, fmt.Errorf("rows are required in section payload"), "rows are required in section payload")
					return nil, false, err
				}
				for _, row := range section.Rows {
					if row.Id == "" || row.Title == "" || row.Description == "" {
						err = logs.Err(ctx, fmt.Errorf("id, title and description are required in row payload"), "id, title and description are required in row payload")
						return nil, false, err
					}
				}
			}
		case "carousel":
			if len(whatsAppMessagePayload.Interactive.Action.Cards) < 2 || len(whatsAppMessagePayload.Interactive.Action.Cards) > 10 {
				err = logs.Err(ctx, fmt.Errorf("cards must be between 2 and 10"), "cards must be between 2 and 10")
				return nil, false, err
			}
			for _, card := range whatsAppMessagePayload.Interactive.Action.Cards {
				if card.CardIndex < 0 || card.CardIndex > 9 || card.Type == "" {
					err = logs.Err(ctx, fmt.Errorf("card index (0-9) and type are required in card payload"), "card index, type are required in card payload")
					return nil, false, err
				}
				if card.Type == "cta_url" && (card.Body == nil || card.Body.Text == "" || card.Action == nil || card.Action.Name == "" || card.Action.Parameters == nil || card.Action.Parameters.DisplayText == "" || card.Action.Parameters.Url == "") {
					err = logs.Err(ctx, fmt.Errorf("body text, action name, display text and url are required in action payload"), "body text, action name, display text and url are required in action payload")
					return nil, false, err
				}
				if card.Type == "product" && (card.Action == nil || card.Action.ProductRetailerId == "" || card.Action.CatalogId == "") {
					err = logs.Err(ctx, fmt.Errorf("product retailer id, catalog id are required in action payload"), "product retailer id, catalog id are required in action payload")
					return nil, false, err
				}
			}
		case "button":
			if len(whatsAppMessagePayload.Interactive.Action.Buttons) == 0 || len(whatsAppMessagePayload.Interactive.Action.Buttons) > 3 {
				err = logs.Err(ctx, fmt.Errorf("buttons must be between 1 and 3"), "buttons must be between 1 and 3")
				return nil, false, err
			}
			for _, button := range whatsAppMessagePayload.Interactive.Action.Buttons {
				if button.Type == "" || button.Reply == nil || button.Reply.Id == "" || button.Reply.Title == "" {
					err = logs.Err(ctx, fmt.Errorf("type, id and title are required in button payload"), "type, id and title are required in button payload")
					return nil, false, err
				}
			}
		}
	case "location_request_message":
		if whatsAppMessagePayload.Interactive.Action.Name != "send_location" {
			err = logs.Err(ctx, fmt.Errorf("name must be send_location in action payload"), "name must be send_location in action payload")
			return nil, false, err
		}
	case "address_message":
		if whatsAppMessagePayload.Interactive.Action.Name != "address_message" {
			err = logs.Err(ctx, fmt.Errorf("name must be address_message in action payload"), "name must be send_address in action payload")
			return nil, false, err
		}
		if whatsAppMessagePayload.Interactive.Action.Parameters == nil || whatsAppMessagePayload.Interactive.Action.Parameters.Country == "" {
			err = logs.Err(ctx, fmt.Errorf("country is required in parameters payload"), "country is required in parameters payload")
			return nil, false, err
		}
	}
	err = utils.ValidateStruct(ctx, whatsAppMessagePayload, "")
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("incorrect message payload: %s", err.Error()), fmt.Sprintf("incorrect message payload: %s", err.Error()))
		return nil, false, err
	}

	whatsAppMessagePayloadBytes, err := json.Marshal(whatsAppMessagePayload)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to marshal message payload: %s", err.Error()), "failed to marshal message payload")
		return nil, false, err
	}
	logs.WithContext(ctx).Info(fmt.Sprintf("Sending message payload: %s", string(whatsAppMessagePayloadBytes)))

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/messages", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, whatsAppMessagePayload)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to send message: %s", err.Error()), "failed to send message")
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
		err = logs.Err(ctx, fmt.Errorf("message_id parameter is required"), fmt.Sprintf("message_id parameter is required: %s", err.Error()))
		return nil, false, err
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s", WHATSAPP_BASE_URL, apiVersion, messageId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to get message status: %s", err.Error()), "failed to get message status")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["message_status"] = res

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) UploadMedia(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("UploadMedia Execute - Start")

	mimeLimit := uint32(2000)
	mimeLimitParam, mimeLimitParamOk := params["mime_limit"]
	if mimeLimitParamOk {
		mimeLimitInt, ok := mimeLimitParam.(uint32)
		if ok {
			mimeLimit = mimeLimitInt
		}
	}

	mediaFile, mediaFileOk := params["file"]
	if !mediaFileOk {
		err = logs.Err(ctx, fmt.Errorf("file parameter is required (base64 encoded content)"), "file parameter is required (base64 encoded content)")
		return nil, false, err
	}

	mediaFileStr, ok := mediaFile.(string)
	if !ok {
		err = logs.Err(ctx, fmt.Errorf("file must be a base64 encoded string"), "file must be a base64 encoded string")
		return nil, false, err
	}

	mediaFileName, mediaFileNameOk := params["file_name"]
	if !mediaFileNameOk {
		err = logs.Err(ctx, fmt.Errorf("file_name parameter is required"), "file_name parameter is required")
		return nil, false, err
	}
	mediaFileNameStr, ok := mediaFileName.(string)
	if !ok {
		err = logs.Err(ctx, fmt.Errorf("file_name must be a string"), "file_name must be a string")
		return nil, false, err
	}
	fileBytes, err := base64.StdEncoding.DecodeString(mediaFileStr)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to decode base64 file: %s", err.Error()), "failed to decode base64 file")
		return nil, false, err
	}
	mimetype.SetLimit(mimeLimit)
	fMime := mimetype.Detect(fileBytes)
	logs.WithContext(ctx).Info(fmt.Sprintf("File MIME: %s", fMime.String()))
	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/media", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)

	var reqBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&reqBody)

	typeField, err := multipartWriter.CreateFormField("type")
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to create type field: %s", err.Error()), "failed to create type field")
		return nil, false, err
	}
	_, err = typeField.Write([]byte(fMime.String()))
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to write type field: %s", err.Error()), "failed to write type field")
		return nil, false, err
	}

	messagingProductField, err := multipartWriter.CreateFormField("messaging_product")
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to create messaging_product field: %s", err.Error()), "failed to create messaging_product field")
		return nil, false, err
	}
	_, err = messagingProductField.Write([]byte("whatsapp"))
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to write messaging_product field: %s", err.Error()), "failed to write messaging_product field")
		return nil, false, err
	}

	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, mediaFileNameStr))
	fileHeader.Set("Content-Type", fMime.String())

	fileWriter, err := multipartWriter.CreatePart(fileHeader)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to create file field: %s", err.Error()), "failed to create file field")
		return nil, false, err
	}
	_, err = io.Copy(fileWriter, bytes.NewReader(fileBytes))
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to write file: %s", err.Error()), "failed to write file")
		return nil, false, err
	}

	err = multipartWriter.Close()
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to close multipart writer: %s", err.Error()), "failed to close multipart writer")
		return nil, false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &reqBody)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to create request: %s", err.Error()), "failed to create request")
		return nil, false, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to upload media: %s", err.Error()), "failed to upload media")
		return nil, false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to read response: %s", err.Error()), "failed to read response")
		return nil, false, err
	}

	if resp.StatusCode != http.StatusOK {
		err = logs.Err(ctx, fmt.Errorf("media upload failed with status %d: %s", resp.StatusCode, string(body)), "media upload failed")
		return nil, false, err
	}

	var uploadResponse map[string]interface{}
	err = json.Unmarshal(body, &uploadResponse)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to parse response: %s", err.Error()), "failed to parse response")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["response"] = uploadResponse
	if id, ok := uploadResponse["id"]; ok {
		toolResult["media_id"] = id
	}

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) RetrieveMedia(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("RetrieveMedia Execute - Start")

	mediaUrl, urlOk := params["url"]
	if !urlOk {
		err = logs.Err(ctx, fmt.Errorf("url parameter is required"), "url parameter is required")
		return nil, false, err
	}

	mediaUrlStr, ok := mediaUrl.(string)
	if !ok {
		err = logs.Err(ctx, fmt.Errorf("url must be a string"), "url must be a string")
		return nil, false, err
	}

	if mediaUrlStr == "" {
		err = logs.Err(ctx, fmt.Errorf("url cannot be empty"), "url cannot be empty")
		return nil, false, err
	}

	expectedSha256, shaOk := params["sha256"]
	expectedSha256Str := ""
	if shaOk {
		if shaStr, ok := expectedSha256.(string); ok {
			expectedSha256Str = shaStr
		}
	}

	mimeType, mimeOk := params["mime_type"]
	mimeTypeStr := ""
	if mimeOk {
		if mimeStr, ok := mimeType.(string); ok {
			mimeTypeStr = mimeStr
		}
	}

	downloadHeaders := http.Header{}
	downloadHeaders.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))

	downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaUrlStr, nil)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to create download request: %s", err.Error()), "failed to create download request")
		return nil, false, err
	}
	downloadReq.Header = downloadHeaders

	client := &http.Client{}
	downloadResp, err := client.Do(downloadReq)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to download media: %s", err.Error()), "failed to download media")
		return nil, false, err
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		err = logs.Err(ctx, fmt.Errorf("media download failed with status %d", downloadResp.StatusCode), "media download failed")
		return nil, false, err
	}

	fileBytes, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to read downloaded file: %s", err.Error()), "failed to read downloaded file")
		return nil, false, err
	}

	hash := sha256.Sum256(fileBytes)
	calculatedSha256 := hex.EncodeToString(hash[:])

	if shaOk && expectedSha256Str != "" {
		if !strings.EqualFold(calculatedSha256, expectedSha256Str) {
			err = logs.Err(ctx, fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedSha256Str, calculatedSha256), "SHA256 verification failed")
			return nil, false, err
		}
	}

	detectedMime := mimetype.Detect(fileBytes)
	finalMimeType := mimeTypeStr
	if !mimeOk || mimeTypeStr == "" {
		finalMimeType = detectedMime.String()
	}

	fileName := "media"
	ext := detectedMime.Extension()
	if ext != "" {
		fileName = "media" + ext
	}

	fileBase64 := base64.StdEncoding.EncodeToString(fileBytes)

	toolResult = make(map[string]interface{})
	toolResult["file_content"] = fileBase64
	toolResult["file_name"] = fileName
	toolResult["mime_type"] = finalMimeType
	toolResult["sha256"] = calculatedSha256
	toolResult["file_size"] = len(fileBytes)
	toolResult["media_url"] = mediaUrlStr

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) DeleteMedia(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("DeleteMedia Execute - Start")

	mediaId, mediaIdOk := params["media_id"]
	if !mediaIdOk {
		err = logs.Err(ctx, fmt.Errorf("media_id parameter is required"), "media_id parameter is required")
		return nil, false, err
	}

	mediaIdStr, ok := mediaId.(string)
	if !ok {
		err = logs.Err(ctx, fmt.Errorf("media_id must be a string"), "media_id must be a string")
		return nil, false, err
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s", WHATSAPP_BASE_URL, apiVersion, mediaIdStr)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	res, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodDelete, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to delete media: %s", err.Error()), "failed to delete media")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if statusCode == http.StatusOK || statusCode == http.StatusNoContent {
		toolResult["status"] = "deleted"
		toolResult["media_id"] = mediaIdStr
	} else {
		toolResult["response"] = res
		toolResult["status_code"] = statusCode
	}

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) GetMediaUrl(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetMediaUrl Execute - Start")

	mediaId, mediaIdOk := params["media_id"]
	if !mediaIdOk {
		err = logs.Err(ctx, fmt.Errorf("media_id parameter is required"), "media_id parameter is required")
		return nil, false, err
	}

	mediaIdStr, ok := mediaId.(string)
	if !ok {
		err = logs.Err(ctx, fmt.Errorf("media_id must be a string"), "media_id must be a string")
		return nil, false, err
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s", WHATSAPP_BASE_URL, apiVersion, mediaIdStr)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	res, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to get media URL: %s", err.Error()), "failed to get media URL")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if statusCode == http.StatusOK {
		if resMap, ok := res.(map[string]interface{}); ok {
			mediaUrl, urlOk := resMap["url"].(string)
			expectedSha256, shaOk := resMap["sha256"].(string)
			mimeType, mimeOk := resMap["mime_type"].(string)

			if !urlOk || mediaUrl == "" {
				err = logs.Err(ctx, fmt.Errorf("media URL not found in response"), "media URL not found in response")
				return nil, false, err
			}

			retrieveParams := map[string]interface{}{
				"url": mediaUrl,
			}
			if shaOk && expectedSha256 != "" {
				retrieveParams["sha256"] = expectedSha256
			}
			if mimeOk && mimeType != "" {
				retrieveParams["mime_type"] = mimeType
			}

			retrieveResult, _, retrieveErr := whatsAppTool.RetrieveMedia(ctx, retrieveParams)
			if retrieveErr != nil {
				return nil, false, retrieveErr
			}

			toolResult = retrieveResult
		} else {
			toolResult["response"] = res
		}
	} else {
		toolResult["response"] = res
		toolResult["status_code"] = statusCode
	}

	return toolResult, false, nil
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
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, nil)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to get business profile: %s", err.Error()), "failed to get business profile")
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
		err = logs.Err(ctx, fmt.Errorf("business_account_id is required to retrieve message templates"), fmt.Sprintf("business_account_id is required to retrieve message templates: %s", err.Error()))
		return nil, false, err
	}

	url := fmt.Sprintf("%s/%s/%s/message_templates", WHATSAPP_BASE_URL, apiVersion, businessAccountId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
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

	allTemplates := make([]interface{}, 0)
	err = whatsAppTool.fetchTemplatesRecursive(ctx, url, headers, queryParams, &allTemplates)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to get message templates: %s", err.Error()), "failed to get message templates")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["message_templates"] = allTemplates

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) fetchTemplatesRecursive(ctx context.Context, url string, headers http.Header, queryParams map[string]string, allTemplates *[]interface{}) error {
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		return err
	}

	resMap, ok := res.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected response format")
	}

	if data, exists := resMap["data"]; exists {
		if templatesArray, ok := data.([]interface{}); ok {
			*allTemplates = append(*allTemplates, templatesArray...)
		}
	}

	if paging, exists := resMap["paging"].(map[string]interface{}); exists {
		if cursors, exists := paging["cursors"].(map[string]interface{}); exists {
			if after, exists := cursors["after"].(string); exists && after != "" {
				nextQueryParams := make(map[string]string)
				for k, v := range queryParams {
					nextQueryParams[k] = v
				}
				nextQueryParams["after"] = after
				return whatsAppTool.fetchTemplatesRecursive(ctx, url, headers, nextQueryParams, allTemplates)
			}
		}
	}

	return nil
}

func (whatsAppTool *WhatsAppTool) MarkMessageAsRead(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("MarkMessageAsRead Execute - Start")

	messageId, messageIdOk := params["message_id"]
	if !messageIdOk {
		err = logs.Err(ctx, fmt.Errorf("message_id parameter is required"), "message_id parameter is required")
		return nil, false, err
	}

	messageIdStr, ok := messageId.(string)
	if !ok {
		err = logs.Err(ctx, fmt.Errorf("message_id must be a string"), "message_id must be a string")
		return nil, false, err
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/messages", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        messageIdStr,
	}

	res, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to mark message as read: %s", err.Error()), "failed to mark message as read")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if statusCode == http.StatusOK {
		if resMap, ok := res.(map[string]interface{}); ok {
			if success, successOk := resMap["success"].(bool); successOk && success {
				toolResult["status"] = "read"
				toolResult["message_id"] = messageIdStr
			} else {
				toolResult["response"] = resMap
			}
		} else {
			toolResult["response"] = res
		}
	} else {
		toolResult["response"] = res
		toolResult["status_code"] = statusCode
	}

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) SendTypingIndicator(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SendTypingIndicator Execute - Start")

	messageId, messageIdOk := params["message_id"]
	if !messageIdOk {
		err = logs.Err(ctx, fmt.Errorf("message_id parameter is required"), "message_id parameter is required")
		return nil, false, err
	}

	messageIdStr, ok := messageId.(string)
	if !ok {
		err = logs.Err(ctx, fmt.Errorf("message_id must be a string"), "message_id must be a string")
		return nil, false, err
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/messages", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	typingType := "text"
	if indicatorType, typeOk := params["typing_type"]; typeOk {
		if typeStr, ok := indicatorType.(string); ok {
			typingType = typeStr
		}
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        messageIdStr,
		"typing_indicator": map[string]interface{}{
			"type": typingType,
		},
	}

	res, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to send typing indicator: %s", err.Error()), "failed to send typing indicator")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if statusCode == http.StatusOK {
		if resMap, ok := res.(map[string]interface{}); ok {
			if success, successOk := resMap["success"].(bool); successOk && success {
				toolResult["status"] = "typing_indicator_sent"
				toolResult["message_id"] = messageIdStr
			} else {
				toolResult["response"] = resMap
			}
		} else {
			toolResult["response"] = res
		}
	} else {
		toolResult["response"] = res
		toolResult["status_code"] = statusCode
	}

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) GetThroughput(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetThroughput Execute - Start")

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s?fields=throughput", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	queryParams := map[string]string{}

	if startTime, startOk := params["start_time"]; startOk {
		queryParams["start_time"] = fmt.Sprintf("%v", startTime)
	}
	if endTime, endOk := params["end_time"]; endOk {
		queryParams["end_time"] = fmt.Sprintf("%v", endTime)
	}

	res, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to get throughput: %s", err.Error()), "failed to get throughput")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if statusCode == http.StatusOK {
		toolResult["throughput"] = res
	} else {
		toolResult["response"] = res
		toolResult["status_code"] = statusCode
	}

	return toolResult, false, nil
}

func (whatsAppTool *WhatsAppTool) CreateGroup(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("CreateGroup Execute - Start")

	subject, subjectOk := params["subject"]
	if !subjectOk {
		err = logs.Err(ctx, fmt.Errorf("subject parameter is required"), "subject parameter is required")
		return nil, false, err
	}

	subjectStr, ok := subject.(string)
	if !ok {
		err = logs.Err(ctx, fmt.Errorf("subject must be a string"), "subject must be a string")
		return nil, false, err
	}

	if subjectStr == "" {
		err = logs.Err(ctx, fmt.Errorf("subject cannot be empty"), "subject cannot be empty")
		return nil, false, err
	}

	description, descriptionOk := params["description"]
	descriptionStr := ""
	if descriptionOk {
		if descStr, ok := description.(string); ok {
			descriptionStr = descStr
		}
	}

	joinApprovalMode, joinApprovalModeOk := params["join_approval_mode"]
	joinApprovalModeStr := ""
	if joinApprovalModeOk {
		if modeStr, ok := joinApprovalMode.(string); ok {
			if modeStr == "auto_approve " || modeStr == "approval_required " {
				joinApprovalModeStr = modeStr
			} else {
				err = logs.Err(ctx, fmt.Errorf("join_approval_mode must be auto_approve or approval_required"), "join_approval_mode must be auto_approve or approval_required")
				return nil, false, err
			}
		}
	}

	apiVersion := whatsAppTool.WhatsAppAccount.ApiVersion
	if apiVersion == "" {
		apiVersion = "v18.0"
	}

	url := fmt.Sprintf("%s/%s/%s/groups", WHATSAPP_BASE_URL, apiVersion, whatsAppTool.WhatsAppAccount.PhoneNumberId)
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", whatsAppTool.WhatsAppAccount.ApiKey))
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"subject":           subjectStr,
	}
	if joinApprovalModeStr != "" {
		payload["join_approval_mode"] = joinApprovalModeStr
	}

	if descriptionStr != "" {
		payload["description"] = descriptionStr
	}

	res, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, payload)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to create group: %s", err.Error()), "failed to create group")
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if statusCode == http.StatusOK || statusCode == http.StatusCreated {
		if resMap, ok := res.(map[string]interface{}); ok {
			toolResult["response"] = resMap
			if groupId, groupIdOk := resMap["id"]; groupIdOk {
				toolResult["group_id"] = groupId
			}
			toolResult["status"] = "created"
		} else {
			toolResult["response"] = res
			toolResult["status"] = "created"
		}
	} else {
		toolResult["response"] = res
		toolResult["status_code"] = statusCode
	}

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
			err = logs.Err(bgCtx, fmt.Errorf("failed to marshal body: %s", err.Error()), "failed to marshal body")
			return
		}

		paramBytes, err := json.Marshal(params)
		if err != nil {
			err = logs.Err(bgCtx, fmt.Errorf("failed to marshal params: %s", err.Error()), "failed to marshal params")
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
			err = logs.Err(bgCtx, fmt.Errorf("failed to insert query: %s", err.Error()), "failed to insert query")
			return
		}

		var webhookPayload WhatsAppWebhookPayload
		err = json.Unmarshal(bodyBytes, &webhookPayload)
		if err != nil {
			err = logs.Err(bgCtx, fmt.Errorf("failed to unmarshal webhook payload: %s", err.Error()), "failed to unmarshal webhook payload")
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
										err = logs.Err(bgCtx, fmt.Errorf("failed to execute callback hook: %s", err.Error()), "failed to execute callback hook")
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
									err = logs.Err(bgCtx, fmt.Errorf("failed to execute callback hook: %s", err.Error()), "failed to execute callback hook")
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
		err = logs.Err(ctx, fmt.Errorf("failed to marshal tool: %s", err.Error()), "failed to marshal tool")
		return nil, err
	}
	return toolJson, nil
}

func (whatsAppTool *WhatsAppTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	err := json.Unmarshal(toolObjJson, &whatsAppTool)
	if err != nil {
		err = logs.Err(ctx, fmt.Errorf("failed to unmarshal tool: %s", err.Error()), "failed to unmarshal tool")
		return nil, err
	}
	return whatsAppTool, nil
}
