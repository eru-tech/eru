package module_model

import (
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/gateway"
	"log"
	"strings"
)

type ModuleProjectI interface {
	AddGateway(gatewayObj gateway.GatewayI)
	AddAuth(authObj auth.AuthI)
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

func (prg *Project) AddGateway(gatewayObjI gateway.GatewayI) error {
	log.Println("inside AddGateway")
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
	log.Print(gKey)
	prg.Gateways[gKey] = gatewayObjI
	log.Println(prg)
	return nil
}

func (prg *Project) AddAuth(authType string, authObjI auth.AuthI) error {
	log.Println("inside AddAuth")
	prg.Auth[authType] = authObjI
	return nil
}
func (prg *Project) RemoveAuth(authType string) error {
	log.Println("inside RemoveAuth")
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
