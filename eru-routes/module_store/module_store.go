package module_store

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-routes/module_model"
	"github.com/eru-tech/eru/eru-routes/routes"
	"github.com/eru-tech/eru/eru-store/store"
	"log"
	"net/http"
	"strings"
)

var Eruqlbaseurl = "http://localhost:8087"
var FuncThreads = 3
var LoopThreads = 3

type StoreHolder struct {
	Store ModuleStoreI
}
type ModuleStoreI interface {
	store.StoreI
	SaveProject(projectId string, realStore ModuleStoreI, persist bool) error
	SaveProjectConfig(projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error
	RemoveProject(projectId string, realStore ModuleStoreI) error
	SaveProjectAuthorizer(projectId string, authorizer routes.Authorizer, realStore ModuleStoreI) error
	RemoveProjectAuthorizer(projectId string, authorizerName string) error
	GetProjectAuthorizer(projectId string, authorizerName string) (routes.Authorizer, error)
	GetProjectConfig(projectId string) (*module_model.Project, error)
	GetProjectList() []map[string]interface{}
	SaveRoute(routeObj routes.Route, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveRoute(routeName string, projectId string, realStore ModuleStoreI) error
	GetAndValidateRoute(routeName string, projectId string, host string, url string, method string, headers http.Header) (route routes.Route, err error)
	GetAndValidateFunc(funcName string, projectId string, host string, url string, method string, headers http.Header) (funcGroup routes.FuncGroup, err error)
	SaveFunc(funcObj routes.FuncGroup, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveFunc(funcName string, projectId string, realStore ModuleStoreI) error
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
		if project.FuncGroups == nil {
			project.FuncGroups = make(map[string]routes.FuncGroup)
		}
		if project.Authorizers == nil {
			project.Authorizers = make(map[string]routes.Authorizer)
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

func (ms *ModuleStore) SaveProjectConfig(projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error {
	if _, ok := ms.Projects[projectId]; ok {
		ms.Projects[projectId].ProjectConfig = projectConfig
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
}

func (ms *ModuleStore) SaveProjectAuthorizer(projectId string, authorizer routes.Authorizer, realStore ModuleStoreI) error {
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].Authorizers == nil {
			ms.Projects[projectId].Authorizers = make(map[string]routes.Authorizer)
		}
		ms.Projects[projectId].Authorizers[authorizer.AuthorizerName] = authorizer
		return realStore.SaveStore("", realStore)
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
}

func (ms *ModuleStore) RemoveProjectAuthorizer(projectId string, authorizerName string) error {
	if _, ok := ms.Projects[projectId]; ok {
		if _, authOk := ms.Projects[projectId].Authorizers[authorizerName]; authOk {
			delete(ms.Projects[projectId].Authorizers, authorizerName)
			return nil
		} else {
			return errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " not found"))
	}
}
func (ms *ModuleStore) GetProjectAuthorizer(projectId string, authorizerName string) (routes.Authorizer, error) {
	if _, ok := ms.Projects[projectId]; ok {
		if _, authOk := ms.Projects[projectId].Authorizers[authorizerName]; authOk {
			return ms.Projects[projectId].Authorizers[authorizerName], nil
		} else {
			return routes.Authorizer{}, errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
		}
	} else {
		return routes.Authorizer{}, errors.New(fmt.Sprint("Project ", projectId, " not found"))
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

func (ms *ModuleStore) GetAndValidateRoute(routeName string, projectId string, host string, url string, method string, headers http.Header) (route routes.Route, err error) {
	cloneRoute := routes.Route{}
	if prg, ok := ms.Projects[projectId]; ok {
		if route, ok = prg.Routes[routeName]; !ok {
			return cloneRoute, errors.New(fmt.Sprint("Route ", routeName, " does not exists"))
		}
		routeI, jmErr := json.Marshal(route)
		if jmErr != nil {
			log.Print()
			return cloneRoute, errors.New("route marshal failed")
		}
		jmErr = json.Unmarshal(routeI, &cloneRoute)
		if jmErr != nil {
			log.Print()
			return cloneRoute, errors.New("route unmarshal failed")
		}
		cloneRoute.TokenSecret = prg.ProjectConfig.TokenSecret
	} else {
		return cloneRoute, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
	err = cloneRoute.Validate(host, url, method, headers)
	if err != nil {
		return cloneRoute, err
	}
	return cloneRoute, nil
}

func (ms *ModuleStore) GetAndValidateFunc(funcName string, projectId string, host string, url string, method string, headers http.Header) (funcGroup routes.FuncGroup, err error) {
	log.Println("inside GetAndValidateFunc")
	if prg, ok := ms.Projects[projectId]; ok {
		if funcGroup, ok = prg.FuncGroups[funcName]; !ok {
			return funcGroup, errors.New(fmt.Sprint("Function ", funcName, " does not exists"))
		}
		funcGroup.TokenSecret = prg.ProjectConfig.TokenSecret
	} else {
		return funcGroup, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
	var errArray []string
	for k, v := range funcGroup.FuncSteps {
		fs := funcGroup.FuncSteps[k]
		err = ms.loadRoutesForFunction(fs, v.RouteName, projectId, host, v.Path, method, headers)
		if err != nil {
			log.Println(err)
			errArray = append(errArray, err.Error())
		}
	}
	if len(errArray) > 0 {
		return funcGroup, errors.New(strings.Join(errArray, " , "))
	}
	return
}

func (ms *ModuleStore) loadRoutesForFunction(funcStep *routes.FuncStep, routeName string, projectId string, host string, url string, method string, headers http.Header) (err error) {
	log.Println("inside loadRoutesForFunction for route = ", funcStep.GetRouteName())
	var errArray []string
	r := routes.Route{}
	if funcStep.FunctionName != "" {
		funcGroup, fgErr := ms.GetAndValidateFunc(funcStep.FunctionName, projectId, host, url, method, headers)
		if fgErr != nil {
			err = fgErr
			return
		}
		funcStep.FuncGroup = funcGroup
	} else {
		if funcStep.QueryName != "" {
			log.Print("making dummy route for query name ", funcStep.QueryName)
			r.RouteName = funcStep.QueryName
			r.Url = "/"
			r.MatchType = "PREFIX"
			output := ""
			if funcStep.QueryOutput == "csv" {
				output = "/csv"
			} else if funcStep.QueryOutput == "excel" {
				output = "/excel"
			}

			r.RewriteUrl = fmt.Sprint("/store/", projectId, "/myquery/execute/", funcStep.QueryName, output)
			tg := routes.TargetHost{}
			tg.Method = "POST"
			tmpSplit := strings.Split(Eruqlbaseurl, "://")
			tg.Host = Eruqlbaseurl
			tg.Scheme = "https"
			if len(tmpSplit) > 0 {
				tg.Scheme = tmpSplit[0]
				tg.Host = tmpSplit[1]
			}
			tg.Allocation = 100
			r.LoopVariable = ""
			r.Condition = ""
			r.TargetHosts = append(r.TargetHosts, tg)
		} else if funcStep.Api.Host != "" {
			log.Print("making dummy route for query name ", funcStep.Api.Host)
			r.RouteName = strings.Replace(funcStep.Api.Host, ".", "", -1)
			r.Url = "/"
			r.MatchType = "PREFIX"
			r.RewriteUrl = funcStep.ApiPath
			r.LoopVariable = ""
			r.Condition = ""
			r.OnError = "IGNORE"
			r.TargetHosts = append(r.TargetHosts, funcStep.Api)
		} else {
			r, err = ms.GetAndValidateRoute(routeName, projectId, host, url, method, headers)
			if err != nil {
				return
			}
		}
		funcStep.Route = r

	}
	for ck, cv := range funcStep.FuncSteps {
		log.Println("inside funcStep.FuncSteps - child iteration")
		fs := funcStep.FuncSteps[ck]
		err = ms.loadRoutesForFunction(fs, cv.RouteName, projectId, host, cv.Path, method, headers)
		if err != nil {
			log.Println(err)
			errArray = append(errArray, err.Error())
		}
	}
	if len(errArray) > 0 {
		return errors.New(strings.Join(errArray, " , "))
	}
	return
}

func (ms *ModuleStore) SaveFunc(funcObj routes.FuncGroup, projectId string, realStore ModuleStoreI, persist bool) error {
	log.Println("inside SaveFunc")
	prj, err := ms.GetProjectConfig(projectId)
	if err != nil {
		log.Print(err)
		return err
	}
	err = prj.AddFunc(funcObj)
	if persist == true {
		return realStore.SaveStore("", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveFunc(funcName string, projectId string, realStore ModuleStoreI) error {
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.FuncGroups[funcName]; ok {
			delete(prg.FuncGroups, funcName)
			log.Print("SaveStore called from RemoveFunc")
			return realStore.SaveStore("", realStore)
		} else {
			return errors.New(fmt.Sprint("Function ", funcName, " does not exists"))
		}
	} else {
		return errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
}
