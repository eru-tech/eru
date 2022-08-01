package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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

func (apiGateway *ApiGateway) Send(msg string, templateId string, params url.Values) (map[string]interface{}, error) {
	log.Println("inside ApiGateway Send")
	resBody := make(map[string]interface{})
	log.Println(params.Get("silent"))
	if params.Get("silent") == "true" {
		log.Println("inside params.Get(\"silent\")")
		msg = "777777 "
	} else {
		req, err := http.NewRequest(apiGateway.GatewayMethod, apiGateway.GatewayUrl, nil)
		if err != nil {
			log.Println(err)
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
	}
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

func (apiGateway *ApiGateway) MakeFromJson(rj *json.RawMessage) error {
	log.Println("inside ApiGateway MakeFromJson")
	err := json.Unmarshal(*rj, &apiGateway)
	if err != nil {
		log.Print("error json.Unmarshal(*rj, &smsGateway)")
		log.Print(err)
		return err
	}
	return nil
}
