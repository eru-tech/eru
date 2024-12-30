package module_store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	models "github.com/eru-tech/eru/eru-ai/models"
	module_model "github.com/eru-tech/eru/eru-ai/module_model"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-store/store"
)

var Erufuncbaseurl = "http://localhost:8083"

type StoreHolder struct {
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error)
	GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (module_model.ExtendedProject, error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SaveModel(ctx context.Context, modelObj models.ModelI, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error
	RemoveModel(ctx context.Context, modelId string, projectId string, tenantId string, realStore ModuleStoreI) error
	GetModel(ctx context.Context, projectId string, tenantId string, modelId string, s ModuleStoreI) (models.ModelI, error)
	SaveProjectSettings(ctx context.Context, projectId string, projectSettings module_model.ProjectSettings, realStore ModuleStoreI) error
	RemoveTenants()
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
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	if _, ok := ms.Projects[projectId]; !ok {
		project := new(module_model.Project)
		project.ProjectId = projectId
		if ms.Projects == nil {
			ms.Projects = make(map[string]*module_model.Project)
		}
		if project.Tenants == nil {
			project.Tenants = make(map[string]module_model.TenantConfig)
		}

		ms.Projects[projectId] = project
		if persist {
			logs.WithContext(ctx).Info("SaveStore called from SaveProject")
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			return nil
		}
	} else {
		err := errors.New("Project " + projectId + " already exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveProject - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if _, ok := ms.Projects[projectId]; ok {
		delete(ms.Projects, projectId)
		logs.WithContext(ctx).Info("SaveStore called from RemoveProject")
		return realStore.SaveStore(ctx, projectId, "", realStore)
	} else {
		err := errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (ePrj module_model.ExtendedProject, err error) {
	logs.WithContext(ctx).Debug("GetExtendedProjectConfig - Start")
	ePrj = module_model.ExtendedProject{}
	if prj, ok := ms.Projects[projectId]; ok {
		ePrj.TenantVariables, err = realStore.FetchTenantVars(ctx, projectId)
		if err != nil {
			logs.WithContext(ctx).Warn(err.Error())
		}
		ePrj.Variables, err = realStore.FetchVars(ctx, projectId)
		if err != nil {
			logs.WithContext(ctx).Warn(err.Error())
		}
		ePrj.SecretManager, err = realStore.FetchSm(ctx, projectId)
		if err != nil {
			logs.WithContext(ctx).Warn(err.Error())
		}
		ePrj.SecretManager, err = realStore.FetchSm(ctx, projectId)
		if err != nil {
			logs.WithContext(ctx).Warn(err.Error())
		}
		ePrj.ProjectId = prj.ProjectId
		ePrj.ProjectSettings = prj.ProjectSettings
		ePrj.Tenants = prj.Tenants
		return ePrj, nil
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return module_model.ExtendedProject{}, err
	}
}

func (ms *ModuleStore) GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error) {
	logs.WithContext(ctx).Debug("GetProjectConfig - Start")
	if _, ok := ms.Projects[projectId]; ok {
		return ms.Projects[projectId], nil
	} else {
		err := errors.New("Project " + projectId + " does not exists")
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
		project["project_name"] = k
		projects[i] = project
		i++
	}
	return projects
}
func (ms *ModuleStore) SaveModel(ctx context.Context, modelObj models.ModelI, projectId string, tenantId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveModel - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}

	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return err
	}

	//save original modelObj with variables
	err = prj.AddModel(ctx, tenantId, modelObj)
	if err != nil {
		return err
	}

	if persist {
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	}
	return nil
}

func (ms *ModuleStore) RemoveModel(ctx context.Context, modelId string, projectId string, tenantId string, realStore ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("RemoveModel - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()

	if prj, ok := ms.Projects[projectId]; ok {
		if _, ok := prj.Tenants[tenantId]; !ok {
			err = errors.New("tenant " + tenantId + " does not exists")
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
		if _, ok := prj.Tenants[tenantId].Model[modelId]; !ok {
			err = errors.New("Model " + modelId + " does not exists")
			logs.WithContext(ctx).Info(err.Error())
			return err
		}
		err = prj.RemoveModel(ctx, tenantId, modelId)
		if err != nil {
			return err
		}
		return realStore.SaveTenantStore(ctx, projectId, tenantId, "", prj.Tenants[tenantId])
	} else {
		err = errors.New("Project " + projectId + " does not exists")
		logs.WithContext(ctx).Info(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetModelClone(ctx context.Context, projectId string, tenantId string, modelId string, s ModuleStoreI) (modelObjClone models.ModelI, err error) {
	logs.WithContext(ctx).Debug("GetModelClone - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		return
	}
	if _, ok := prj.Tenants[tenantId]; !ok {
		err = errors.New("tenant " + tenantId + " not found")
		logs.WithContext(ctx).Error(err.Error())
		return
	} else if modelObj, ok := prj.Tenants[tenantId].Model[modelId]; !ok {
		err = errors.New("model " + modelId + " not found")
		logs.WithContext(ctx).Error(err.Error())
		return
	} else {
		modelObjClone, err = ms.GetModelCloneObject(ctx, projectId, tenantId, modelObj, s)
		return
	}
}

func (ms *ModuleStore) GetModelCloneObject(ctx context.Context, projectId string, tenantId string, modelObj models.ModelI, s ModuleStoreI) (modelObjClone models.ModelI, err error) {
	logs.WithContext(ctx).Debug("GetModelCloneObject - Start")

	modelObjJson, modelObjJsonErr := json.Marshal(modelObj)
	if modelObjJsonErr != nil {
		err = errors.New("error while cloning modelObj (marshal)")
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(modelObjJsonErr.Error())
		return
	}
	modelObjJson = s.ReplaceTenantVariables(ctx, projectId, tenantId, modelObjJson)
	modelObjJson = s.ReplaceVariables(ctx, projectId, modelObjJson, nil)

	iCloneI := reflect.New(reflect.TypeOf(modelObj))
	modelObjCloneErr := json.Unmarshal(modelObjJson, iCloneI.Interface())
	if modelObjCloneErr != nil {
		err = errors.New("error while cloning modelObj(unmarshal)")
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(modelObjCloneErr.Error())
		return
	}
	return iCloneI.Elem().Interface().(models.ModelI), nil
}
func (ms *ModuleStore) GetModel(ctx context.Context, projectId string, tenantId string, modelId string, s ModuleStoreI) (models.ModelI, error) {
	logs.WithContext(ctx).Debug("GetModel - Start")
	return ms.GetModelClone(ctx, projectId, tenantId, modelId, s)

}

func (ms *ModuleStore) SaveProjectSettings(ctx context.Context, projectId string, projectSettings module_model.ProjectSettings, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveProjectConfig - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	err := ms.checkProjectExists(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	ms.Projects[projectId].ProjectSettings = projectSettings
	logs.WithContext(ctx).Info("SaveStore called from SaveProjectSettings")
	return realStore.SaveStore(ctx, projectId, "", realStore)
}
func (ms *ModuleStore) GetStoreWithoutTenants(ctx context.Context, realStore store.StoreI) (b []byte, err error) {
	logs.WithContext(ctx).Debug("GetStoreByteArrayWithoutTenants - Start")
	logs.WithContext(ctx).Info("calling custom get store byte array without tenants from eruai")

	realStoreJson, err := json.Marshal(realStore)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	newMs := new(ModuleDbStore)
	err = UnMarshalStore(ctx, realStoreJson, newMs)
	//err = json.Unmarshal(realStoreJson, newMs)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}

	newMs.RemoveTenants()
	b, err = json.Marshal(newMs)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return
}

func (ms *ModuleStore) RemoveTenants() {
	for key, project := range ms.Projects {
		project.Tenants = nil
		ms.Projects[key] = project
	}
}
