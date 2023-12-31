package module_store

import (
	"context"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-alerts/channel"
	"github.com/eru-tech/eru/eru-alerts/module_model"
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
	SaveMessageTemplate(ctx context.Context, projectId string, messageTemplate channel.MessageTemplate, realStore ModuleStoreI) error
	RemoveMessageTemplate(ctx context.Context, projectId string, templateName string, realStore ModuleStoreI) error
	GetMessageTemplate(ctx context.Context, projectId string, templateId string, realStore ModuleStoreI) (channel.MessageTemplate, error)
	SaveChannel(ctx context.Context, channelObj channel.ChannelI, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveChannel(ctx context.Context, channelName string, projectId string, realStore ModuleStoreI) error
	GetChannel(ctx context.Context, channelName string, projectId string, realStore ModuleStoreI) (channel.ChannelI, error)
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
	logs.WithContext(ctx).Debug("SaveProject - Start")
	//TODO to handle edit project once new project attributes are finalized
	if _, ok := ms.Projects[projectId]; !ok {
		project := new(module_model.Project)
		project.ProjectId = projectId
		if ms.Projects == nil {
			ms.Projects = make(map[string]*module_model.Project)
		}
		ms.Projects[projectId] = project
		if persist == true {
			logs.WithContext(ctx).Info("SaveStore called from SaveProject")
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			return nil
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " already exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveProject - Start")
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		logs.WithContext(ctx).Info("SaveStore called from RemoveProject")
		return realStore.SaveStore(ctx, projectId, "", realStore)
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error) {
	logs.WithContext(ctx).Debug("GetProjectConfig - Start")
	if _, ok := ms.Projects[projectId]; ok {
		return ms.Projects[projectId], nil
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
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

func (ms *ModuleStore) SaveMessageTemplate(ctx context.Context, projectId string, messageTemplate channel.MessageTemplate, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveMessageTemplate - Start")
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		return err
	}
	if ms.Projects[projectId].MessageTemplates == nil {
		ms.Projects[projectId].MessageTemplates = make(map[string]channel.MessageTemplate)
	}
	ms.Projects[projectId].MessageTemplates[messageTemplate.TemplateName] = messageTemplate
	return realStore.SaveStore(ctx, projectId, "", realStore)
}

func (ms *ModuleStore) RemoveMessageTemplate(ctx context.Context, projectId string, templateName string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveMessageTemplate - Start")
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		return err
	}
	if _, ok := ms.Projects[projectId].MessageTemplates[templateName]; ok {
		delete(ms.Projects[projectId].MessageTemplates, templateName)
		return realStore.SaveStore(ctx, projectId, "", realStore)
	} else {
		err := errors.New(fmt.Sprint("MessageTemplates ", templateName, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetMessageTemplate(ctx context.Context, projectId string, templateName string, realStore ModuleStoreI) (channel.MessageTemplate, error) {
	logs.WithContext(ctx).Debug("GetMessageTemplate - Start")
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		return channel.MessageTemplate{}, err
	}
	if mt, ok := ms.Projects[projectId].MessageTemplates[templateName]; ok {
		return mt, nil
	} else {
		err = errors.New(fmt.Sprint("MessageTemplates ", templateName, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return channel.MessageTemplate{}, err
	}
}

func (ms *ModuleStore) SaveChannel(ctx context.Context, channelObj channel.ChannelI, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveChannel - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}
	err = prj.AddChannel(ctx, channelObj)
	if persist == true {
		return realStore.SaveStore(ctx, projectId, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveChannel(ctx context.Context, channelName string, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveChannel - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		cKey := fmt.Sprint(channelName)
		if _, ok := prg.Channels[cKey]; ok {
			delete(prg.Channels, cKey)
			logs.WithContext(ctx).Info(("SaveStore called from RemoveChannel"))
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			err := errors.New(fmt.Sprint("Channel ", cKey, " does not exists"))
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetChannel(ctx context.Context, channelName string, projectId string, realStore ModuleStoreI) (channel.ChannelI, error) {
	logs.WithContext(ctx).Debug("GetChannel - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		cKey := fmt.Sprint(channelName)
		if ch, ok := prg.Channels[cKey]; ok {
			return ch, nil
		} else {
			err := errors.New(fmt.Sprint("Channel ", cKey, " does not exists"))
			logs.WithContext(ctx).Error(err.Error())
			return nil, err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return nil, err
	}
}
