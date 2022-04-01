package module_store

import (
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-routes/module_model"
	"github.com/eru-tech/eru/eru-routes/routes"
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
	SaveRoute(routeObj routes.Route, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveRoute(routeName string, projectId string, realStore ModuleStoreI) error
	GetAndValidateRoute(routeName string, projectId string, host string, url string, method string) (route routes.Route, err error)
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
		if project.Routes == nil {
			project.Routes = make(map[string]routes.Route)
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

func (ms *ModuleStore) SaveRoute(routeObj routes.Route, projectId string, realStore ModuleStoreI, persist bool) error {
	log.Println("inside SaveRoute")
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return err
	}
	err = prj.AddRoute(routeObj)
	if persist == true {
		return realStore.SaveStore("", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveRoute(routeName string, projectId string, realStore ModuleStoreI) error {
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.Routes[routeName]; ok {
			delete(prg.Routes, routeName)
			log.Print("SaveStore called from RemoveRoute")
			return realStore.SaveStore("", realStore)
		} else {
			return errors.New(fmt.Sprint("Route ", routeName, " does not exists"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}

func (ms *ModuleStore) GetAndValidateRoute(routeName string, projectId string, host string, url string, method string) (route routes.Route, err error) {
	if prg, ok := ms.Projects[projectId]; ok {
		if route, ok = prg.Routes[routeName]; !ok {
			return route, errors.New(fmt.Sprint("Route ", routeName, " does not exists"))
		}
	} else {
		return route, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
	err = route.Validate(host, url, method)
	if err != nil {
		return
	}
	err = route.MakeFinalUrl(url)
	return

}
