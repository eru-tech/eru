package stocks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eru_models "github.com/eru-tech/eru/eru-models"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

const (
	SvAuthSubdomain     = "eruauth"
	SvModelSubdomain    = "model"
	SvFunctionSubdomain = "erufunc"
	SvQlSubdomain       = "eruql"
)

const (
	SvLogin                   = "login"
	SvGetPouchStocksReturns   = "get_pouch_stocks_returns"
	SvGeneratePouch           = "generate_pouch"
	SvSavePouch               = "save_pouch"
	SvGetUniverseList         = "get_universe_list"
	SvGetPouchlist            = "get_pouchlist"
	SvGetPouchTimeframes      = "get_pouch_timeframes"
	SmartvaluesDefaultProject = "smartvalues"
)

type SmartvaluesAccount struct {
	BaseUrl   string `json:"base_url" eru:"required" desc:"smartvalues base url e.g. smartvalues.com - the service subdomain is prefixed to it by the tool"`
	ProjectId string `json:"project_id" desc:"smartvalues project name used in the url path"`
	Username  string `json:"username" eru:"required" desc:"smartvalues login username"`
	Password  string `json:"password" eru:"required" desc:"smartvalues login password in plain text - it is base64 encoded before being posted"`
}

type SmartvaluesTool struct {
	tools.Tool
	SmartvaluesAccount SmartvaluesAccount `json:"smartvalues_account"`
}

type smartvaluesTokens struct {
	AccessToken string
	IdToken     string
	Expiry      time.Time
}

type smartvaluesLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
	Expiry       string `json:"expiry"`
	ExpiresIn    int    `json:"expires_in"`
	Id           string `json:"id"`
}

var (
	svTokenMu    sync.RWMutex
	svTokenCache = make(map[string]smartvaluesTokens)
)

type SmartvaluesLoginParams struct {
}

type SmartvaluesGetPouchStocksReturnsParams struct {
	PouchId        int    `json:"pouch_id" eru:"required" desc:"id of the pouch whose stock level returns are to be fetched"`
	StockPriceDate string `json:"stock_price_date" eru:"required" desc:"date in YYYY-MM-DD format as of which stock prices are considered"`
	Exchange       string `json:"exchange" eru:"required" desc:"exchange code e.g. XNAS for nasdaq, XNSE for nse"`
}

type SmartvaluesGeneratePouchParams struct {
	Exchange        string `json:"exchange" eru:"required" desc:"exchange code e.g. XNAS for nasdaq, XNSE for nse"`
	UniverseName    string `json:"universe_name" eru:"required" desc:"name of the stock universe the pouch is generated from e.g. Global 500"`
	StartDate       string `json:"start_date" eru:"required" desc:"pouch start date in RFC3339 format e.g. 2021-08-01T18:30:00.000Z"`
	ReviewDate      string `json:"review_date" eru:"required" desc:"pouch review date in RFC3339 format"`
	TimeframeSize   int    `json:"timeframe_size" eru:"required" desc:"number of stocks selected in each timeframe"`
	LookbackPeriods string `json:"lookback_periods" eru:"required" desc:"number of lookback months as a string e.g. 9"`
	LookbackType    string `json:"lookback_type" eru:"required" desc:"lookback type e.g. FIXED_START_DATE"`
	RankTolerance   int    `json:"rank_tolerance" eru:"required" desc:"rank tolerance within which an existing stock is retained"`
	PouchType       string `json:"pouch_type" eru:"required" desc:"algo variant used to rank stocks e.g. PRICE"`
	StockPriceDate  string `json:"stock_price_date" eru:"required" desc:"date in RFC3339 format as of which stock prices are considered"`
	ReviewFreq      string `json:"review_freq" eru:"required" desc:"review frequency e.g. MONTHLY, QUARTERLY, ANNUALLY"`
	ReviewFreqDay   int    `json:"review_freq_day" eru:"required" desc:"day of the period on which the pouch is reviewed"`
	ResetYears      int    `json:"reset_years" desc:"number of years after which the pouch is reset"`
	RetainStocks    string `json:"retain_stocks" desc:"Y to retain existing stocks within rank tolerance, else N"`
	TfWt            string `json:"tf_wt" desc:"timeframe weight allocation e.g. DISTINCT"`
	ExcludeNonFno   string `json:"exclude_non_fno" desc:"Y to exclude stocks not in the fno segment, else N"`
}

