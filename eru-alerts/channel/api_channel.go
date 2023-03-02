package channel

import (
	"encoding/json"
	"github.com/eru-tech/eru/eru-routes/routes"
	"log"
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

func (apiChannel *ApiChannel) MakeFromJson(rj *json.RawMessage) error {
	log.Println("inside ApiGateway MakeFromJson")
	err := json.Unmarshal(*rj, &apiChannel)
	if err != nil {
		log.Print("error json.Unmarshal(*rj, &smsGateway)")
		log.Print(err)
		return err
	}
	return nil
}
func (apiChannel *ApiChannel) Execute(r *http.Request, mt MessageTemplate) (response *http.Response, err error) {

	msg, msgErr := apiChannel.ProcessMessageTemplate(r, mt)
	if msgErr != nil {
		return nil, msgErr
	}
	log.Println("msg = ", msg)

	trReqVars := &routes.TemplateVars{}
	if trReqVars.Vars == nil {
		trReqVars.Vars = make(map[string]interface{})
	}
	trReqVars.Vars["Msg"] = msg.Msg
	trReqVars.Vars["TemplateId"] = mt.TemplateId
	response, _, err = apiChannel.Api.Execute(r, r.URL.Path, false, "", trReqVars, 1)
	return response, nil
}
func (apiChannel *ApiChannel) Send(msg string, templateId string, params url.Values) (map[string]interface{}, error) {
	log.Println("inside ApiChannel Send")

	/*req, err := http.NewRequest(apiChannel.ApiMethod, apiChannel.ApiUrl, nil)
		if err != nil {
			log.Println(err)
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
		log.Println(req)
		response, err := httpClient.Do(req)
		defer response.Body.Close()
		if err != nil {
			log.Println("---")
			log.Println(err)
			return nil, err
		}

		if err = json.NewDecoder(response.Body).Decode(&resBody); err != nil {
			log.Print(err)
			return nil, err
		}
		log.Println("==================")
		log.Println(resBody)

		//todo to remove this hard coded call and make it configurable
		saveOtp := make(map[string][]map[string]string)
		saveDoc := make(map[string]string)
		saveDoc["mobile_no"] = params.Get("to")
		saveDoc["otp"] = strings.SplitAfter(msg, " ")[0]
		saveOtp["docs"] = append(saveOtp["docs"], saveDoc)
		log.Println(saveOtp)
		saveOtpReqBody, err := json.Marshal(saveOtp)
		if err != nil {
			log.Print(err)
			return nil, err
		}

		eruQueriesURL := os.Getenv("ERUQUERIESURL")
		if eruQueriesURL == "" {
			eruQueriesURL = "http://localhost:8087"
		}
		log.Print(saveOtpReqBody)
		saveOtpResp, err := http.Post(fmt.Sprint(eruQueriesURL, "/store/smartvalues/myquery/execute/save_otp"), "application/json", bytes.NewBuffer(saveOtpReqBody))
		if err != nil {
			log.Print(err)
			return nil, err
		}
		defer saveOtpResp.Body.Close()

		return resBody, nil
	}

	func (apiChannel *ApiChannel) MakeFromJson(rj *json.RawMessage) error {
		log.Println("inside ApiChannel MakeFromJson")
		err := json.Unmarshal(*rj, &apiChannel)
		if err != nil {
			log.Print("error json.Unmarshal(*rj, &apiChannel)")
			log.Print(err)
			return err
		}
		return nil
	*/
	return nil, nil
}
