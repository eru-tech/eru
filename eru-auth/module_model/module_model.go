package module_model

import (
	"context"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/gateway"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
	"strings"
)

type StoreCompare struct {
	DeleteAuth   []string
	NewAuth      []string
	MismatchAuth map[string]interface{}
}

type ModuleProjectI interface {
	AddGateway(ctx context.Context, gatewayObj gateway.GatewayI)
	AddAuth(ctx context.Context, authObj auth.AuthI)
	RemoveAuth(ctx context.Context, authType string) error
	CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error)
}

type Project struct {
	ProjectId        string `eru:"required"`
	Gateways         map[string]gateway.GatewayI
	MessageTemplates map[string]MessageTemplate
	Auth             map[string]auth.AuthI
	ProjectSettings  ProjectSettings `json:"project_settings"`
}
type ProjectSettings struct {
	ClaimsKey string `json:"claims_key" eru:"required"`
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

func (prj *Project) AddGateway(ctx context.Context, gatewayObjI gateway.GatewayI) error {
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
	prj.Gateways[gKey] = gatewayObjI
	return nil
}

func (prj *Project) AddAuth(ctx context.Context, authType string, authObjI auth.AuthI) error {
	logs.WithContext(ctx).Debug("AddAuth - Start")
	prj.Auth[authType] = authObjI
	return nil
}
func (prj *Project) RemoveAuth(ctx context.Context, authType string) error {
	logs.WithContext(ctx).Debug("RemoveAuth - Start")
	delete(prj.Auth, authType)
	return nil
}

func (mt *MessageTemplate) GetMessageText(vars string) string {
	text := mt.TemplateText
	for _, v := range strings.Split(vars, ",") {
		text = strings.Replace(text, "{#var#}", v, 1)
	}
	return text
}

func (prj *Project) CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error) {
	logs.WithContext(ctx).Debug("CompareProject - Start")
	storeCompare := StoreCompare{}
	for _, ma := range prj.Auth {
		maNameI, _ := ma.GetAttribute(ctx, "AuthName")
		maName := maNameI.(string)
		var diffR utils.DiffReporter
		aFound := false
		for _, ca := range compareProject.Auth {
			caNameI, _ := ca.GetAttribute(ctx, "AuthName")
			caName := caNameI.(string)
			if maName == caName {
				aFound = true
				if !cmp.Equal(ma, ca, cmp.Reporter(&diffR)) {
					if storeCompare.MismatchAuth == nil {
						storeCompare.MismatchAuth = make(map[string]interface{})
					}
					storeCompare.MismatchAuth[maName] = diffR.Output()
				}
				break
			}
		}
		if !aFound {
			storeCompare.DeleteAuth = append(storeCompare.DeleteAuth, maName)
		}
	}
	for _, ca := range compareProject.Auth {
		caNameI, _ := ca.GetAttribute(ctx, "AuthName")
		caName := caNameI.(string)
		rFound := false
		for _, ma := range prj.Auth {
			maNameI, _ := ma.GetAttribute(ctx, "AuthName")
			maName := maNameI.(string)
			if maName == caName {
				rFound = true
				break
			}
		}
		if !rFound {
			storeCompare.NewAuth = append(storeCompare.NewAuth, caName)
		}
	}
	return storeCompare, nil
}
