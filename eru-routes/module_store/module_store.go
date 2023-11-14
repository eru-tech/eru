package module_store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/module_model"
	"github.com/eru-tech/eru/eru-routes/routes"
	"github.com/eru-tech/eru/eru-store/store"
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
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	SaveProjectConfig(ctx context.Context, projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	SaveProjectAuthorizer(ctx context.Context, projectId string, authorizer routes.Authorizer, realStore ModuleStoreI) error
	RemoveProjectAuthorizer(ctx context.Context, projectId string, authorizerName string) error
	GetProjectAuthorizer(ctx context.Context, projectId string, authorizerName string) (routes.Authorizer, error)
	GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SaveRoute(ctx context.Context, routeObj routes.Route, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveRoute(ctx context.Context, routeName string, projectId string, realStore ModuleStoreI) error
	GetAndValidateRoute(ctx context.Context, routeName string, projectId string, host string, url string, method string, headers http.Header, s ModuleStoreI) (route routes.Route, err error)
	GetAndValidateFunc(ctx context.Context, funcName string, projectId string, host string, url string, method string, headers http.Header, s ModuleStoreI) (funcGroup routes.FuncGroup, err error)
	SaveFunc(ctx context.Context, funcObj routes.FuncGroup, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveFunc(ctx context.Context, funcName string, projectId string, realStore ModuleStoreI) error
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
			logs.WithContext(ctx).Info("SaveStore called from SaveProject")
			return realStore.SaveStore(ctx, "", realStore)
		} else {
			return nil
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " already exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) SaveProjectConfig(ctx context.Context, projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveProjectConfig - Start")
	if _, ok := ms.Projects[projectId]; ok {
		ms.Projects[projectId].ProjectConfig = projectConfig
		return realStore.SaveStore(ctx, "", realStore)
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) SaveProjectAuthorizer(ctx context.Context, projectId string, authorizer routes.Authorizer, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("SaveProjectAuthorizer - Start")
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].Authorizers == nil {
			ms.Projects[projectId].Authorizers = make(map[string]routes.Authorizer)
		}
		ms.Projects[projectId].Authorizers[authorizer.AuthorizerName] = authorizer
		return realStore.SaveStore(ctx, "", realStore)
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) RemoveProjectAuthorizer(ctx context.Context, projectId string, authorizerName string) error {
	logs.WithContext(ctx).Debug("RemoveProjectAuthorizer - Start")
	if _, ok := ms.Projects[projectId]; ok {
		if _, authOk := ms.Projects[projectId].Authorizers[authorizerName]; authOk {
			delete(ms.Projects[projectId].Authorizers, authorizerName)
			return nil
		} else {
			err := errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}
func (ms *ModuleStore) GetProjectAuthorizer(ctx context.Context, projectId string, authorizerName string) (routes.Authorizer, error) {
	logs.WithContext(ctx).Debug("GetProjectAuthorizer - Start")
	if _, ok := ms.Projects[projectId]; ok {
		if _, authOk := ms.Projects[projectId].Authorizers[authorizerName]; authOk {
			return ms.Projects[projectId].Authorizers[authorizerName], nil
		} else {
			err := errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
			logs.WithContext(ctx).Error(err.Error())
			return routes.Authorizer{}, err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
		logs.WithContext(ctx).Error(err.Error())
		return routes.Authorizer{}, err
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
		//project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
}

func (ms *ModuleStore) SaveRoute(ctx context.Context, routeObj routes.Route, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveRoute - Start")
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	err = prj.AddRoute(ctx, routeObj)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	if persist == true {
		return realStore.SaveStore(ctx, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveRoute(ctx context.Context, routeName string, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveRoute - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.Routes[routeName]; ok {
			delete(prg.Routes, routeName)
			logs.WithContext(ctx).Info("SaveStore called from RemoveRoute")
			return realStore.SaveStore(ctx, "", realStore)
		} else {
			err := errors.New(fmt.Sprint("Route ", routeName, " does not exists"))
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetAndValidateRoute(ctx context.Context, routeName string, projectId string, host string, url string, method string, headers http.Header, s ModuleStoreI) (route routes.Route, err error) {
	logs.WithContext(ctx).Debug("GetAndValidateRoute - Start")
	cloneRoute := routes.Route{}
	if prg, ok := ms.Projects[projectId]; ok {
		if route, ok = prg.Routes[routeName]; !ok {
			err = errors.New(fmt.Sprint("Route ", routeName, " does not exists"))
			logs.WithContext(ctx).Error(err.Error())
			return cloneRoute, err
		}
		routeI, jmErr := json.Marshal(route)
		if jmErr != nil {
			err = errors.New("route marshal failed")
			logs.WithContext(ctx).Error(fmt.Sprint(err.Error(), " : ", jmErr.Error()))
			return cloneRoute, err
		}
		routeI = s.ReplaceVariables(ctx, projectId, routeI)
		jmErr = json.Unmarshal(routeI, &cloneRoute)
		if jmErr != nil {
			err = errors.New("route unmarshal failed")
			logs.WithContext(ctx).Error(fmt.Sprint(err.Error(), " : ", jmErr.Error()))
			return cloneRoute, err
		}
		cloneRoute.TokenSecret = prg.ProjectConfig.TokenSecret
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return cloneRoute, err
	}
	err = cloneRoute.Validate(ctx, host, url, method, headers)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return cloneRoute, err
	}
	return cloneRoute, nil
}

func (ms *ModuleStore) GetAndValidateFunc(ctx context.Context, funcName string, projectId string, host string, url string, method string, headers http.Header, s ModuleStoreI) (cloneFunc routes.FuncGroup, err error) {
	logs.WithContext(ctx).Debug("GetAndValidateFunc - Start")
	funcGroup := routes.FuncGroup{}
	if prg, ok := ms.Projects[projectId]; ok {
		if funcGroup, ok = prg.FuncGroups[funcName]; !ok {
			return funcGroup, errors.New(fmt.Sprint("Function ", funcName, " does not exists"))
		}

		FuncI, jmErr := json.Marshal(funcGroup)
		if jmErr != nil {
			err = errors.New("funcGroup marshal failed")
			logs.WithContext(ctx).Error(fmt.Sprint(err.Error(), " : ", jmErr.Error()))
			return cloneFunc, err
		}
		FuncI = s.ReplaceVariables(ctx, projectId, FuncI)
		jmErr = json.Unmarshal(FuncI, &cloneFunc)
		if jmErr != nil {
			err = errors.New("funcGroup unmarshal failed")
			logs.WithContext(ctx).Error(fmt.Sprint(err.Error(), " : ", jmErr.Error()))
			return cloneFunc, err
		}
		cloneFunc.TokenSecret = prg.ProjectConfig.TokenSecret
	} else {
		return cloneFunc, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}

	var errArray []string
	for k, v := range cloneFunc.FuncSteps {
		fs := cloneFunc.FuncSteps[k]
		err = ms.LoadRoutesForFunction(ctx, fs, v.RouteName, projectId, host, v.Path, method, headers, s)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			errArray = append(errArray, err.Error())
		}
	}
	if len(errArray) > 0 {
		err = errors.New(strings.Join(errArray, " , "))
		logs.WithContext(ctx).Error(err.Error())
		return cloneFunc, err
	}
	return
}

func (ms *ModuleStore) LoadRoutesForFunction(ctx context.Context, funcStep *routes.FuncStep, routeName string, projectId string, host string, url string, method string, headers http.Header, s ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug(fmt.Sprint("loadRoutesForFunction - Start : ", funcStep.GetRouteName()))
	var errArray []string
	r := routes.Route{}
	if funcStep.FunctionName != "" {
		funcGroup, fgErr := ms.GetAndValidateFunc(ctx, funcStep.FunctionName, projectId, host, url, method, headers, s)
		if fgErr != nil {
			err = fgErr
			return
		}
		funcStep.FuncGroup = funcGroup
	} else {
		if funcStep.QueryName != "" {
			logs.WithContext(ctx).Info(fmt.Sprint("making dummy route for query name ", funcStep.QueryName))
			r.RouteName = funcStep.QueryName
			r.Url = "/"
			r.MatchType = "PREFIX"
			output := ""
			encode := ""
			if funcStep.QueryOutput == "csv" {
				output = "/csv"
			} else if funcStep.QueryOutput == "excel" {
				output = "/excel"
			}

			if funcStep.QueryOutputEncode {
				encode = "/encode"
			}

			r.RewriteUrl = fmt.Sprint("/store/", projectId, "/myquery/execute/", funcStep.QueryName, output, encode)
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
			logs.WithContext(ctx).Info(fmt.Sprint("making dummy route for api ", funcStep.GetRouteName()))
			r.RouteName = strings.Replace(strings.Replace(funcStep.Api.Host, ".", "", -1), ":", "", -1)
			r.Url = "/"
			r.MatchType = "PREFIX"
			r.RewriteUrl = funcStep.ApiPath
			r.LoopVariable = ""
			r.Condition = ""
			r.OnError = "IGNORE"
			r.TargetHosts = append(r.TargetHosts, funcStep.Api)
		} else {
			r, err = ms.GetAndValidateRoute(ctx, routeName, projectId, host, url, method, headers, s)
			if err != nil {
				return
			}
		}
		funcStep.Route = r
	}
	for ck, cv := range funcStep.FuncSteps {
		logs.WithContext(ctx).Info("inside funcStep.FuncSteps - child iteration")
		fs := funcStep.FuncSteps[ck]
		err = ms.LoadRoutesForFunction(ctx, fs, cv.RouteName, projectId, host, cv.Path, method, headers, s)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			errArray = append(errArray, err.Error())
		}
	}
	if len(errArray) > 0 {
		return errors.New(strings.Join(errArray, " , "))
	}
	return
}

func (ms *ModuleStore) SaveFunc(ctx context.Context, funcObj routes.FuncGroup, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug(fmt.Sprint("SaveFunc - Start"))
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	err = prj.AddFunc(ctx, funcObj)
	if persist == true {
		return realStore.SaveStore(ctx, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveFunc(ctx context.Context, funcName string, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug(fmt.Sprint("RemoveFunc - Start"))
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.FuncGroups[funcName]; ok {
			delete(prg.FuncGroups, funcName)
			logs.WithContext(ctx).Info(fmt.Sprint("SaveStore called from RemoveFunc"))
			return realStore.SaveStore(ctx, "", realStore)
		} else {
			err := errors.New(fmt.Sprint("Function ", funcName, " does not exists"))
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}
