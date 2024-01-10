package module_model

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/gateway"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-secret-manager/sm"
	"github.com/eru-tech/eru/eru-store/store"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
	"strings"
)

type StoreCompare struct {
	store.StoreCompare
	DeleteAuth   []string               `json:"delete_auth"`
	NewAuth      []string               `json:"new_auth"`
	MismatchAuth map[string]interface{} `json:"mismatch_auth"`
}

type ModuleProjectI interface {
	AddGateway(ctx context.Context, gatewayObj gateway.GatewayI)
	AddAuth(ctx context.Context, authObj auth.AuthI)
	RemoveAuth(ctx context.Context, authType string) error
	CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error)
}
type ExtendedProject struct {
	Project
	Variables     store.Variables `json:"variables"`
	SecretManager sm.SmStoreI     `json:"secret_manager"`
}

type Project struct {
	ProjectId        string                      `json:"project_id" eru:"required"`
	Gateways         map[string]gateway.GatewayI `json:"gateways"`
	MessageTemplates map[string]MessageTemplate  `json:"message_templates"`
	Auth             map[string]auth.AuthI       `json:"auth"`
	ProjectSettings  ProjectSettings             `json:"project_settings"`
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
	GatewayName  string `json:"gateway_name" eru:"required"`
	TemplateType string `json:"template_type" eru:"required"`
	TemplateName string `json:"template_name"`
	TemplateId   string `json:"template_id" eru:"required"`
	TemplateText string `json:"template_text" eru:"required"`
}

