package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type ApiGateway struct {
	Gateway
	GatewayUrl    string            `eru:"required"`
	GatewayMethod string            `eru:"required"`
	QueryParams   map[string]string `eru:"required"`
}

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func (apiGateway *ApiGateway) Send(ctx context.Context, msg string, templateId string, params url.Values) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("Send - Start")
	resBody := make(map[string]interface{})
	logs.WithContext(ctx).Info(params.Get("silent"))
	if params.Get("silent") == "true" {
		msg = "777777 "
	} else {
		//TODO change it to utils
		req, err := http.NewRequest(apiGateway.GatewayMethod, apiGateway.GatewayUrl, nil)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}

		for k, v := range apiGateway.QueryParams {
			if k == "msg" {
				params.Set(v, msg)
			}
			if k == "templateId" {
				params.Set(v, templateId)
			}
		}
		req.URL.RawQuery = params.Encode()
		response, err := utils.ExecuteHttp(req.Context(), req)
		// response, err := httpClient.Do(req)
		defer response.Body.Close()
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}

		if err = json.NewDecoder(response.Body).Decode(&resBody); err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
	}
	//todo to remove this hard coded call and make it configurable
	saveOtp := make(map[string][]map[string]string)
	saveDoc := make(map[string]string)
	saveDoc["mobile_no"] = params.Get("to")
	saveDoc["otp"] = strings.SplitAfter(msg, " ")[0]
	saveOtp["docs"] = append(saveOtp["docs"], saveDoc)
	saveOtpReqBody, err := json.Marshal(saveOtp)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}

	eruQueriesURL := os.Getenv("ERUQUERIESURL")
	if eruQueriesURL == "" {
		eruQueriesURL = "http://localhost:8087"
	}
	saveOtpResp, err := http.Post(fmt.Sprint(eruQueriesURL, "/store/smartvalues/myquery/execute/save_otp"), "application/json", bytes.NewBuffer(saveOtpReqBody))
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
	defer saveOtpResp.Body.Close()

	return resBody, nil
}

func (apiGateway *ApiGateway) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &apiGateway)
	if err != nil {
		err = errors.New(fmt.Sprint("error json.Unmarshal(*rj, &smsGateway) ", err.Error()))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
