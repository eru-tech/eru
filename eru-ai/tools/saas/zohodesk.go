package saas

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

type ZohoTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type ZohoAccount struct {
	OrgId                   string `json:"org_id"`
	AccessToken             string `json:"-"`
	RefreshToken            string `json:"-"`
	TokenExpirationDateTime string `json:"token_expiration_date_time"`
}

type zohoAccountWithToken struct {
	OrgId                   string `json:"org_id"`
	AccessToken             string `json:"access_token"`
	RefreshToken            string `json:"refresh_token"`
	TokenExpirationDateTime string `json:"token_expiration_date_time"`
}

const (
	GetTickets       = "get_tickets"
	GetOrganizations = "get_organizations"
	Login            = "login"
	RenewToken       = "renew_token"
	GetSsoUrl        = "get_sso_url"
	StopAutoRenew    = "stop_auto_renew"
)

const (
	DeskBaseUrl = "https://desk.zoho.in/api/v1"
)

type ZohoDeskTool struct {
	tools.Tool
	ZohoAccount ZohoAccount `json:"zoho_account"`
	AuthName    string      `json:"auth_name"`
}

type zohoDeskToolWithToken struct {
	tools.Tool
	ZohoAccount zohoAccountWithToken
	AuthName    string
}

func (zohoDeskTool *ZohoDeskTool) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, GetTickets, GetOrganizations, Login, RenewToken, GetSsoUrl, StopAutoRenew)
	return actions
}

func (zohoDeskTool *ZohoDeskTool) GetSpec() tools.Tooling {
	return zohoDeskTool
}

