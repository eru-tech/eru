package ecomm

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

type AmazonTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type AmazonTool struct {
	tools.Tool
	AmazonAccount AmazonAccount `json:"amazon_account"`
	AuthName      string        `json:"auth_name"`
}
type amazonToolWithToken struct {
	tools.Tool
	AmazonAccount amazonAccountWithToken
	AuthName      string
}

const (
	SellerBaseUrl = "https://graph.microsoft.com"
)

func (amazonTool *AmazonTool) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, GetOrders, GetPayments, Login, RenewToken, GetSsoUrl, StopAutoRenew)
	return actions
}

func (amazonTool *AmazonTool) GetSpec() tools.Tooling {
	return amazonTool
}

func (amazonTool *AmazonTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &amazonTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (amazonTool *AmazonTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("AmazonTool Execute - Start")
	switch actionName {
	case GetOrders:
		return amazonTool.GetOrders(ctx, params)
	case GetPayments:
		return amazonTool.GetPayments(ctx, params)
	case Login:
		return amazonTool.Login(ctx, projectId, tenantId, params, "")
	case RenewToken:
		return amazonTool.RenewToken(ctx, projectId, tenantId, params)
	case GetSsoUrl:
		return amazonTool.GetSsoUrl(ctx, projectId, tenantId, params)
	case StopAutoRenew:
		return amazonTool.StopAutoRenew(ctx, projectId, tenantId, params)
	default:
		return nil, false, fmt.Errorf("action %s not found", actionName)
	}
}

func (amazonTool *AmazonTool) GetOrders(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetOrders Execute - Start")
	url := fmt.Sprint(SellerBaseUrl, "/orders/v0/orders")
	headers := http.Header{}
	headers.Set("x-amz-access-token", amazonTool.AmazonAccount.AccessToken)
	headers.Set("x-amz-date", time.Now().UTC().Format(time.RFC3339))
	headers.Set("user-agent", amazonTool.AmazonAccount.UserAgent)

	queryParams := map[string]string{}
	for k, v := range params {
		queryParams[k] = v.(string)
	}

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["orders"] = res
	return toolResult, false, nil
}

func (amazonTool *AmazonTool) GetPayments(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetPayments Execute - Start")
	url := fmt.Sprint(SellerBaseUrl, "finances/v0/financialEvents")
	headers := http.Header{}
	headers.Set("x-amz-access-token", amazonTool.AmazonAccount.AccessToken)
	headers.Set("x-amz-date", time.Now().UTC().Format(time.RFC3339))
	headers.Set("user-agent", amazonTool.AmazonAccount.UserAgent)

	queryParams := map[string]string{}
	for k, v := range params {
		queryParams[k] = v.(string)
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["payments"] = res
	return toolResult, false, nil
}

func (amazonTool *AmazonTool) GetSsoUrl(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetSsoUrl Execute - Start")

	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", amazonTool.AuthName, "/getssourl")
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
func (amazonTool *AmazonTool) RenewToken(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	params["refresh_token"] = amazonTool.AmazonAccount.RefreshToken
	return amazonTool.Login(ctx, projectId, tenantId, params, "/renew")
}

func (amazonTool *AmazonTool) Login(ctx context.Context, projectId string, tenantId string, params map[string]interface{}, renewStr string) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("Login Execute - Start")
	if amazonTool.AuthName == "" {
		err = errors.New("auth name is required")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	eruauthUrl := ctx.Value("eruauthbaseurl").(string)
	url := fmt.Sprint(eruauthUrl, "/", projectId, "/", amazonTool.AuthName, "/idptoken", renewStr)
	logs.WithContext(ctx).Info(fmt.Sprint("url: ", url))
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, params)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	var amazonTokens AmazonTokens
	resBytes, _ := json.Marshal(res)
	err = json.Unmarshal(resBytes, &amazonTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = json.Unmarshal(resBytes, &amazonTokens)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = amazonTool.saveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(amazonTool.AuthName, "_access_token"), amazonTokens.AccessToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}
	err = amazonTool.saveTenantSecret(ctx, projectId, tenantId, fmt.Sprint(amazonTool.AuthName, "_refresh_token"), amazonTokens.RefreshToken)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, false, err
	}

	amazonTool.AmazonAccount.TokenExpirationDateTime = time.Now().UTC().Add(time.Duration(amazonTokens.ExpiresIn) * time.Second).Format(time.RFC3339)
	persistStore = true

	if amazonTool.Tool.Hooks.ARRT != "" {
		hookBody := map[string]interface{}{
			"Vars": map[string]interface{}{
				"Body": map[string]interface{}{
					"tool_name": amazonTool.ToolName,
					"tenant_id": tenantId,
				},
				"OrgBody": map[string]interface{}{
					"tool_name": amazonTool.ToolName,
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
		jobName := fmt.Sprint(amazonTool.Tool.Hooks.ARRT, "_", tenantId)
		err = amazonTool.Scheduler.Unschedule(ctx, "", jobName)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			//continue even if unschedule fails
		}

		schedulerCommand := fmt.Sprint("CALL schedule_procedure('", amazonTool.Tool.Hooks.ARRT, "','", string(hookBodyBytes), "','", amazonTool.Scheduler.GetSchedulerName(), "')")

		cronStr := utils.GetCronStr(ctx, time.Now().UTC().Add(1*time.Hour))
		jobId, err := amazonTool.Scheduler.Schedule(ctx, jobName, schedulerCommand, cronStr)
		if err != nil {
			return nil, persistStore, err
		}
		logs.WithContext(ctx).Info(fmt.Sprint("jobId: ", jobId))
	}
	toolResult = make(map[string]interface{})
	toolResult["login_status"] = "success"
	return toolResult, persistStore, nil
}

func (amazonTool *AmazonTool) saveTenantSecret(ctx context.Context, projectId string, tenantId string, secretName string, secretValue string) (err error) {
	logs.WithContext(ctx).Debug("saveTenantSecret Execute - Start")
	eruaiport := ctx.Value("eruaiport").(string)
	url := fmt.Sprint("http://localhost:", eruaiport, "/store/", projectId, "/", tenantId, "/sm/set")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	secretPost := make(map[string]interface{})
	secretInnerPost := make(map[string]interface{})
	secretInnerPost[secretName] = secretValue
	secretPost["secret_value"] = secretInnerPost
	_, _, _, _, err = utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, secretPost)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (amazonTool *AmazonTool) SetPrivateAttributes(ctx context.Context, realTool tools.Tooling) (err error) {
	amazonTool.AmazonAccount.AccessToken = "$SECRET_amazon_access_token"
	amazonTool.AmazonAccount.RefreshToken = "$SECRET_amazon_refresh_token"
	return nil
}

func (amazonTool *AmazonTool) GetBytes(ctx context.Context) ([]byte, error) {

	amazonToolWithToken := amazonToolWithToken{
		Tool: amazonTool.Tool,
		AmazonAccount: amazonAccountWithToken{
			UserAgent:               amazonTool.AmazonAccount.UserAgent,
			AccessToken:             amazonTool.AmazonAccount.AccessToken,
			RefreshToken:            amazonTool.AmazonAccount.RefreshToken,
			TokenExpirationDateTime: amazonTool.AmazonAccount.TokenExpirationDateTime,
		},
		AuthName: amazonTool.AuthName,
	}

	toolJson, err := json.Marshal(amazonToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}
func (amazonTool *AmazonTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	amazonToolWithToken := amazonToolWithToken{}
	err := json.Unmarshal(toolObjJson, &amazonToolWithToken)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	amazonTool = &AmazonTool{
		Tool: amazonToolWithToken.Tool,
		AmazonAccount: AmazonAccount{
			UserAgent:               amazonToolWithToken.AmazonAccount.UserAgent,
			AccessToken:             amazonToolWithToken.AmazonAccount.AccessToken,
			RefreshToken:            amazonToolWithToken.AmazonAccount.RefreshToken,
			TokenExpirationDateTime: amazonToolWithToken.AmazonAccount.TokenExpirationDateTime,
		},
		AuthName: amazonToolWithToken.AuthName,
	}
	return amazonTool, nil
}
func (amazonTool *AmazonTool) StopAutoRenew(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	if amazonTool.Scheduler == nil {
		err = errors.New("scheduler not defined")
		logs.Err(ctx, err, "")
		return nil, false, err
	}
	amazonTool.Scheduler.Unschedule(ctx, "", fmt.Sprint(amazonTool.Tool.Hooks.ARRT, "_", tenantId))
	toolResult = make(map[string]interface{})
	toolResult["stop_auto_renew_status"] = "success"
	amazonTool.AmazonAccount.TokenExpirationDateTime = ""
	persistStore = true
	return toolResult, persistStore, nil
}