type SmartvaluesPouchDocs struct {
	IconBase64         string `json:"icon_base64" desc:"base64 encoded pouch icon"`
	PouchName          string `json:"pouch_name" eru:"required" desc:"name of the pouch"`
	InvestmentStrategy string `json:"investment_strategy" desc:"investment strategy of the pouch e.g. Momentum"`
	RiskProfile        string `json:"risk_profile" desc:"risk profile of the pouch e.g. LOW, MODERATE, HIGH"`
	Description        string `json:"description" desc:"description of the pouch"`
	IsPublic           bool   `json:"is_public" desc:"true if the pouch is visible to all subscribers"`
	IsHidden           bool   `json:"is_hidden" desc:"true if the pouch is hidden"`
	BenchmarkIndex     string `json:"benchmark_index" desc:"index the pouch returns are benchmarked against"`
	Exchange           string `json:"exchange" desc:"exchange code of the pouch e.g. XNAS"`
	StopSub            string `json:"stop_sub" desc:"Y to stop new subscriptions on this pouch, else N"`
}

type SmartvaluesPouchSubscription struct {
	SubscriptionFreq   string  `json:"subscription_freq" desc:"subscription frequency e.g. MONTHLY, QUARTERLY, ANNUALLY"`
	SubscriptionPerc   string  `json:"subscription_perc" desc:"subscription fee as a percentage of assets under advice"`
	SubscriptionAmount string  `json:"subscription_amount" desc:"flat subscription fee amount"`
	SubscriptionType   string  `json:"subscription_type" desc:"how the fee is derived e.g. MIN, MAX, FLAT"`
	DiscountPerc       *string `json:"discount_perc" desc:"discount percentage on the subscription fee"`
	PouchId            int     `json:"pouch_id" desc:"populated automatically from the pouch_id of the request"`
}

type SmartvaluesSavePouchParams struct {
	PouchId       int                            `json:"pouch_id" eru:"required" desc:"id of the pouch returned by generate_pouch"`
	Docs          SmartvaluesPouchDocs           `json:"docs" eru:"required" desc:"descriptive attributes of the pouch"`
	Subscriptions []SmartvaluesPouchSubscription `json:"subscriptions" desc:"subscription plans of the pouch"`
	Exchange      string                         `json:"exchange" eru:"required" desc:"exchange code e.g. XNAS for nasdaq, XNSE for nse"`
}

type SmartvaluesGetUniverseListParams struct {
	UniverseType string `json:"universe_type" eru:"required" desc:"type of universe e.g. SYSTEM, CUSTOM"`
	Exchange     string `json:"exchange" eru:"required" desc:"exchange or segment code e.g. USEQ"`
}

type SmartvaluesGetPouchTimeframesParams struct {
	PouchId int `json:"pouch_id" eru:"required" desc:"id of the pouch whose timeframes and timeframe level stocks are to be fetched"`
}

type SmartvaluesGetPouchlistParams struct {
	Sort        int    `json:"sort" eru:"required" desc:"sort order code of the pouch list e.g. -13"`
	CurrentDate string `json:"current_date" eru:"required" desc:"date in YYYY-MM-DD format as of which the list is fetched"`
	Exchange    string `json:"exchange" eru:"required" desc:"exchange code e.g. XNAS for nasdaq, XNSE for nse"`
}

