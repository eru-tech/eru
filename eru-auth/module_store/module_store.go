package module_store

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-auth/auth"
	"github.com/eru-tech/eru/eru-auth/gateway"
	"github.com/eru-tech/eru/eru-auth/module_model"
	"github.com/eru-tech/eru/eru-store/store"
	"log"
)

type StoreHolder struct {
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveProject(projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(projectId string, realStore ModuleStoreI) error
	GetProjectConfig(projectId string) (*module_model.Project, error)
	GetProjectList() []map[string]interface{}
	//SaveSmsGateway(projectId string, gatewayName string, smsGateway module_model.SmsGateway, realStore ModuleStoreI) error
	//RemoveSmsGateway(projectId string, gatewayName string, realStore ModuleStoreI) error
	//SaveEmailGateway(projectId string, gatewayName string, emailGateway module_model.EmailGateway, realStore ModuleStoreI) error
	//RemoveEmailGateway(projectId string, gatewayName string, realStore ModuleStoreI) error
	SaveMessageTemplate(projectId string, messageTemplate module_model.MessageTemplate, realStore ModuleStoreI) error
	RemoveMessageTemplate(projectId string, templateId string, realStore ModuleStoreI) error
	SaveGateway(gatewayObj gateway.GatewayI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveGateway(gatewayName string, gatewayType string, channel string, projectId string, realStore ModuleStoreI) error
	GetGatewayFromType(gatewayType string, channel string, projectId string) (gateway.GatewayI, error)
	GetMessageTemplate(gatewayName string, projectId string, templateType string) (module_model.MessageTemplate, error)
	SaveAuth(authObj auth.AuthI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveAuth(authType string, projectId string, realStore ModuleStoreI) error
	GetAuth(projectId string, authName string) (auth.AuthI, error)
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

func (ms *ModuleStore) SaveProject(projectId string, realStore ModuleStoreI, persist bool) error {
	//TODO to handle edit project once new project attributes are finalized
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
			log.Print("SaveStore called from SaveProject")
			return realStore.SaveStore("", realStore)
		} else {
			return nil
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " already exists"))
	}
}

func (ms *ModuleStore) RemoveProject(projectId string, realStore ModuleStoreI) error {
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		log.Print("SaveStore called from RemoveProject")
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetProjectConfig(projectId string) (*module_model.Project, error) {
	if _, ok := ms.Projects[projectId]; ok {
		//log.Println(store.Projects[projectId])
		return ms.Projects[projectId], nil
	} else {
		return nil, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetProjectList() []map[string]interface{} {
	projects := make([]map[string]interface{}, len(ms.Projects))
	i := 0
	for k := range ms.Projects {
		project := make(map[string]interface{})
		project["projectName"] = k
		//project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
}

/*
func (ms *ModuleStore) SaveSmsGateway(projectId string, gatewayName string, smsGateway module_model.SmsGateway, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	if ms.Projects[projectId].SmsGateways == nil {
		ms.Projects[projectId].SmsGateways = make(map[string]module_model.SmsGateway)
	}
	ms.Projects[projectId].SmsGateways[gatewayName] = smsGateway
	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) RemoveSmsGateway(projectId string, gatewayName string, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	if _, ok := ms.Projects[projectId].SmsGateways[gatewayName]; ok {
		delete(ms.Projects[projectId].SmsGateways, gatewayName)
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Smsgateway ", gatewayName, " does not exists"))
	}
}

func (ms *ModuleStore) SaveEmailGateway(projectId string, gatewayName string, emailGateway module_model.EmailGateway, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	if ms.Projects[projectId].EmailGateways == nil {
		ms.Projects[projectId].EmailGateways = make(map[string]module_model.EmailGateway)
	}
	ms.Projects[projectId].EmailGateways[gatewayName] = emailGateway
	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) RemoveEmailGateway(projectId string, gatewayName string, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	if _, ok := ms.Projects[projectId].EmailGateways[gatewayName]; ok {
		delete(ms.Projects[projectId].EmailGateways, gatewayName)
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Emailgateway ", gatewayName, " does not exists"))
	}
}
*/
func (ms *ModuleStore) SaveMessageTemplate(projectId string, messageTemplate module_model.MessageTemplate, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	if ms.Projects[projectId].MessageTemplates == nil {
		ms.Projects[projectId].MessageTemplates = make(map[string]module_model.MessageTemplate)
	}
	templateName := fmt.Sprint(messageTemplate.GatewayName, "_", messageTemplate.TemplateType)
	ms.Projects[projectId].MessageTemplates[templateName] = messageTemplate
	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) RemoveMessageTemplate(projectId string, templateName string, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	if _, ok := ms.Projects[projectId].MessageTemplates[templateName]; ok {
		delete(ms.Projects[projectId].MessageTemplates, templateName)
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("MessageTemplates ", templateName, " does not exists"))
	}
}

func (ms *ModuleStore) SaveGateway(gatewayObj gateway.GatewayI, projectId string, realStore ModuleStoreI, persist bool) error {
	log.Println("inside SaveGateway")
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return err
	}
	err = prj.AddGateway(gatewayObj)
	if persist == true {
		return realStore.SaveStore("", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveGateway(gatewayName string, gatewayType string, channel string, projectId string, realStore ModuleStoreI) error {
	if prg, ok := ms.Projects[projectId]; ok {
		gKey := fmt.Sprint(gatewayName, "_", gatewayType, "_", channel)
		if _, ok := prg.Gateways[gKey]; ok {
			delete(prg.Gateways, gKey)
			log.Print("SaveStore called from RemoveGateway")
			return realStore.SaveStore("", realStore)
		} else {
			return errors.New(fmt.Sprint("Gayeway ", gKey, " does not exists"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetGatewayFromType(gatewayType string, channel string, projectId string) (gateway.GatewayI, error) {

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
			return nil, errors.New(fmt.Sprint("No Gateways Defined"))
		}
	} else {
		return nil, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
	return nil, errors.New(fmt.Sprint("Gateway ", gatewayType, " not found"))
}

func (ms *ModuleStore) GetMessageTemplate(gatewayName string, projectId string, templateType string) (mt module_model.MessageTemplate, err error) {
	if prg, ok := ms.Projects[projectId]; ok {
		if prg.MessageTemplates != nil {
			for k, v := range prg.MessageTemplates {
				if k == fmt.Sprint(gatewayName, "_", templateType) {
					return v, nil
				}
			}
		} else {
			return mt, errors.New(fmt.Sprint("No Message Templates Defined"))
		}
	} else {
		return mt, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
	return mt, errors.New(fmt.Sprint("Message Template ", fmt.Sprint(gatewayName, "_", templateType), " not found"))
}
func (ms *ModuleStore) SaveAuth(authObj auth.AuthI, projectId string, realStore ModuleStoreI, persist bool) error {
	log.Println("inside SaveAuth")
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return err
	}
	authName, err := authObj.GetAttribute("AuthName")
	if err != nil {
		log.Print(err)
		return err
	}
	if persist == true {
		err = authObj.PerformPreSaveTask()
		if err != nil {
			log.Print(err)
			return err
		}
	}

	err = prj.AddAuth(authName.(string), authObj)
	if err != nil {
		log.Print(err)
		return err
	}

	if persist == true {
		return realStore.SaveStore("", realStore)
	}
	return nil
}
func (ms *ModuleStore) RemoveAuth(authName string, projectId string, realStore ModuleStoreI) (err error) {
	if prg, ok := ms.Projects[projectId]; ok {
		if authObj, ok := prg.Auth[authName]; ok {
			err = authObj.PerformPreDeleteTask()
			if err != nil {
				log.Println(err)
				return
			}
		} else {
			return errors.New(fmt.Sprint("Auth ", authName, " does not exists"))
		}
		prg.RemoveAuth(authName)
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
	return realStore.SaveStore("", realStore)
}

func (ms *ModuleStore) GetAuth(projectId string, authName string) (auth.AuthI, error) {
	if prg, ok := ms.Projects[projectId]; ok {
		if prg.Auth != nil {
			for k, v := range prg.Auth {
				if k == authName {
					return v, nil
				}
			}
		} else {
			return nil, errors.New(fmt.Sprint("No Auth Defined for the project : ", projectId))
		}
	} else {
		return nil, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
	return nil, errors.New(fmt.Sprint("Auth ", authName, " not found"))
}
