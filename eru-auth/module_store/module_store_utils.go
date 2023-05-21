package module_store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	authtype "github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/gateway"
	"github.com/eru-tech/eru/eru-auth/module_model"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-store/store"
)

func (ms *ModuleStore) checkProjectExists(ctx context.Context, projectId string) error {
	logs.WithContext(ctx).Debug("checkProjectExists - Start")
	_, ok := ms.Projects[projectId]
	if !ok {
		err := errors.New(fmt.Sprint("project ", projectId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil
}

func UnMarshalStore(ctx context.Context, b []byte, msi ModuleStoreI) error {
	logs.WithContext(ctx).Debug("UnMarshalStore - Start")
	var storeMap map[string]*json.RawMessage
	err := json.Unmarshal(b, &storeMap)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}

	var vars map[string]*store.Variables
	if _, ok := storeMap["Variables"]; ok {
		err = json.Unmarshal(*storeMap["Variables"], &vars)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		msi.SetVars(ctx, vars)
	}

	var prjs map[string]*json.RawMessage
	if _, ok := storeMap["projects"]; ok {

		err = json.Unmarshal(*storeMap["projects"], &prjs)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		for prj, prjJson := range prjs {
			err = msi.SaveProject(ctx, prj, nil, false)
			if err != nil {
				return err
			}
			var prjObjs map[string]*json.RawMessage
			err = json.Unmarshal(*prjJson, &prjObjs)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			p, e := msi.GetProjectConfig(ctx, prj)
			if e != nil {
				return err
			}
			var messageTemplates map[string]module_model.MessageTemplate
			err = json.Unmarshal(*prjObjs["MessageTemplates"], &messageTemplates)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			p.MessageTemplates = messageTemplates

			var gateways map[string]*json.RawMessage
			err = json.Unmarshal(*prjObjs["Gateways"], &gateways)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for gatewayKey, gatewayJson := range gateways {
				logs.WithContext(ctx).Info(fmt.Sprint("gatewayKey === ", gatewayKey))
				var gatewayObj map[string]*json.RawMessage
				err = json.Unmarshal(*gatewayJson, &gatewayObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var gatewayType string
				err = json.Unmarshal(*gatewayObj["GatewayType"], &gatewayType)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				gatewayI := gateway.GetGateway(gatewayType)
				err = gatewayI.MakeFromJson(ctx, gatewayJson)
				if err == nil {
					err = msi.SaveGateway(ctx, gatewayI, prj, nil, false)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}

			var auths map[string]*json.RawMessage
			if _, ok = prjObjs["Auth"]; ok {
				err = json.Unmarshal(*prjObjs["Auth"], &auths)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				for authKey, authJson := range auths {
					logs.WithContext(ctx).Info(fmt.Sprint("authKey === ", authKey))
					var authObj map[string]*json.RawMessage
					err = json.Unmarshal(*authJson, &authObj)
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						return err
					}
					var authType string
					if at, ok := authObj["AuthType"]; ok {
						err = json.Unmarshal(*at, &authType)
						if err != nil {
							logs.WithContext(ctx).Error(err.Error())
							return err
						}
					}
					logs.WithContext(ctx).Info(fmt.Sprint("authType = ", authType))
					authI := authtype.GetAuth(authType)
					err = authI.MakeFromJson(ctx, authJson)
					if err == nil {
						err = msi.SaveAuth(ctx, authI, prj, msi, false)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				}
			}
		}
	}
	return nil
}

func GetStore(storeType string) ModuleStoreI {
	switch storeType {
	case "POSTGRES":
		return new(ModuleDbStore)
	case "STANDALONE":
		return new(ModuleFileStore)
	default:
		return nil
	}
}