var smartvaluesToolActions = []tools.ToolAction{
	{
		ActionName:   SvLogin,
		Description:  "Login to smartvalues with the configured username and password and cache the tokens in memory for the rest of the session",
		SystemPrompt: "Login is performed automatically by every other action - call it explicitly only to force a fresh login.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(SmartvaluesLoginParams{}), []string{})
		},
	},
	{
		ActionName:   SvGetPouchStocksReturns,
		Description:  "Fetch pouch level and stock level realized and unrealized returns of a smartvalues pouch",
		SystemPrompt: "Use the pouch_id of an existing pouch. Returns cagr, realized and unrealized returns for the pouch and each of its stocks.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(SmartvaluesGetPouchStocksReturnsParams{}), []string{})
		},
	},
	{
		ActionName:   SvGeneratePouch,
		Description:  "Run the smartvalues proprietary algorithm to generate a stock pouch (recommendation) with its timeframes and selected stocks",
		SystemPrompt: "Dates are to be passed in RFC3339 format. The generated pouch is temporary until it is persisted with the save_pouch action.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(SmartvaluesGeneratePouchParams{}), []string{})
		},
	},
	{
		ActionName:   SvSavePouch,
		Description:  "Save the descriptive attributes and subscription plans of a generated smartvalues pouch",
		SystemPrompt: "Call this only after generate_pouch, using the pouch_id returned by it.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(SmartvaluesSavePouchParams{}), []string{})
		},
	},
	{
		ActionName:   SvGetUniverseList,
		Description:  "List the stock universes available on smartvalues for an exchange",
		SystemPrompt: "Use this to find a valid universe_name before generating a pouch.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(SmartvaluesGetUniverseListParams{}), []string{})
		},
	},
	{
		ActionName:   SvGetPouchTimeframes,
		Description:  "Fetch the timeframes and the timeframe level stocks of an existing smartvalues pouch",
		SystemPrompt: "Use get_pouchlist to find a valid pouch_id.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(SmartvaluesGetPouchTimeframesParams{}), []string{})
		},
	},
	{
		ActionName:   SvGetPouchlist,
		Description:  "List the smartvalues pouches of an exchange along with their returns, review dates and subscription plans",
		SystemPrompt: "Use this to find an existing pouch_id.",
		OutputSchema: eru_models.JSONSchema{},
		Parameters:   eru_models.JSONSchema{},
		GetParameters: func() eru_models.JSONSchema {
			return utils.StructToJSONSchema(reflect.TypeOf(SmartvaluesGetPouchlistParams{}), []string{})
		},
	},
}

func (svTool *SmartvaluesTool) GetActionsList() []tools.ActionInfo {
	infos := make([]tools.ActionInfo, len(smartvaluesToolActions))
	for i, action := range smartvaluesToolActions {
		infos[i] = tools.ActionInfo{Name: action.ActionName, Description: action.Description}
	}
	return infos
}

func (svTool *SmartvaluesTool) GetActions() []tools.ToolAction {
	return smartvaluesToolActions
}

func (svTool *SmartvaluesTool) SetToolAction(actionName string) {
	for _, action := range smartvaluesToolActions {
		if action.ActionName == actionName {
			svTool.ToolAction = action
			return
		}
	}
	svTool.ToolAction = tools.ToolAction{}
}

func (svTool *SmartvaluesTool) GetSpec() tools.Tooling {
	return svTool
}

