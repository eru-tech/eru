package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/routes"
	"html"
	"io"
	"net/http"
	"strings"
)

const (
	API_CHANNEL  = "API"
	SMTP_CHANNEL = "SMTP"
)

type MessageTemplate struct {
	TemplateType    string `eru:"required"`
	TemplateName    string `eru:"required"`
	TemplateId      string
	TemplateText    string `eru:"required"`
	TemplateSubject string
	ToUsers         string
	CcUsers         string
	BccUsers        string
}
type Message struct {
	Subject string
	Msg     string
	To      []string
	Cc      []string
	Bcc     []string
}

type ChannelI interface {
	Execute(ctx context.Context, r *http.Request, mt MessageTemplate) (response *http.Response, err error)
	GetAttribute(attributeName string) (attributeValue interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
	ProcessMessageTemplate(ctx context.Context, r *http.Request, mt MessageTemplate) (msg Message, err error)
}

type Channel struct {
	ChannelType string `eru:"required"`
	ChannelName string `eru:"required"`
}

func (channel *Channel) Execute(ctx context.Context, r *http.Request, mt MessageTemplate) (response *http.Response, err error) {
	return nil, errors.New("Execute Method not implemented")
}

func (channel *Channel) GetAttribute(attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "ChannelName":
		return channel.ChannelName, nil
	case "ChannelType":
		return channel.ChannelType, nil
	default:
		return nil, errors.New("Attribute not found")
	}
}

func GetChannel(channelType string) ChannelI {
	switch channelType {
	case API_CHANNEL:
		return new(ApiChannel)
	case SMTP_CHANNEL:
		return new(SmtpChannel)
	default:
		return nil
	}
	return nil
}

func (channel *Channel) ProcessMessageTemplate(ctx context.Context, r *http.Request, mt MessageTemplate) (msg Message, err error) {
	logs.WithContext(ctx).Debug("ProcessMessageTemplate - Start")
	r1, r1err := routes.CloneRequest(ctx, r)
	if r1err != nil {
		return Message{}, r1err
	}
	mtRoute := routes.Route{}
	mtRoute.RouteName = mt.TemplateName
	mtRoute.TargetHosts = append(mtRoute.TargetHosts, routes.TargetHost{})

	mtRoute.TransformRequest = fmt.Sprint("{\"msg\":\"", mt.TemplateText, "\"}")
	mtRoute.TransformResponse = fmt.Sprint("{\"res\":{{bytesToString (marshalJSON .Vars.Vars.Body)}},\"to\":\"", mt.ToUsers, "\",\"cc\":\"", mt.CcUsers, "\",\"bcc\":\"", mt.BccUsers, "\",\"subject\":\"", mt.TemplateSubject, "\"}")
	mtResp, _, mtRespErr := mtRoute.Execute(ctx, r, r.URL.Path, false, "", nil, 1)
	if mtRespErr != nil {
		return Message{}, mtRespErr
	}
	var res map[string]interface{}
	tmplBodyFromRes := json.NewDecoder(mtResp.Body)
	tmplBodyFromRes.DisallowUnknownFields()
	if err = tmplBodyFromRes.Decode(&res); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		body, readErr := io.ReadAll(mtResp.Body)
		if readErr != nil {
			err = readErr
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		tempBody := make(map[string]interface{})
		tempBody["msg"] = string(body)
		res = tempBody
	}
	msg = Message{}
	if msgMap, msgMapOk := res["res"].(map[string]interface{}); msgMapOk {

		msg.Msg = msgMap["msg"].(string)
	}
	msg.Subject = res["subject"].(string)
	msg.To = strings.Split(res["to"].(string), ",")
	msg.Cc = strings.Split(res["cc"].(string), ",")
	msg.Bcc = strings.Split(res["bcc"].(string), ",")

	mtRoute.TransformRequest = ""
	mtRoute.TransformResponse = fmt.Sprint("{\"msg\":\"", msg.Msg, "\"}")
	mtRespMsg, _, mtRespMsgErr := mtRoute.Execute(ctx, r1, r1.URL.Path, false, "", nil, 1)
	if mtRespMsgErr != nil {
		return Message{}, mtRespMsgErr
	}
	tmplBodyFromResMsg := json.NewDecoder(mtRespMsg.Body)
	tmplBodyFromResMsg.DisallowUnknownFields()
	if err = tmplBodyFromResMsg.Decode(&res); err != nil {
		logs.WithContext(ctx).Error(err.Error())
		body, readErr := io.ReadAll(mtRespMsg.Body)
		if readErr != nil {
			err = readErr
			logs.WithContext(ctx).Error(err.Error())
			return
		}
		tempBody := make(map[string]interface{})
		tempBody["msg"] = string(body)
		res = tempBody
	}
	if mt.TemplateType == "HTML" {
		msg.Msg = html.UnescapeString(res["msg"].(string))
	} else {
		msg.Msg = res["msg"].(string)
	}
	return msg, mtRespErr
}
