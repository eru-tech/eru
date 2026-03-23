package stocks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	tools "github.com/eru-tech/eru/eru-ai/tools"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server "github.com/eru-tech/eru/eru-server/server"
	utils "github.com/eru-tech/eru/eru-utils"
)

type MassiveAccount struct {
	ApiKey string `json:"api_key"`
}

const (
	GetStocks      = "get_stocks"
	GetStockPrices = "get_stock_prices"
	MarketHolidays = "market_holidays"
	StockSplits    = "stock_splits"
	StockDividends = "stock_dividends"
	GetIndiceValue = "get_indice_value"
)

type MassiveTool struct {
	tools.Tool
	MassiveAccount MassiveAccount `json:"massive_account"`
}

const (
	MassiveBaseUrl = "https://api.massive.com"
)

func (massiveTool *MassiveTool) GetActionsList() []string {
	actions := []string{}
	actions = append(actions, GetStocks, GetStockPrices, MarketHolidays, StockSplits, StockDividends, GetIndiceValue)
	return actions
}

func (massiveTool *MassiveTool) GetSpec() tools.Tooling {
	return massiveTool
}

func (massiveTool *MassiveTool) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &massiveTool)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func (massiveTool *MassiveTool) Execute(ctx context.Context, projectId string, tenantId string, actionName string, params map[string]interface{}) (toolResult map[string]interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("MassiveTool Execute - Start")
	var toolRequest interface{}
	switch actionName {
	case GetStocks:
		toolResult, toolRequest, persistStore, err = massiveTool.GetStocks(ctx, params)
	case GetStockPrices:
		toolResult, toolRequest, persistStore, err = massiveTool.GetStockPrices(ctx, params)
	case MarketHolidays:
		toolResult, toolRequest, persistStore, err = massiveTool.GetMarketHolidays(ctx, params)
	case StockSplits:
		toolResult, toolRequest, persistStore, err = massiveTool.GetStockSplits(ctx, params)
	case StockDividends:
		toolResult, toolRequest, persistStore, err = massiveTool.GetStockDividends(ctx, params)
	case GetIndiceValue:
		toolResult, toolRequest, persistStore, err = massiveTool.GetIndiceValue(ctx, params)
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
			err = errors.New("erufuncbaseurl not found in context")
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		efurlString, ok := efurl.(string)
		if !ok {
			err = errors.New("erufuncbaseurl is not a string")
			logs.WithContext(ctx).Error(err.Error())
			return
		} else {
			bgCtx = context.WithValue(bgCtx, tools.EruFuncBaseUrlKey, efurlString)
		}

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

		hookResult, err := massiveTool.ExecuteHook(bgCtx, "poex", actionName, projectId, tenantId, body, nil)
		if err != nil {
			logs.WithContext(bgCtx).Error(err.Error())
			return
		}
		logs.WithContext(bgCtx).Info(fmt.Sprint(hookResult))
	}, server.ContinueOnMaxRetries)

	return toolResult, persistStore, err
}