func (svTool *SmartvaluesTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &svTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (svTool *SmartvaluesTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(svTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (svTool *SmartvaluesTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &SmartvaluesTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}

func (svTool *SmartvaluesTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SmartvaluesTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case SvLogin:
		toolResult, toolRequest, persistStore, err = svTool.ExecuteLogin(ctx, projectId, tenantId)
	case SvGetPouchStocksReturns:
		toolResult, toolRequest, persistStore, err = svTool.ExecuteGetPouchStocksReturns(ctx, projectId, tenantId, params)
	case SvGeneratePouch:
		toolResult, toolRequest, persistStore, err = svTool.ExecuteGeneratePouch(ctx, projectId, tenantId, params)
	case SvSavePouch:
		toolResult, toolRequest, persistStore, err = svTool.ExecuteSavePouch(ctx, projectId, tenantId, params)
	case SvGetUniverseList:
		toolResult, toolRequest, persistStore, err = svTool.ExecuteGetUniverseList(ctx, projectId, tenantId, params)
	case SvGetPouchlist:
		toolResult, toolRequest, persistStore, err = svTool.ExecuteGetPouchlist(ctx, projectId, tenantId, params)
	case SvGetPouchTimeframes:
		toolResult, toolRequest, persistStore, err = svTool.ExecuteGetPouchTimeframes(ctx, projectId, tenantId, params)
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

		hookResult, hookErr := svTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if hookErr != nil {
			logs.WithContext(bgCtx).Error(hookErr.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (svTool *SmartvaluesTool) projectName() string {
	if svTool.SmartvaluesAccount.ProjectId == "" {
		return SmartvaluesDefaultProject
	}
	return svTool.SmartvaluesAccount.ProjectId
}

func (svTool *SmartvaluesTool) serviceBaseUrl(ctx context.Context, subDomain string) (baseUrl string, err error) {
	domain := strings.TrimSuffix(strings.TrimSpace(svTool.SmartvaluesAccount.BaseUrl), "/")
	scheme := "https"
	if strings.HasPrefix(domain, "http://") {
		scheme = "http"
		domain = strings.TrimPrefix(domain, "http://")
	} else if strings.HasPrefix(domain, "https://") {
		domain = strings.TrimPrefix(domain, "https://")
	}
	if domain == "" {
		err = errors.New("base_url is not set for smartvalues tool")
		logs.WithContext(ctx).Error(err.Error())
		return "", err
	}
	return fmt.Sprint(scheme, "://", subDomain, ".", domain), nil
}

func (svTool *SmartvaluesTool) validateAccount(ctx context.Context) (err error) {
	if svTool.SmartvaluesAccount.BaseUrl == "" {
		err = errors.New("base_url is not set for smartvalues tool")
	} else if svTool.SmartvaluesAccount.Username == "" {
		err = errors.New("username is not set for smartvalues tool")
	} else if svTool.SmartvaluesAccount.Password == "" {
		err = errors.New("password is not set for smartvalues tool")
	}
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
	}
	return err
}

func (svTool *SmartvaluesTool) tokenCacheKey(projectId string, tenantId string) string {
	return fmt.Sprint(projectId, "_", tenantId, "_", svTool.ToolName, "_", svTool.SmartvaluesAccount.Username)
}

func (svTool *SmartvaluesTool) login(ctx context.Context, cacheKey string) (loginResponse smartvaluesLoginResponse, err error) {
	logs.WithContext(ctx).Debug("SmartvaluesTool login - Start")
	if err = svTool.validateAccount(ctx); err != nil {
		return loginResponse, err
	}
	authBaseUrl, err := svTool.serviceBaseUrl(ctx, SvAuthSubdomain)
	if err != nil {
		return loginResponse, err
	}
	url := fmt.Sprint(authBaseUrl, "/", svTool.projectName(), "/eru/login")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	body := map[string]interface{}{
		"username": svTool.SmartvaluesAccount.Username,
		"password": base64.StdEncoding.EncodeToString([]byte(svTool.SmartvaluesAccount.Password)),
	}
	res, _, _, _, err := utils.CallHttp(ctx, http.MethodPost, url, headers, map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return loginResponse, err
	}
	resBytes, err := json.Marshal(res)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return loginResponse, err
	}
	if err = json.Unmarshal(resBytes, &loginResponse); err != nil {
		err = logs.Err(ctx, err, "")
		return loginResponse, err
	}
	if loginResponse.AccessToken == "" {
		err = errors.New("smartvalues login did not return an access_token")
		logs.WithContext(ctx).Error(err.Error())
		return loginResponse, err
	}

	expiry := time.Time{}
	if loginResponse.Expiry != "" {
		expiry, err = time.Parse(time.RFC3339, loginResponse.Expiry)
		if err != nil {
			logs.WithContext(ctx).Info(fmt.Sprint("unable to parse smartvalues token expiry : ", err.Error()))
			expiry = time.Time{}
			err = nil
		}
	}
	if expiry.IsZero() && loginResponse.ExpiresIn > 0 {
		expiry = time.Now().UTC().Add(time.Duration(loginResponse.ExpiresIn) * time.Second)
	}

	svTokenMu.Lock()
	svTokenCache[cacheKey] = smartvaluesTokens{
		AccessToken: loginResponse.AccessToken,
		IdToken:     loginResponse.IdToken,
		Expiry:      expiry,
	}
	svTokenMu.Unlock()
	return loginResponse, nil
}

func (svTool *SmartvaluesTool) getTokens(ctx context.Context, cacheKey string, forceLogin bool) (tokens smartvaluesTokens, err error) {
	if !forceLogin {
		svTokenMu.RLock()
		cachedTokens, cached := svTokenCache[cacheKey]
		svTokenMu.RUnlock()
		if cached && cachedTokens.AccessToken != "" &&
			(cachedTokens.Expiry.IsZero() || time.Now().UTC().Add(30*time.Second).Before(cachedTokens.Expiry)) {
			return cachedTokens, nil
		}
	}
	if _, err = svTool.login(ctx, cacheKey); err != nil {
		return tokens, err
	}
	svTokenMu.RLock()
	tokens = svTokenCache[cacheKey]
	svTokenMu.RUnlock()
	return tokens, nil
}

func smartvaluesAuthHeaders(tokens smartvaluesTokens) http.Header {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	headers.Set("authorization", tokens.AccessToken)
	headers.Set("id_token", tokens.IdToken)
	return headers
}

func (svTool *SmartvaluesTool) callSmartvalues(ctx context.Context, projectId string, tenantId string, url string, body interface{}) (res interface{}, err error) {
	logs.WithContext(ctx).Debug(fmt.Sprint("callSmartvalues - Start : ", url))
	cacheKey := svTool.tokenCacheKey(projectId, tenantId)
	tokens, err := svTool.getTokens(ctx, cacheKey, false)
	if err != nil {
		return nil, err
	}

	res, _, _, statusCode, err := utils.CallHttp(ctx, http.MethodPost, url, smartvaluesAuthHeaders(tokens), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err == nil {
		return res, nil
	}
	if statusCode != http.StatusUnauthorized {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	logs.WithContext(ctx).Info("smartvalues returned 401 - performing login and retrying the action")
	tokens, err = svTool.getTokens(ctx, cacheKey, true)
	if err != nil {
		return nil, err
	}
	res, _, _, _, err = utils.CallHttp(ctx, http.MethodPost, url, smartvaluesAuthHeaders(tokens), map[string]string{}, []*http.Cookie{}, map[string]string{}, body)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	return res, nil
}

func smartvaluesParams(ctx context.Context, params map[string]interface{}, actionParams interface{}) (err error) {
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return err
	}
	if err = json.Unmarshal(paramsBytes, actionParams); err != nil {
		err = logs.Err(ctx, err, "")
		return err
	}
	return nil
}

func (svTool *SmartvaluesTool) ExecuteLogin(ctx context.Context, projectId string, tenantId string) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SmartvaluesTool ExecuteLogin - Start")
	loginResponse, err := svTool.login(ctx, svTool.tokenCacheKey(projectId, tenantId))
	if err != nil {
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["login_status"] = "success"
	toolResult["expiry"] = loginResponse.Expiry
	toolResult["expires_in"] = loginResponse.ExpiresIn
	toolResult["id"] = loginResponse.Id
	return toolResult, map[string]interface{}{"username": svTool.SmartvaluesAccount.Username}, false, nil
}

func (svTool *SmartvaluesTool) ExecuteGetPouchStocksReturns(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SmartvaluesTool ExecuteGetPouchStocksReturns - Start")
	actionParams := SmartvaluesGetPouchStocksReturnsParams{}
	if err = smartvaluesParams(ctx, params, &actionParams); err != nil {
		return nil, nil, false, err
	}
	functionBaseUrl, err := svTool.serviceBaseUrl(ctx, SvFunctionSubdomain)
	if err != nil {
		return nil, nil, false, err
	}
	body := map[string]interface{}{
		"pouch_id":         actionParams.PouchId,
		"stock_price_date": actionParams.StockPriceDate,
		"exchange":         actionParams.Exchange,
	}
	url := fmt.Sprint(functionBaseUrl, "/", svTool.projectName(), "/func/", SvGetPouchStocksReturns)
	res, err := svTool.callSmartvalues(ctx, projectId, tenantId, url, body)
	if err != nil {
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, body, false, nil
}

func (svTool *SmartvaluesTool) ExecuteGeneratePouch(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SmartvaluesTool ExecuteGeneratePouch - Start")
	actionParams := SmartvaluesGeneratePouchParams{}
	if err = smartvaluesParams(ctx, params, &actionParams); err != nil {
		return nil, nil, false, err
	}
	modelBaseUrl, err := svTool.serviceBaseUrl(ctx, SvModelSubdomain)
	if err != nil {
		return nil, nil, false, err
	}
	bodyBytes, err := json.Marshal(actionParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}
	body := make(map[string]interface{})
	if err = json.Unmarshal(bodyBytes, &body); err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}
	url := fmt.Sprint(modelBaseUrl, "/get_timeframes")
	res, err := svTool.callSmartvalues(ctx, projectId, tenantId, url, body)
	if err != nil {
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, body, false, nil
}

func (svTool *SmartvaluesTool) ExecuteSavePouch(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SmartvaluesTool ExecuteSavePouch - Start")
	actionParams := SmartvaluesSavePouchParams{}
	if err = smartvaluesParams(ctx, params, &actionParams); err != nil {
		return nil, nil, false, err
	}
	functionBaseUrl, err := svTool.serviceBaseUrl(ctx, SvFunctionSubdomain)
	if err != nil {
		return nil, nil, false, err
	}
	if actionParams.Docs.Exchange == "" {
		actionParams.Docs.Exchange = actionParams.Exchange
	}
	for i := range actionParams.Subscriptions {
		actionParams.Subscriptions[i].PouchId = actionParams.PouchId
	}
	bodyBytes, err := json.Marshal(actionParams)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}
	body := make(map[string]interface{})
	if err = json.Unmarshal(bodyBytes, &body); err != nil {
		err = logs.Err(ctx, err, "")
		return nil, nil, false, err
	}
	url := fmt.Sprint(functionBaseUrl, "/", svTool.projectName(), "/func/edit_pouch")
	res, err := svTool.callSmartvalues(ctx, projectId, tenantId, url, body)
	if err != nil {
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, body, false, nil
}

func (svTool *SmartvaluesTool) executeMyQuery(ctx context.Context, projectId string, tenantId string, queryName string, outputType string, body map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	qlBaseUrl, err := svTool.serviceBaseUrl(ctx, SvQlSubdomain)
	if err != nil {
		return nil, nil, false, err
	}
	url := fmt.Sprint(qlBaseUrl, "/store/", svTool.projectName(), "/myquery/execute/", queryName)
	if outputType != "" {
		url = fmt.Sprint(url, "/", outputType)
	}
	res, err := svTool.callSmartvalues(ctx, projectId, tenantId, url, body)
	if err != nil {
		return nil, nil, false, err
	}
	toolResult = make(map[string]interface{})
	toolResult["result"] = res
	return toolResult, body, false, nil
}

func (svTool *SmartvaluesTool) ExecuteGetUniverseList(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SmartvaluesTool ExecuteGetUniverseList - Start")
	actionParams := SmartvaluesGetUniverseListParams{}
	if err = smartvaluesParams(ctx, params, &actionParams); err != nil {
		return nil, nil, false, err
	}
	body := map[string]interface{}{
		"universe_type": actionParams.UniverseType,
		"exchange":      actionParams.Exchange,
	}
	return svTool.executeMyQuery(ctx, projectId, tenantId, SvGetUniverseList, "", body)
}

func (svTool *SmartvaluesTool) ExecuteGetPouchlist(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SmartvaluesTool ExecuteGetPouchlist - Start")
	actionParams := SmartvaluesGetPouchlistParams{}
	if err = smartvaluesParams(ctx, params, &actionParams); err != nil {
		return nil, nil, false, err
	}
	body := map[string]interface{}{
		"sort":         actionParams.Sort,
		"current_date": actionParams.CurrentDate,
		"exchange":     actionParams.Exchange,
	}
	return svTool.executeMyQuery(ctx, projectId, tenantId, SvGetPouchlist, "", body)
}

func (svTool *SmartvaluesTool) ExecuteGetPouchTimeframes(ctx context.Context, projectId string, tenantId string, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("SmartvaluesTool ExecuteGetPouchTimeframes - Start")
	actionParams := SmartvaluesGetPouchTimeframesParams{}
	if err = smartvaluesParams(ctx, params, &actionParams); err != nil {
		return nil, nil, false, err
	}
	body := map[string]interface{}{
		"pouch_id": actionParams.PouchId,
	}
	return svTool.executeMyQuery(ctx, projectId, tenantId, SvGetPouchTimeframes, "sql", body)
}

func init() {
	tools.RegisterToolCatalog(tools.ToolCatalogEntry{
		Public:      false,
		ToolType:    "Smartvalues",
		Category:    "Finance",
		Description: "Smartvalues proprietary algorithm to generate, save and analyse stock recommendation pouches",
		Actions: func() []tools.ActionInfo {
			infos := make([]tools.ActionInfo, len(smartvaluesToolActions))
			for i, a := range smartvaluesToolActions {
				infos[i] = tools.ActionInfo{Name: a.ActionName, Description: a.Description}
			}
			return infos
		}(),
		OAuthEnabled: false,
		Icon:         "",
		IconType:     "svg",
		ToolSchema:   utils.StructToJSONSchema(reflect.TypeOf(SmartvaluesTool{}), []string{}),
	})
}
