package module_store

import (
	"context"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/gateway"
	"github.com/eru-tech/eru/eru-auth/module_model"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-store/store"
)

type StoreHolder struct {
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SaveMessageTemplate(ctx context.Context, projectId string, messageTemplate module_model.MessageTemplate, realStore ModuleStoreI) error
	RemoveMessageTemplate(ctx context.Context, projectId string, templateId string, realStore ModuleStoreI) error
	SaveGateway(ctx context.Context, gatewayObj gateway.GatewayI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveGateway(ctx context.Context, gatewayName string, gatewayType string, channel string, projectId string, realStore ModuleStoreI) error
	GetGatewayFromType(ctx context.Context, gatewayType string, channel string, projectId string) (gateway.GatewayI, error)
	GetMessageTemplate(ctx context.Context, gatewayName string, projectId string, templateType string) (module_model.MessageTemplate, error)
	SaveAuth(ctx context.Context, authObj auth.AuthI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveAuth(ctx context.Context, authType string, projectId string, realStore ModuleStoreI) error
	GetAuth(ctx context.Context, projectId string, authName string) (auth.AuthI, error)
}

type ModuleStore struct {
	Projects map[string]*module_model.Project `json:"projects"` //ProjectId is the key
}

type ModuleFileStore struct {
	store.FileStore
	ModuleStore
}
type ModuleDbStore struct {
	store.DbStore
	ModuleStore
}

func (ms *ModuleStore) SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error {
	//TODO to handle edit project once new project attributes are finalized
	logs.WithContext(ctx).Debug("SaveProject - Start")
	if _, ok := ms.Projects[projectId]; !ok {
		project := new(module_model.Project)
		project.ProjectId = projectId
		if ms.Projects == nil {
			ms.Projects = make(map[string]*module_model.Project)
		}
		if project.Gateways == nil {
			project.Gateways = make(map[string]gateway.GatewayI)
		}
		//if project.SmsGateways == nil {
		//	project.SmsGateways = make(map[string]module_model.SmsGateway)
		//}
		//if project.EmailGateways == nil {
		//	project.EmailGateways = make(map[string]module_model.EmailGateway)
		//}
		if project.MessageTemplates == nil {
			project.MessageTemplates = make(map[string]module_model.MessageTemplate)
		}
		if project.Auth == nil {
			project.Auth = make(map[string]auth.AuthI)
		}

		ms.Projects[projectId] = project
		if persist == true {
			logs.WithContext(ctx).Info("SaveStore called from SaveProject")
			return realStore.SaveStore(ctx, "", realStore)
		} else {
			return nil
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " already exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveProject - Start")
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		logs.WithContext(ctx).Info("SaveStore called from RemoveProject")
		return realStore.SaveStore(ctx, "", realStore)
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error) {
	logs.WithContext(ctx).Debug("GetProjectConfig - Start")
	if _, ok := ms.Projects[projectId]; ok {
		return ms.Projects[projectId], nil
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return nil, err
	}
}

func (ms *ModuleStore) GetProjectList(ctx context.Context) []map[string]interface{} {
	logs.WithContext(ctx).Debug("GetProjectList - Start")
	projects := make([]map[string]interface{}, len(ms.Projects))
	i := 0
	for k := range ms.Projects {
		project := make(map[string]interface{})
		project["projectName"] = k
		projects[i] = project
		i++
	}
	return projects
}

func (ms *ModuleStore) SaveMessageTemplate(ctx context.Context, projectId string, messageTemplate module_model.MessageTemplate, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveMessageTemplate - Start")
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		return err
	}
	if ms.Projects[projectId].MessageTemplates == nil {
		ms.Projects[projectId].MessageTemplates = make(map[string]module_model.MessageTemplate)
	}
	templateName := fmt.Sprint(messageTemplate.GatewayName, "_", messageTemplate.TemplateType)
	ms.Projects[projectId].MessageTemplates[templateName] = messageTemplate
	return realStore.SaveStore(ctx, "", realStore)
}

func (ms *ModuleStore) RemoveMessageTemplate(ctx context.Context, projectId string, templateName string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveMessageTemplate - Start")
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		return err
	}
	if _, ok := ms.Projects[projectId].MessageTemplates[templateName]; ok {
		delete(ms.Projects[projectId].MessageTemplates, templateName)
		return realStore.SaveStore(ctx, "", realStore)
	} else {
		err = errors.New(fmt.Sprint("MessageTemplates ", templateName, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) SaveGateway(ctx context.Context, gatewayObj gateway.GatewayI, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveGateway - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}
	err = prj.AddGateway(ctx, gatewayObj)
	if persist == true {
		return realStore.SaveStore(ctx, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveGateway(ctx context.Context, gatewayName string, gatewayType string, channel string, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveGateway - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		gKey := fmt.Sprint(gatewayName, "_", gatewayType, "_", channel)
		if _, ok := prg.Gateways[gKey]; ok {
			delete(prg.Gateways, gKey)
			logs.WithContext(ctx).Info("SaveStore called from RemoveGateway")
			return realStore.SaveStore(ctx, "", realStore)
		} else {
			err := errors.New(fmt.Sprint("Gateway ", gKey, " does not exists"))
			logs.WithContext(ctx).Info(err.Error())
			return err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetGatewayFromType(ctx context.Context, gatewayType string, channel string, projectId string) (gateway.GatewayI, error) {
	logs.WithContext(ctx).Debug("GetGatewayFromType - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		//todo to random selection of gateway based on allocation in case multiple gateway are defined
		if prg.Gateways != nil {
			for _, v := range prg.Gateways {
				gt, err := v.GetAttribute("GatewayType")
				ch, err := v.GetAttribute("Channel")
				if err != nil {
					return nil, err
				}
				if gt.(string) == gatewayType && ch.(string) == channel {
					return v, nil
				}
			}
		} else {
			err := errors.New(fmt.Sprint("No Gateways Defined"))
			logs.WithContext(ctx).Info(err.Error())
			return nil, err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return nil, err
	}
	err := errors.New(fmt.Sprint("Gateway ", gatewayType, " not found"))
	logs.WithContext(ctx).Info(err.Error())
	return nil, err
}

func (ms *ModuleStore) GetMessageTemplate(ctx context.Context, gatewayName string, projectId string, templateType string) (mt module_model.MessageTemplate, err error) {
	logs.WithContext(ctx).Debug("GetMessageTemplate - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		if prg.MessageTemplates != nil {
			for k, v := range prg.MessageTemplates {
				if k == fmt.Sprint(gatewayName, "_", templateType) {
					return v, nil
				}
			}
		} else {
			err = errors.New(fmt.Sprint("No Message Templates Defined"))
			logs.WithContext(ctx).Info(err.Error())
			return mt, err

		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return mt, err
	}
	err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	logs.WithContext(ctx).Info(err.Error())
	return mt, err
}
func (ms *ModuleStore) SaveAuth(ctx context.Context, authObj auth.AuthI, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveAuth - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}
	authName, err := authObj.GetAttribute(ctx, "AuthName")
	if err != nil {
		return err
	}
	if persist == true {
		err = authObj.PerformPreSaveTask(ctx)
		if err != nil {
			return err
		}
	}

	err = prj.AddAuth(ctx, authName.(string), authObj)
	if err != nil {
		return err
	}

	if persist == true {
		return realStore.SaveStore(ctx, "", realStore)
	}
	return nil
}
func (ms *ModuleStore) RemoveAuth(ctx context.Context, authName string, projectId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveAuth - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		if authObj, ok := prg.Auth[authName]; ok {
			err = authObj.PerformPreDeleteTask(ctx)
			if err != nil {
				return
			}
		} else {
			err = errors.New(fmt.Sprint("Auth ", authName, " does not exists"))
			logs.WithContext(ctx).Info(err.Error())
			return err
		}
		err = prg.RemoveAuth(ctx, authName)
		if err != nil {
			return err
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
	return realStore.SaveStore(ctx, "", realStore)
}

func (ms *ModuleStore) GetAuth(ctx context.Context, projectId string, authName string) (auth.AuthI, error) {
	logs.WithContext(ctx).Debug("GetAuth - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		if prg.Auth != nil {
			for k, v := range prg.Auth {
				if k == authName {
					return v, nil
				}
			}
		} else {
			err := errors.New(fmt.Sprint("No Auth Defined for the project : ", projectId))
			logs.WithContext(ctx).Info(err.Error())
			return nil, err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Info(err.Error())
		return nil, err
	}
	err := errors.New(fmt.Sprint("Auth ", authName, " not found"))
	logs.WithContext(ctx).Info(err.Error())
	return nil, err
}