func (massiveTool *MassiveTool) GetStocks(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetStocks Execute - Start")

	nextUrl := ""

	queryParams := map[string]string{}
	for k, v := range params {
		queryParams[k] = fmt.Sprintf("%v", v)
	}

	allTickers, err := massiveTool.getStocksRecursive(ctx, queryParams, nextUrl)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["tickers"] = allTickers
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (massiveTool *MassiveTool) getStocksRecursive(ctx context.Context, queryParams map[string]string, nextUrl string) ([]interface{}, error) {
	logs.WithContext(ctx).Debug("getStocksRecursive Execute - Start")
	var allTickers []interface{}

	var url string
	currentQueryParams := make(map[string]string)

	if nextUrl != "" {
		url = nextUrl
	} else {
		url = fmt.Sprint(MassiveBaseUrl, "/v3/reference/tickers")
		for k, v := range queryParams {
			currentQueryParams[k] = v
		}
	}
	currentQueryParams["apiKey"] = massiveTool.MassiveAccount.ApiKey
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, currentQueryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	responseMap, ok := res.(map[string]interface{})
	if !ok {
		logs.WithContext(ctx).Error("Response is not a map")
		return nil, errors.New("invalid response format")
	}

	if resultsData, exists := responseMap["results"]; exists {
		if resultsList, ok := resultsData.([]interface{}); ok {
			allTickers = append(allTickers, resultsList...)
		}
	}

	if nextUrlField, exists := responseMap["next_url"]; exists {
		if nextUrlStr, ok := nextUrlField.(string); ok && nextUrlStr != "" {
			logs.WithContext(ctx).Info(fmt.Sprintf("Found next_url: %s, making recursive call", nextUrlStr))
			time.Sleep(13 * time.Second)
			nextTickers, err := massiveTool.getStocksRecursive(ctx, queryParams, nextUrlStr)
			if err != nil {
				return nil, err
			}
			allTickers = append(allTickers, nextTickers...)
		}
	}

	logs.WithContext(ctx).Debug(fmt.Sprintf("No more next_url found. Total tickers collected: %d", len(allTickers)))

	return allTickers, nil
}

func (massiveTool *MassiveTool) GetStockPrices(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetStockPrices Execute - Start")

	date, exists := params["date"]
	if !exists {
		return nil, nil, false, errors.New("date parameter is required")
	}
	dateStr := fmt.Sprintf("%v", date)
	if dateStr == "" {
		return nil, nil, false, errors.New("date parameter cannot be empty")
	}

	url := fmt.Sprint(MassiveBaseUrl, "/v2/aggs/grouped/locale/us/market/stocks/", dateStr)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		if k != "date" {
			queryParams[k] = fmt.Sprintf("%v", v)
		}
	}
	queryParams["apiKey"] = massiveTool.MassiveAccount.ApiKey

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["data"] = res
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (massiveTool *MassiveTool) GetMarketHolidays(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetMarketHolidays Execute - Start")

	url := fmt.Sprint(MassiveBaseUrl, "/v1/marketstatus/upcoming")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		queryParams[k] = fmt.Sprintf("%v", v)
	}
	queryParams["apiKey"] = massiveTool.MassiveAccount.ApiKey

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["data"] = res
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (massiveTool *MassiveTool) GetStockSplits(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetStockSplits Execute - Start")

	nextUrl := ""

	queryParams := map[string]string{}
	for k, v := range params {
		queryParams[k] = fmt.Sprintf("%v", v)
	}

	allSplits, err := massiveTool.getStockSplitsRecursive(ctx, queryParams, nextUrl)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["splits"] = allSplits
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (massiveTool *MassiveTool) getStockSplitsRecursive(ctx context.Context, queryParams map[string]string, nextUrl string) ([]interface{}, error) {
	logs.WithContext(ctx).Debug("getStockSplitsRecursive Execute - Start")
	var allSplits []interface{}

	var url string
	currentQueryParams := make(map[string]string)

	if nextUrl != "" {
		url = nextUrl
	} else {
		url = fmt.Sprint(MassiveBaseUrl, "/v3/reference/splits")
		for k, v := range queryParams {
			currentQueryParams[k] = v
		}
	}
	currentQueryParams["apiKey"] = massiveTool.MassiveAccount.ApiKey

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, currentQueryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	responseMap, ok := res.(map[string]interface{})
	if !ok {
		logs.WithContext(ctx).Error("Response is not a map")
		return nil, errors.New("invalid response format")
	}

	if resultsData, exists := responseMap["results"]; exists {
		if resultsList, ok := resultsData.([]interface{}); ok {
			allSplits = append(allSplits, resultsList...)
		}
	}

	if nextUrlField, exists := responseMap["next_url"]; exists {
		if nextUrlStr, ok := nextUrlField.(string); ok && nextUrlStr != "" {
			logs.WithContext(ctx).Info(fmt.Sprintf("Found next_url: %s, making recursive call", nextUrlStr))
			time.Sleep(13 * time.Second)
			nextSplits, err := massiveTool.getStockSplitsRecursive(ctx, queryParams, nextUrlStr)
			if err != nil {
				return nil, err
			}
			allSplits = append(allSplits, nextSplits...)
		}
	}

	logs.WithContext(ctx).Debug(fmt.Sprintf("No more next_url found. Total splits collected: %d", len(allSplits)))

	return allSplits, nil
}

func (massiveTool *MassiveTool) GetStockDividends(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetStockDividends Execute - Start")

	nextUrl := ""

	queryParams := map[string]string{}
	for k, v := range params {
		queryParams[k] = fmt.Sprintf("%v", v)
	}

	allDividends, err := massiveTool.getStockDividendsRecursive(ctx, queryParams, nextUrl)
	if err != nil {
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["dividends"] = allDividends
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (massiveTool *MassiveTool) getStockDividendsRecursive(ctx context.Context, queryParams map[string]string, nextUrl string) ([]interface{}, error) {
	logs.WithContext(ctx).Debug("getStockDividendsRecursive Execute - Start")
	var allDividends []interface{}

	var url string
	currentQueryParams := make(map[string]string)

	if nextUrl != "" {
		url = nextUrl
	} else {
		url = fmt.Sprint(MassiveBaseUrl, "/v3/reference/dividends")
		for k, v := range queryParams {
			currentQueryParams[k] = v
		}
	}
	currentQueryParams["apiKey"] = massiveTool.MassiveAccount.ApiKey

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, currentQueryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	responseMap, ok := res.(map[string]interface{})
	if !ok {
		logs.WithContext(ctx).Error("Response is not a map")
		return nil, errors.New("invalid response format")
	}

	if resultsData, exists := responseMap["results"]; exists {
		if resultsList, ok := resultsData.([]interface{}); ok {
			allDividends = append(allDividends, resultsList...)
		}
	}

	if nextUrlField, exists := responseMap["next_url"]; exists {
		if nextUrlStr, ok := nextUrlField.(string); ok && nextUrlStr != "" {
			logs.WithContext(ctx).Info(fmt.Sprintf("Found next_url: %s, making recursive call", nextUrlStr))
			time.Sleep(13 * time.Second)
			nextDividends, err := massiveTool.getStockDividendsRecursive(ctx, queryParams, nextUrlStr)
			if err != nil {
				return nil, err
			}
			allDividends = append(allDividends, nextDividends...)
		}
	}

	logs.WithContext(ctx).Debug(fmt.Sprintf("No more next_url found. Total dividends collected: %d", len(allDividends)))

	return allDividends, nil
}

func (massiveTool *MassiveTool) GetIndiceValue(ctx context.Context, params map[string]interface{}) (toolResult map[string]interface{}, toolRequest interface{}, persistStore bool, err error) {
	logs.WithContext(ctx).Debug("GetIndiceValue Execute - Start")

	ticker, exists := params["ticker"]
	if !exists {
		return nil, nil, false, errors.New("ticker parameter is required")
	}
	tickerStr := fmt.Sprintf("%v", ticker)
	if tickerStr == "" {
		return nil, nil, false, errors.New("ticker parameter cannot be empty")
	}

	date, exists := params["date"]
	if !exists {
		return nil, nil, false, errors.New("date parameter is required")
	}
	dateStr := fmt.Sprintf("%v", date)
	if dateStr == "" {
		return nil, nil, false, errors.New("date parameter cannot be empty")
	}

	url := fmt.Sprint(MassiveBaseUrl, "/v1/open-close/", tickerStr, "/", dateStr)
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	queryParams := make(map[string]string)
	for k, v := range params {
		if k != "ticker" && k != "date" {
			queryParams[k] = fmt.Sprintf("%v", v)
		}
	}
	queryParams["apiKey"] = massiveTool.MassiveAccount.ApiKey

	res, _, _, _, err := utils.CallHttp(ctx, http.MethodGet, url, headers, map[string]string{}, []*http.Cookie{}, queryParams, nil)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, nil, false, err
	}

	toolResult = make(map[string]interface{})
	toolResult["data"] = res
	return toolResult, map[string]interface{}{"query": queryParams}, false, nil
}

func (massiveTool *MassiveTool) GetBytes(ctx context.Context) ([]byte, error) {
	toolJson, err := json.Marshal(massiveTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return toolJson, nil
}

func (massiveTool *MassiveTool) BytesToTool(ctx context.Context, toolObjJson []byte) (tools.Tooling, error) {
	newTool := &MassiveTool{}
	err := json.Unmarshal(toolObjJson, newTool)
	if err != nil {
		err = logs.Err(ctx, err, "")
		return nil, err
	}
	return newTool, nil
}
