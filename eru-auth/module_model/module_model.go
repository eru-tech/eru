package module_model

import (
	"context"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/gateway"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"strings"
)

type ModuleProjectI interface {
	AddGateway(ctx context.Context, gatewayObj gateway.GatewayI)
	AddAuth(ctx context.Context, authObj auth.AuthI)
	RemoveAuth(ctx context.Context, authType string) error
}

type Project struct {
	ProjectId        string `eru:"required"`
	Gateways         map[string]gateway.GatewayI
	MessageTemplates map[string]MessageTemplate
	Auth             map[string]auth.AuthI
}

/*
	type SmsGateway struct {
		GatewayName   string `eru:"required"`
		GatewayUrl    string `eru:"required"`
		GatewayMethod string `eru:"required"`
		Allocation    int    `eru:"required"`
	}

	type EmailGateway struct {
		GatewayName   string `eru:"required"`
		GatewayUrl    string `eru:"required"`
		GatewayMethod string `eru:"required"`
		Allocation    int    `eru:"required"`
	}
*/
type MessageTemplate struct {
	GatewayName  string `eru:"required"`
	TemplateType string `eru:"required"`
	TemplateName string
	TemplateId   string `eru:"required"`
	TemplateText string `eru:"required"`
}

func (prg *Project) AddGateway(ctx context.Context, gatewayObjI gateway.GatewayI) error {
	logs.WithContext(ctx).Debug("AddGateway - Start")
	gatewayName, err := gatewayObjI.GetAttribute("GatewayName")
	if err != nil {
		return err
	}
	gatewayType, err := gatewayObjI.GetAttribute("GatewayType")
	if err != nil {
		return err
	}
	channel, err := gatewayObjI.GetAttribute("Channel")
	if err != nil {
		return err
	}
	gKey := fmt.Sprint(gatewayName.(string), "_", gatewayType.(string), "_", channel.(string))
	prg.Gateways[gKey] = gatewayObjI
	return nil
}

func (prg *Project) AddAuth(ctx context.Context, authType string, authObjI auth.AuthI) error {
	logs.WithContext(ctx).Debug("AddAuth - Start")
	prg.Auth[authType] = authObjI
	return nil
}
func (prg *Project) RemoveAuth(ctx context.Context, authType string) error {
	logs.WithContext(ctx).Debug("RemoveAuth - Start")
	delete(prg.Auth, authType)
	return nil
}

func (mt *MessageTemplate) GetMessageText(vars string) string {
	text := mt.TemplateText
	for _, v := range strings.Split(vars, ",") {
		text = strings.Replace(text, "{#var#}", v, 1)
	}
	return text
}
