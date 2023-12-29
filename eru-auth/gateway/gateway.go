package gateway

import (
	"context"
	"encoding/json"
	"errors"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"net/url"
)

type GatewayI interface {
	Send(ctx context.Context, msg string, templateId string, params url.Values) (map[string]interface{}, error)
	GetAttribute(attributeName string) (attributeValue interface{}, err error)
	MakeFromJson(ctx context.Context, rj *json.RawMessage) error
}

type Gateway struct {
	GatewayType string `json:"gateway_type" eru:"required"`
	Channel     string `json:"channel" eru:"required"`
	GatewayName string `json:"gateway_name" eru:"required"`
	Allocation  int    `json:"allocation" eru:"required"`
}

func (gateway *Gateway) Send(ctx context.Context, msg string, templateId string, params url.Values) (map[string]interface{}, error) {
	err := errors.New("Send Method not implemented")
	logs.WithContext(ctx).Error(err.Error())
	return nil, err
}

func (gateway *Gateway) GetAttribute(attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "gateway_name":
		return gateway.GatewayName, nil
	case "gateway_type":
		return gateway.GatewayType, nil
	case "channel":
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
