package module_store

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-alerts/channel"
	"github.com/eru-tech/eru/eru-alerts/module_model"
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
	SaveMessageTemplate(projectId string, messageTemplate channel.MessageTemplate, realStore ModuleStoreI) error
	RemoveMessageTemplate(projectId string, templateName string, realStore ModuleStoreI) error
	GetMessageTemplate(projectId string, templateId string, realStore ModuleStoreI) (channel.MessageTemplate, error)
	SaveChannel(channelObj channel.ChannelI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveChannel(channelName string, projectId string, realStore ModuleStoreI) error
	GetChannel(channelName string, projectId string, realStore ModuleStoreI) (channel.ChannelI, error)
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
		/*if project.Storages == nil {
			project.Storages = make(map[string]storage.StorageI)
		}*/
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

func (ms *ModuleStore) SaveMessageTemplate(projectId string, messageTemplate channel.MessageTemplate, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	if ms.Projects[projectId].MessageTemplates == nil {
		ms.Projects[projectId].MessageTemplates = make(map[string]channel.MessageTemplate)
	}
	ms.Projects[projectId].MessageTemplates[messageTemplate.TemplateName] = messageTemplate
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

func (ms *ModuleStore) GetMessageTemplate(projectId string, templateName string, realStore ModuleStoreI) (channel.MessageTemplate, error) {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return channel.MessageTemplate{}, err
	}
	if mt, ok := ms.Projects[projectId].MessageTemplates[templateName]; ok {
		return mt, nil
	} else {
		return channel.MessageTemplate{}, errors.New(fmt.Sprint("MessageTemplates ", templateName, " does not exists"))
	}

}

func (ms *ModuleStore) SaveChannel(channelObj channel.ChannelI, projectId string, realStore ModuleStoreI, persist bool) error {
	log.Println("inside SaveChannel")
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return err
	}
	log.Println(channelObj)
	err = prj.AddChannel(channelObj)
	if persist == true {
		return realStore.SaveStore("", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveChannel(channelName string, projectId string, realStore ModuleStoreI) error {
	if prg, ok := ms.Projects[projectId]; ok {
		cKey := fmt.Sprint(channelName)
		if _, ok := prg.Channels[cKey]; ok {
			delete(prg.Channels, cKey)
			log.Print("SaveStore called from RemoveChannel")
			return realStore.SaveStore("", realStore)
		} else {
			return errors.New(fmt.Sprint("Channel ", cKey, " does not exists"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetChannel(channelName string, projectId string, realStore ModuleStoreI) (channel.ChannelI, error) {
	if prg, ok := ms.Projects[projectId]; ok {
		cKey := fmt.Sprint(channelName)
		if ch, ok := prg.Channels[cKey]; ok {
			return ch, nil
		} else {
			return nil, errors.New(fmt.Sprint("Channel ", cKey, " does not exists"))
		}
	} else {
		return nil, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}