func (prj *Project) AddGateway(ctx context.Context, gatewayObjI gateway.GatewayI) error {
	logs.WithContext(ctx).Debug("AddGateway - Start")
	gatewayName, err := gatewayObjI.GetAttribute("gateway_name")
	if err != nil {
		return err
	}
	gatewayType, err := gatewayObjI.GetAttribute("gateway_type")
	if err != nil {
		return err
	}
	channel, err := gatewayObjI.GetAttribute("channel")
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

func (ePrj *ExtendedProject) CompareProject(ctx context.Context, compareProject ExtendedProject) (StoreCompare, error) {
	logs.WithContext(ctx).Debug("CompareProject - Start")
	storeCompare := StoreCompare{}
	storeCompare.CompareVariables(ctx, ePrj.Variables, compareProject.Variables)
	storeCompare.CompareSecretManager(ctx, ePrj.SecretManager, compareProject.SecretManager)

	var diffR utils.DiffReporter
	if !cmp.Equal(ePrj.ProjectSettings, compareProject.ProjectSettings, cmp.Reporter(&diffR)) {
		if storeCompare.MismatchSettings == nil {
			storeCompare.MismatchSettings = make(map[string]interface{})
		}
		storeCompare.MismatchSettings["settings"] = diffR.Output()
	}

	for _, ma := range ePrj.Auth {
		maNameI, _ := ma.GetAttribute(ctx, "auth_name")
		maName := maNameI.(string)
		var diffR utils.DiffReporter
		aFound := false
		for _, ca := range compareProject.Auth {
			caNameI, _ := ca.GetAttribute(ctx, "auth_name")
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
		caNameI, _ := ca.GetAttribute(ctx, "auth_name")
		caName := caNameI.(string)
		rFound := false
		for _, ma := range ePrj.Auth {
			maNameI, _ := ma.GetAttribute(ctx, "auth_name")
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

func (ePrj *ExtendedProject) UnmarshalJSON(b []byte) error {
	logs.Logger.Info("UnMarshal ExtendedProject - Start")
	ctx := context.Background()
	var ePrjMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &ePrjMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	projectId := ""
	if _, ok := ePrjMap["project_id"]; ok {
		if ePrjMap["project_id"] != nil {
			err = json.Unmarshal(*ePrjMap["project_id"], &projectId)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.ProjectId = projectId
		}
	}

	var ps ProjectSettings
	if _, ok := ePrjMap["project_settings"]; ok {
		if ePrjMap["project_settings"] != nil {
			err = json.Unmarshal(*ePrjMap["project_settings"], &ps)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.ProjectSettings = ps
		}
	}

	var vars store.Variables
	if _, ok := ePrjMap["variables"]; ok {
		if ePrjMap["variables"] != nil {
			err = json.Unmarshal(*ePrjMap["variables"], &vars)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.Variables = vars
		}
	}

	var mt map[string]MessageTemplate
	if _, ok := ePrjMap["message_templates"]; ok {
		if ePrjMap["message_templates"] != nil {
			err = json.Unmarshal(*ePrjMap["message_templates"], &mt)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			ePrj.MessageTemplates = mt
		}
	}

	var smObj map[string]*json.RawMessage
	var smJson *json.RawMessage
	if _, ok := ePrjMap["secret_manager"]; ok {
		if ePrjMap["secret_manager"] != nil {
			err = json.Unmarshal(*ePrjMap["secret_manager"], &smObj)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			err = json.Unmarshal(*ePrjMap["secret_manager"], &smJson)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}

			var smType string
			if _, stOk := smObj["sm_store_type"]; stOk {
				err = json.Unmarshal(*smObj["sm_store_type"], &smType)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				smI := sm.GetSm(smType)
				err = smI.MakeFromJson(ctx, smJson)
				if err == nil {
					ePrj.SecretManager = smI
				} else {
					return err
				}
			} else {
				logs.WithContext(ctx).Info("ignoring secret manager as sm_store_type attribute not found")
			}
		} else {
			logs.WithContext(ctx).Info("secret manager attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("secret manager attribute not found in store")
	}

	var auths map[string]*json.RawMessage
	if _, ok := ePrjMap["auth"]; ok {
		if ePrjMap["auth"] != nil {
			err = json.Unmarshal(*ePrjMap["auth"], &auths)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for _, authJson := range auths {
				var authObj map[string]*json.RawMessage
				err = json.Unmarshal(*authJson, &authObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var authType string
				err = json.Unmarshal(*authObj["auth_type"], &authType)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var authName string
				err = json.Unmarshal(*authObj["auth_name"], &authName)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				authI := auth.GetAuth(authType)
				err = authI.MakeFromJson(ctx, authJson)
				if err == nil {
					if ePrj.Auth == nil {
						ePrj.Auth = make(map[string]auth.AuthI)
					}
					ePrj.Auth[authName] = authI
				} else {
					return err
				}
			}
		} else {
			logs.WithContext(ctx).Info("auth attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("auth attribute not found in store")
	}

	var gateways map[string]*json.RawMessage
	if _, ok := ePrjMap["gateways"]; ok {
		if ePrjMap["gateways"] != nil {
			err = json.Unmarshal(*ePrjMap["gateways"], &gateways)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for _, gatewayJson := range gateways {
				var gatewayObj map[string]*json.RawMessage
				err = json.Unmarshal(*gatewayJson, &gatewayObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var gatewayType string
				err = json.Unmarshal(*gatewayObj["gateway_type"], &gatewayType)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var gatewayName string
				err = json.Unmarshal(*gatewayObj["gateway_name"], &gatewayName)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var channel string
				err = json.Unmarshal(*gatewayObj["channel"], &channel)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}

				gKey := fmt.Sprint(gatewayName, "_", gatewayType, "_", channel)

				gatewayI := gateway.GetGateway(gatewayType)
				err = gatewayI.MakeFromJson(ctx, gatewayJson)
				if err == nil {
					if ePrj.Gateways == nil {
						ePrj.Gateways = make(map[string]gateway.GatewayI)
					}
					ePrj.Gateways[gKey] = gatewayI
				} else {
					return err
				}
			}
		} else {
			logs.WithContext(ctx).Info("gateway attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("gateway attribute not found in store")
	}

	return nil
}
