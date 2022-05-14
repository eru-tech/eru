package module_store

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-rules/module_model"
	"github.com/eru-tech/eru/eru-store/store"
	"log"
	"time"
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
	SaveDataType(projectId string, dataType module_model.DataType, realStore ModuleStoreI, persist bool) error
	RemoveDataType(projectId string, dataTypeName string, realStore ModuleStoreI) error
	GetDataTypeList(projectId string) (map[string]module_model.DataType, error)
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
		if project.DMNs == nil {
			project.DMNs = make(map[string]module_model.DMN)
		}
		if project.DataTypes == nil {
			project.DataTypes = make(map[string]module_model.DataType)
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
		project["createdBy"] = "user 1"
		project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
}

func (ms *ModuleStore) SaveDataType(projectId string, dataType module_model.DataType, realStore ModuleStoreI, persist bool) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	ms.Projects[projectId].DataTypes[dataType.Name] = dataType
	if persist == true {
		log.Print("SaveStore called from SaveDataType")
		return realStore.SaveStore("", realStore)
	} else {
		return nil
	}
}

func (ms *ModuleStore) RemoveDataType(projectId string, dataTypeName string, realStore ModuleStoreI) error {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return err
	}
	if _, ok := ms.Projects[projectId].DataTypes[dataTypeName]; ok {
		delete(ms.Projects[projectId].DataTypes, dataTypeName)
		log.Print("SaveStore called from RemoveDataType")
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Datatype ", dataTypeName, " does not exists"))
	}
}

func (ms *ModuleStore) GetDataTypeList(projectId string) (map[string]module_model.DataType, error) {
	err := ms.checkProjectExists(projectId)
	if err != nil {
		return nil, err
	}
	return ms.Projects[projectId].DataTypes, nil
}

func (ms *ModuleStore) checkProjectExists(projectId string) error {
	_, ok := ms.Projects[projectId]
	if !ok {
		return errors.New(fmt.Sprint("project ", projectId, " not found"))
	}
	return nil
}
