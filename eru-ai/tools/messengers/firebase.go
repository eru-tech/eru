package messengers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
	"google.golang.org/api/option"
)

const (
	SaveFirebaseToken = "save_firebase_token"
	SendNotification  = "send_notification"
)

const (
	UPSERT_FIREBASE_TOKEN  = "insert into eruai_firebase_tokens (project_id, tenant_id, user_id, device_id, firebase_token, created_at, updated_at) values ($1, $2, $3, $4, $5, now(), now()) on conflict (project_id, tenant_id, user_id, device_id) do update set firebase_token = excluded.firebase_token, updated_at = now()"
	SELECT_FIREBASE_TOKENS = "select device_id, firebase_token from eruai_firebase_tokens where project_id = $1 and tenant_id = $2 and user_id = $3"
	DELETE_FIREBASE_TOKEN  = "delete from eruai_firebase_tokens where project_id = $1 and tenant_id = $2 and user_id = $3 and device_id = $4"
)

type FirebaseAccount struct {
	ProjectId      string `json:"project_id" eru:"required" desc:"Firebase project id"`
	ServiceAccount string `json:"service_account" eru:"required" desc:"Firebase service account"`
}

type FirebaseTool struct {
	tools.Tool
	FirebaseAccount FirebaseAccount `json:"firebase_account"`
}

type FirebaseSaveTokenParams struct {
	UserId        string `json:"user_id" eru:"required" desc:"user id owning the device"`
	DeviceId      string `json:"device_id" eru:"required" desc:"device id"`
	FirebaseToken string `json:"firebase_token" eru:"required" desc:"FCM registration token for the device"`
}

type FirebaseSendNotificationParams struct {
	UserId string            `json:"user_id" eru:"required" desc:"target user id"`
	Title  string            `json:"title" eru:"required" desc:"notification title"`
	Body   string            `json:"body" eru:"required" desc:"notification body"`
	Data   map[string]string `json:"data" desc:"optional data payload"`
}

var firebaseToolActions = []tools.ToolAction{
	{
		ActionName:   SaveFirebaseToken,
		Description:  "Upsert a Firebase device token for a user",
		SystemPrompt: "Upsert a Firebase device token for a user",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(FirebaseSaveTokenParams{}), []string{})
		},
	},
	{
		ActionName:   SendNotification,
		Description:  "Send a Firebase push notification to all devices of a user",
		SystemPrompt: "Send a Firebase push notification to all devices of a user",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(FirebaseSendNotificationParams{}), []string{})
		},
	},
}

func (f *FirebaseTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(firebaseToolActions))
	for i, a := range firebaseToolActions {
		infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
	}
	return infos
}

func (f *FirebaseTool) GetActions() []tools.ToolAction {
	return firebaseToolActions
}

func (f *FirebaseTool) GetSpec() tools.Tooling {
	return f
}

