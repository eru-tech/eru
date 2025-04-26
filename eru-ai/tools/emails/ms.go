package emails

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
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
	actions = append(actions, "read_email")
	actions = append(actions, "send_email")
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

func (msEmailTool *MsEmailTool) Execute(ctx context.Context, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, err error) {
	logs.WithContext(ctx).Debug("MsEmailTool Execute - Start")
	switch actionName {
	case "read_email":
		return msEmailTool.ReadEmail(ctx, params)
	case "send_email":
		return msEmailTool.SendEmail(ctx, params)
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
	logs.WithContext(ctx).Info(fmt.Sprint(res))

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
