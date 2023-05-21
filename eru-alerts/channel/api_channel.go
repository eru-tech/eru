package channel

import (
	"context"
	"encoding/json"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/routes"
	"net/http"
	"net/url"
)

type ApiChannel struct {
	Channel
	Api routes.Route
}

var httpClient = http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func (apiChannel *ApiChannel) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {
	logs.WithContext(ctx).Debug("MakeFromJson - Start")
	err := json.Unmarshal(*rj, &apiChannel)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}
func (apiChannel *ApiChannel) Execute(ctx context.Context, r *http.Request, mt MessageTemplate) (response *http.Response, err error) {
	logs.WithContext(ctx).Debug("Execute - Start")
	msg, msgErr := apiChannel.ProcessMessageTemplate(ctx, r, mt)
	if msgErr != nil {
		return nil, msgErr
	}
	logs.WithContext(ctx).Debug(fmt.Sprint("msg = ", msg))

	trReqVars := &routes.TemplateVars{}
	if trReqVars.Vars == nil {
		trReqVars.Vars = make(map[string]interface{})
	}
	trReqVars.Vars["Msg"] = msg.Msg
	trReqVars.Vars["TemplateId"] = mt.TemplateId
	response, _, err = apiChannel.Api.Execute(ctx, r, r.URL.Path, false, "", trReqVars, 1)
	return response, nil
}
func (apiChannel *ApiChannel) Send(ctx context.Context, msg string, templateId string, params url.Values) (map[string]interface{}, error) {
	logs.WithContext(ctx).Debug("Send - Start")

	/*req, err := http.NewRequest(apiChannel.ApiMethod, apiChannel.ApiUrl, nil)
		if err != nil {
		}

		for k, v := range apiChannel.QueryParams {
			if k == "msg" {
				params.Set(v, msg)
			}
			if k == "templateId" {
				params.Set(v, templateId)
			}
		}
		req.URL.RawQuery = params.Encode()
		response, err := httpClient.Do(req)
		defer response.Body.Close()
		if err != nil {
			return nil, err
		}

		if err = json.NewDecoder(response.Body).Decode(&resBody); err != nil {
			return nil, err
		}

		//todo to remove this hard coded call and make it configurable
		saveOtp := make(map[string][]map[string]string)
		saveDoc := make(map[string]string)
		saveDoc["mobile_no"] = params.Get("to")
		saveDoc["otp"] = strings.SplitAfter(msg, " ")[0]
		saveOtp["docs"] = append(saveOtp["docs"], saveDoc)
		saveOtpReqBody, err := json.Marshal(saveOtp)
		if err != nil {
			return nil, err
		}

		eruQueriesURL := os.Getenv("ERUQUERIESURL")
		if eruQueriesURL == "" {
			eruQueriesURL = "http://localhost:8087"
		}
		saveOtpResp, err := http.Post(fmt.Sprint(eruQueriesURL, "/store/smartvalues/myquery/execute/save_otp"), "application/json", bytes.NewBuffer(saveOtpReqBody))
		if err != nil {
			return nil, err
		}
		defer saveOtpResp.Body.Close()

		return resBody, nil
	}

	func (apiChannel *ApiChannel) MakeFromJson(rj *json.RawMessage) error {
		err := json.Unmarshal(*rj, &apiChannel)
		if err != nil {
			return err
		}
		return nil
	*/
	return nil, nil
}
