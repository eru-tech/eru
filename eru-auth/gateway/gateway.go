package gateway

import (
	"encoding/json"
	"errors"
	"net/url"
)

type GatewayI interface {
	Send(msg string, templateId string, params url.Values) (map[string]interface{}, error)
	GetAttribute(attributeName string) (attributeValue interface{}, err error)
	MakeFromJson(rj *json.RawMessage) error
}

type Gateway struct {
	GatewayType string `eru:"required"`
	Channel     string `eru:"required"`
	GatewayName string `eru:"required"`
	Allocation  int    `eru:"required"`
}

func (gateway *Gateway) Send(msg string, templateId string, params url.Values) (map[string]interface{}, error) {
	return nil, errors.New("Send Method not implemented")
}

func (gateway *Gateway) GetAttribute(attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "GatewayName":
		return gateway.GatewayName, nil
	case "GatewayType":
		return gateway.GatewayType, nil
	case "Channel":
		return gateway.Channel, nil
	default:
		return nil, errors.New("Attribute not found")
	}
}

func GetGateway(gatewayType string) GatewayI {
	switch gatewayType {
	case "API":
		return new(ApiGateway)
	case "SMTP":
		return new(SmtpGateway)
	default:
		return nil
	}
	return nil
}
