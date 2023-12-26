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
	"github.com/eru-tech/eru/eru-repos/repos"
	"github.com/eru-tech/eru/eru-secret-manager/sm"
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
	if _, ok := storeMap["variables"]; ok {
		if storeMap["variables"] != nil {
			err = json.Unmarshal(*storeMap["variables"], &vars)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			msi.SetVars(ctx, vars)
		}
	}

	var prjSm map[string]*json.RawMessage
	if _, ok := storeMap["secret_manager"]; ok {
		if storeMap["secret_manager"] != nil {
			err = json.Unmarshal(*storeMap["secret_manager"], &prjSm)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for prj, smJson := range prjSm {
				var smObj map[string]*json.RawMessage
				err = json.Unmarshal(*smJson, &smObj)
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
						err = msi.SaveSm(ctx, prj, smI, msi, false)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				} else {
					logs.WithContext(ctx).Info("ignoring secret manager as sm_store_type attribute not found")
				}
			}
		} else {
			logs.WithContext(ctx).Info("secret manager attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("secret manager attribute not found in store")
	}

	var prjRepo map[string]*json.RawMessage
	if _, ok := storeMap["repos"]; ok {
		if storeMap["repos"] != nil {
			err = json.Unmarshal(*storeMap["repos"], &prjRepo)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return err
			}
			for prj, repoJson := range prjRepo {
				var repoObj map[string]*json.RawMessage
				err = json.Unmarshal(*repoJson, &repoObj)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					return err
				}
				var repoType string
				if _, rtOk := repoObj["repo_type"]; rtOk {
					err = json.Unmarshal(*repoObj["repo_type"], &repoType)
					if err != nil {
						logs.WithContext(ctx).Error(err.Error())
						return err
					}
					repoI := repos.GetRepo(repoType)
					err = repoI.MakeFromJson(ctx, repoJson)
					if err == nil {
						err = msi.SaveRepo(ctx, prj, repoI, msi, false)
						if err != nil {
							return err
						}
					} else {
						return err
					}
				} else {
					logs.WithContext(ctx).Info("ignoring repo as repo type not found")
				}
			}
		} else {
			logs.WithContext(ctx).Info("repos attribute is nil")
		}
	} else {
		logs.WithContext(ctx).Info("repos attribute not found in store")
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