func (f *FirebaseTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	if err := json.Unmarshal(*rj, &f); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (f *FirebaseTool) SetToolAction(actionName string) {
	for _, a := range firebaseToolActions {
		if a.ActionName == actionName {
			f.ToolAction = a
			return
		}
	}
	f.ToolAction = tools.ToolAction{}
}

func (f *FirebaseTool) GetBytes(ctx context.Context) ([]byte, error) {
	b, err := json.Marshal(f)
	if err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return b, nil
}

func (f *FirebaseTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	if err := json.Unmarshal(toolObjJson, &f); err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return &FirebaseTool{
		Tool: f.Tool,
		FirebaseAccount: FirebaseAccount{
			ProjectId:      f.FirebaseAccount.ProjectId,
			ServiceAccount: f.FirebaseAccount.ServiceAccount,
		},
	}, nil
}

func (f *FirebaseTool) GetAttribute(ctx context.Context, attributeName string) (interface{}, error) {
	switch attributeName {
	case "tool_name":
		return f.ToolName, nil
	case "tool_type":
		return f.ToolType, nil
	case "system_prompt":
		return f.SystemPrompt, nil
	case "output_schema":
		return f.OutputSchema, nil
	case "parameters":
		return f.Parameters, nil
	case "description":
		return f.Description, nil
	case "project_id":
		return f.FirebaseAccount.ProjectId, nil
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}

func (f *FirebaseTool) SetAttribute(ctx context.Context, attributeName string, attributeValue interface{}) error {
	switch attributeName {
	case "tool_name":
		f.ToolName = attributeValue.(string)
	case "tool_type":
		f.ToolType = attributeValue.(string)
	case "system_prompt":
		f.SystemPrompt = attributeValue.(string)
	case "output_schema":
		f.OutputSchema = attributeValue.(eru_models.JSONSchema)
	case "parameters":
		f.Parameters = attributeValue.(eru_models.JSONSchema)
	case "description":
		f.Description = attributeValue.(string)
	case "project_id":
		f.FirebaseAccount.ProjectId = attributeValue.(string)
	case "service_account":
		f.FirebaseAccount.ServiceAccount = attributeValue.(string)
	default:
		err := errors.New("attribute not found")
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (f *FirebaseTool) unmarshalParams(ctx context.Context, params map[string]interface{}, target interface{}) error {
	b, err := json.Marshal(params)
	if err != nil {
		return logs.Err(ctx, err, "")
	}
	if err := json.Unmarshal(b, target); err != nil {
		return logs.Err(ctx, err, "")
	}
	return nil
}

func (f *FirebaseTool) getMessagingClient(ctx context.Context) (*messaging.Client, error) {
	if f.FirebaseAccount.ServiceAccount == "" {
		return nil, errors.New("service_account is not configured")
	}
	if f.FirebaseAccount.ProjectId == "" {
		return nil, errors.New("project_id is not configured")
	}
	sa, err := base64.StdEncoding.DecodeString(f.FirebaseAccount.ServiceAccount)
	if err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	opt := option.WithCredentialsJSON(sa)
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: f.FirebaseAccount.ProjectId}, opt)
	if err != nil {
		return nil, logs.Err(ctx, err, "")
	}
	return app.Messaging(ctx)
}

func (f *FirebaseTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("FirebaseTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case SaveFirebaseToken:
		toolResult, toolRequest, persistStore, err = f.SaveFirebaseToken(ctx, projectId, tenantId, params)
	case SendNotification:
		toolResult, toolRequest, persistStore, err = f.SendNotification(ctx, projectId, tenantId, params)
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
			logs.WithContext(ctx).Error("erufuncbaseurl not found in context")
			return
		}
		efurlString, ok := efurl.(string)
		if !ok {
			logs.WithContext(ctx).Error("erufuncbaseurl is not a string")
			return
		}
		bgCtx = context.WithValue(bgCtx, tools.EruFuncBaseUrlKey, efurlString)

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
		hookResult, hookErr := f.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if hookErr != nil {
			logs.WithContext(bgCtx).Error(hookErr.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (f *FirebaseTool) SaveFirebaseToken(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, interface{}, bool, error) {
	logs.WithContext(ctx).Debug("FirebaseTool SaveFirebaseToken - Start")
	p := FirebaseSaveTokenParams{}
	if err := f.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if err := utils.ValidateStruct(ctx, p, ""); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	if f.ToolDb == nil {
		err := errors.New("tool db not configured")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	q := models.Queries{}
	q.Query = f.ToolDb.GetDbQuery(ctx, UPSERT_FIREBASE_TOKEN)
	q.Vals = append(q.Vals, projectId, tenantId, p.UserId, p.DeviceId, p.FirebaseToken)
	q.Rank = 1
	if _, err := utils.ExecuteDbSave(ctx, f.ToolDb.GetConn(), []*models.Queries{&q}); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	toolResult := map[string]interface{}{
		"save_status": "success",
		"user_id":     p.UserId,
		"device_id":   p.DeviceId,
	}
	return toolResult, map[string]interface{}{"body": p}, false, nil
}

func (f *FirebaseTool) SendNotification(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (map[string]interface{}, interface{}, bool, error) {
	logs.WithContext(ctx).Debug("FirebaseTool SendNotification - Start")
	p := FirebaseSendNotificationParams{}
	if err := f.unmarshalParams(ctx, params, &p); err != nil {
		return nil, nil, false, err
	}
	if err := utils.ValidateStruct(ctx, p, ""); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	if f.ToolDb == nil {
		err := errors.New("tool db not configured")
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	selectQ := models.Queries{}
	selectQ.Query = f.ToolDb.GetDbQuery(ctx, SELECT_FIREBASE_TOKENS)
	selectQ.Vals = append(selectQ.Vals, projectId, tenantId, p.UserId)
	rows, err := utils.ExecuteDbFetch(ctx, f.ToolDb.GetConn(), selectQ)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}
	if len(rows) == 0 {
		return map[string]interface{}{
			"sent":    0,
			"failed":  0,
			"deleted": 0,
			"message": "no devices registered for user",
		}, map[string]interface{}{"body": p}, false, nil
	}

	client, err := f.getMessagingClient(ctx)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	sent := 0
	failed := 0
	deleted := 0
	var failures []map[string]interface{}
	var deleteQueries []*models.Queries
	rank := 1
	for _, row := range rows {
		deviceId, _ := row["device_id"].(string)
		token, _ := row["firebase_token"].(string)
		if token == "" {
			continue
		}
		msg := &messaging.Message{
			Token: token,
			Notification: &messaging.Notification{
				Title: p.Title,
				Body:  p.Body,
			},
			Data: p.Data,
		}
		msgId, sendErr := client.Send(ctx, msg)
		if sendErr != nil {
			failed++
			failures = append(failures, map[string]interface{}{
				"device_id": deviceId,
				"error":     sendErr.Error(),
			})
			logs.WithContext(ctx).Error(fmt.Sprintf("fcm send failed for device %s: %s", deviceId, sendErr.Error()))
			if messaging.IsUnregistered(sendErr) || messaging.IsInvalidArgument(sendErr) || messaging.IsSenderIDMismatch(sendErr) {
				dq := models.Queries{}
				dq.Query = f.ToolDb.GetDbQuery(ctx, DELETE_FIREBASE_TOKEN)
				dq.Vals = append(dq.Vals, projectId, tenantId, p.UserId, deviceId)
				dq.Rank = rank
				rank++
				deleteQueries = append(deleteQueries, &dq)
				deleted++
			}
			continue
		}
		logs.WithContext(ctx).Info(fmt.Sprintf("fcm sent device=%s message_id=%s", deviceId, msgId))
		sent++
	}

	if len(deleteQueries) > 0 {
		if _, delErr := utils.ExecuteDbSave(ctx, f.ToolDb.GetConn(), deleteQueries); delErr != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("failed to delete invalid tokens: %s", delErr.Error()))
		}
	}

	toolResult := map[string]interface{}{
		"sent":     sent,
		"failed":   failed,
		"deleted":  deleted,
		"total":    len(rows),
		"failures": failures,
	}
	return toolResult, map[string]interface{}{"body": p}, false, nil
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		ToolType:    "FIREBASE",
		Category:    "Communication",
		Description: "Firebase Cloud Messaging tool to register device tokens and send push notifications to a user's devices",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(firebaseToolActions))
			for i, a := range firebaseToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(FirebaseTool{}), []string{}),
	})
}