func (zohoDeskTool *ZohoDeskTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &zohoDeskTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (zohoDeskTool *ZohoDeskTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("ZohoDeskTool Execute - Start")
	switch actionName {
	case GetTickets:
		return zohoDeskTool.GetTickets(ctx, params)
	case GetOrganizations:
		return zohoDeskTool.GetOrganizations(ctx, params)
	case Login:
		return zohoDeskTool.Login(ctx, projectId, tenantId, params, "")
	case RenewToken:
		return zohoDeskTool.RenewToken(ctx, projectId, tenantId, params)
	case GetSsoUrl:
		return zohoDeskTool.GetSsoUrl(ctx, projectId, tenantId, params)
	case StopAutoRenew:
		return zohoDeskTool.StopAutoRenew(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (zohoDeskTool *ZohoDeskTool) GetTickets(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetTickets Execute - Start")

	url := fmt.Sprint(DeskBaseUrl, "/tickets")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprint("Zoho-oauthtoken ", zohoDeskTool.ZohoAccount.AccessToken))
	headers.Set("orgId", zohoDeskTool.ZohoAccount.OrgId)
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		queryParams[k] = fmt.Sprint(v)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["data"] = res
	}

	return toolResult, false, nil
}

func (zohoDeskTool *ZohoDeskTool) GetOrganizations(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetOrganizations Execute - Start")

	url := fmt.Sprint(DeskBaseUrl, "/organizations")
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprint("Zoho-oauthtoken ", zohoDeskTool.ZohoAccount.AccessToken))
	headers.Set("orgId", zohoDeskTool.ZohoAccount.OrgId)
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		queryParams[k] = fmt.Sprint(v)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	if resMap, ok := res.(map[string]interface{}); ok {
		toolResult = resMap
	} else {
		toolResult["data"] = res
	}

	return toolResult, false, nil
}

func (zohoDeskTool *ZohoDeskTool) GetSsoUrl(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetSsoUrl Execute - Start")

	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", zohoDeskTool.AuthName, "/getssourl")
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

func (zohoDeskTool *ZohoDeskTool) RenewToken(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	params["refresh_token"] = zohoDeskTool.ZohoAccount.RefreshToken
	return zohoDeskTool.Login(ctx, projectId, tenantId, params, "/renew")
}

func (zohoDeskTool *ZohoDeskTool) Login(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, renewStr string) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Login Execute - Start")
	if zohoDeskTool.AuthName == "" {
		err = errors.New("auth name is required")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", zohoDeskTool.AuthName, "/idptoken", renewStr)
	logs.WithContext(ctx).Info(fmt.Sprint("url: ", url))
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	var zohoTokens ZohoTokens
	resBytes, _ := json.Marshal(res)
	err = json.Unmarshal(resBytes, &zohoTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = json.Unmarshal(resBytes, &zohoTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = zohoDeskTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(zohoDeskTool.ToolName, "_access_token"), zohoTokens.AccessToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = zohoDeskTool.SaveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(zohoDeskTool.ToolName, "_refresh_token"), zohoTokens.RefreshToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	zohoDeskTool.ZohoAccount.TokenExpirationDateTime = time.Now().UTC().Add(time.Duration(zohoTokens.ExpiresIn) * time.Second).Format(time.RFC3339)
	persistStore = true

	if zohoDeskTool.Tool.Hooks.ARRT != "" {
		hookBody := map[string]interface{}{
			"Vars": map[string]interface{}{
				"Body": map[string]interface{}{
					"tool_name": zohoDeskTool.ToolName,
					"tenant_id": tenantId,
				},
				"OrgBody": map[string]interface{}{
					"tool_name": zohoDeskTool.ToolName,
					"tenant_id": tenantId,
				},
			},
			"ReqVars": map[string]interface{}{},
			"ResVars": map[string]interface{}{},
		}
		hookBodyBytes, err := json.Marshal(hookBody)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, persistStore, err
		}
		jobName := fmt.Sprint(zohoDeskTool.ToolName, "_", zohoDeskTool.Tool.Hooks.ARRT, "_", tenantId)
		err = zohoDeskTool.Scheduler.Unschedule(ctx, "", jobName)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}

		schedulerCommand := fmt.Sprint("CALL schedule_procedure('", zohoDeskTool.Tool.Hooks.ARRT, "','", string(hookBodyBytes), "','", zohoDeskTool.Scheduler.GetSchedulerName(), "')")

		cronStr := utils.GetCronStr(ctx, time.Now().UTC().Add(1*time.Hour))
		jobId, err := zohoDeskTool.Scheduler.Schedule(ctx, jobName, schedulerCommand, cronStr)
		if err != nil {
			return nil, persistStore, err
		}
		logs.WithContext(ctx).Info(fmt.Sprint("jobId: ", jobId))
	}
	toolResult = make(map[string]interface{})
	toolResult["login_status"] = "success"
	return toolResult, persistStore, nil
}

func (zohoDeskTool *ZohoDeskTool) SetPrivateAttributes(ctx context.Context, realTool tools.Tooling) (err error) {
	zohoDeskTool.ZohoAccount.AccessToken = fmt.Sprint("$SECRET_", zohoDeskTool.ToolName, "_access_token")
	zohoDeskTool.ZohoAccount.RefreshToken = fmt.Sprint("$SECRET_", zohoDeskTool.ToolName, "_refresh_token")
	return nil
}

func (zohoDeskTool *ZohoDeskTool) GetBytes(ctx context.Context) ([]byte, error) {

	zohoDeskToolWithToken := zohoDeskToolWithToken{
		Tool: zohoDeskTool.Tool,
		ZohoAccount: zohoAccountWithToken{
			OrgId:                   zohoDeskTool.ZohoAccount.OrgId,
			AccessToken:             zohoDeskTool.ZohoAccount.AccessToken,
			RefreshToken:            zohoDeskTool.ZohoAccount.RefreshToken,
			TokenExpirationDateTime: zohoDeskTool.ZohoAccount.TokenExpirationDateTime,
		},
		AuthName: zohoDeskTool.AuthName,
	}

	toolJson, err := json.Marshal(zohoDeskToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (zohoDeskTool *ZohoDeskTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	zohoDeskToolWithToken := zohoDeskToolWithToken{}
	err := json.Unmarshal(toolObjJson, &zohoDeskToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	zohoDeskTool = &ZohoDeskTool{
		Tool: zohoDeskToolWithToken.Tool,
		ZohoAccount: ZohoAccount{
			OrgId:                   zohoDeskToolWithToken.ZohoAccount.OrgId,
			AccessToken:             zohoDeskToolWithToken.ZohoAccount.AccessToken,
			RefreshToken:            zohoDeskToolWithToken.ZohoAccount.RefreshToken,
			TokenExpirationDateTime: zohoDeskToolWithToken.ZohoAccount.TokenExpirationDateTime,
		},
		AuthName: zohoDeskToolWithToken.AuthName,
	}
	return zohoDeskTool, nil
}

func (zohoDeskTool *ZohoDeskTool) StopAutoRenew(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	if zohoDeskTool.Scheduler == nil {
		err = errors.New("scheduler not defined")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	zohoDeskTool.Scheduler.Unschedule(ctx, "", fmt.Sprint(zohoDeskTool.ToolName, "_", zohoDeskTool.Tool.Hooks.ARRT, "_", tenantId))
	toolResult = make(map[string]interface{})
	toolResult["stop_auto_renew_status"] = "success"
	zohoDeskTool.ZohoAccount.TokenExpirationDateTime = ""
	persistStore = true
	return toolResult, persistStore, nil
}
