package module_store

import (
	"bufio"
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eru-tech/eru/eru-db/db"
	"github.com/eru-tech/eru/eru-events/events"
	"github.com/eru-tech/eru/eru-functions/functions"
	"github.com/eru-tech/eru/eru-functions/module_model"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	models "github.com/eru-tech/eru/eru-models"
	"github.com/eru-tech/eru/eru-store/store"
	eru_utils "github.com/eru-tech/eru/eru-utils"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"
)

var Eruqlbaseurl = "http://localhost:8087"
var FuncThreads = 3
var LoopThreads = 3
var EventThreads = 3

const (
	SELECT_FUNC_ASYNC = "update erufunctions_async set async_status='IN PROGRESS', processed_date=now() where async_id = ??? and (async_status=??? or 'ALL'=???) returning async_id, event_id, func_group_name func_name, func_step_name,  event_msg, event_request, request_id"
	UPDATE_FUNC_ASYNC = "update erufunctions_async set async_status=???, processed_date=now(), event_response=??? where async_id = ???"
)

type StoreHolder struct {
	Store ModuleStoreI
}

type AsyncFuncData struct {
	AsyncId      string                     `json:"async_id"`
	EventId      string                     `json:"event_id"`
	FuncName     string                     `json:"func_group_name"`
	FuncStepName string                     `json:"func_step_name"`
	EventMsg     functions.FuncTemplateVars `json:"event_msg"`
	EventRequest string                     `json:"event_request"`
	RequestId    string                     `json:"request_id"`
}
type ModuleStoreI interface {
	store.StoreI
	SaveProject(ctx context.Context, projectId string, realStore ModuleStoreI, persist bool) error
	//SaveProjectConfig(ctx context.Context, projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error
	SaveProjectSettings(ctx context.Context, projectId string, projectConfig module_model.ProjectSettings, realStore ModuleStoreI) error
	RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error
	//SaveProjectAuthorizer(ctx context.Context, projectId string, authorizer functions.Authorizer, realStore ModuleStoreI) error
	//RemoveProjectAuthorizer(ctx context.Context, projectId string, authorizerName string) error
	//GetProjectAuthorizer(ctx context.Context, projectId string, authorizerName string) (functions.Authorizer, error)
	GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error)
	GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (module_model.ExtendedProject, error)
	GetProjectList(ctx context.Context) []map[string]interface{}
	SaveRoute(ctx context.Context, routeObj functions.Route, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveRoute(ctx context.Context, routeName string, projectId string, realStore ModuleStoreI) error
	GetAndValidateRoute(ctx context.Context, routeName string, projectId string, host string, url string, method string, headers http.Header, s ModuleStoreI) (route functions.Route, err error)
	GetAndValidateFunc(ctx context.Context, funcName string, projectId string, host string, url string, method string, headers http.Header, reqBody map[string]interface{}, s ModuleStoreI) (funcGroup functions.FuncGroup, err error)
	GetWf(ctx context.Context, wfName string, projectId string, s ModuleStoreI) (wfObj functions.Workflow, err error)
	ValidateFunc(ctx context.Context, funcObj functions.FuncGroup, projectId string, host string, url string, method string, headers http.Header, reqBody map[string]interface{}, s ModuleStoreI) (funcGroup functions.FuncGroup, err error)
	SaveFunc(ctx context.Context, funcObj functions.FuncGroup, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveFunc(ctx context.Context, funcName string, projectId string, realStore ModuleStoreI) error
	GetFunctionNames(ctx context.Context, projectId string) (functions []string, err error)
	SaveWf(ctx context.Context, wfObj functions.Workflow, projectId string, realStore ModuleStoreI, persist bool) error
	RemoveWf(ctx context.Context, wfName string, projectId string, realStore ModuleStoreI) error
	FetchAsyncEvent(ctx context.Context, asyncId string, asyncStatus string, realStore ModuleStoreI) (asyncFuncData AsyncFuncData, err error)
	UpdateAsyncEvent(ctx context.Context, asyncId string, asyncStatus string, eventResponse string, realStore ModuleStoreI) (err error)
	FetchProjectEvents(ctx context.Context, s ModuleStoreI) (err error)
	StartPolling(ctx context.Context, projectId string, event events.EventI, s ModuleStoreI) (err error)
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
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}

	//TODO to handle edit project once new project attributes are finalized
	if _, ok := ms.Projects[projectId]; !ok {
		project := new(module_model.Project)
		project.ProjectId = projectId
		if ms.Projects == nil {
			ms.Projects = make(map[string]*module_model.Project)
		}
		if project.Routes == nil {
			project.Routes = make(map[string]functions.Route)
		}
		if project.FuncGroups == nil {
			project.FuncGroups = make(map[string]functions.FuncGroup)
		}
		//if project.Authorizers == nil {
		//	project.Authorizers = make(map[string]functions.Authorizer)
		//}
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

//func (ms *ModuleStore) SaveProjectConfig(ctx context.Context, projectId string, projectConfig module_model.ProjectConfig, realStore ModuleStoreI) error {
//	logs.WithContext(ctx).Debug("SaveProjectConfig - Start")
//	if _, ok := ms.Projects[projectId]; ok {
//		ms.Projects[projectId].ProjectConfig = projectConfig
//		return realStore.SaveStore(ctx, projectId,"", realStore)
//	} else {
//		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
//		logs.WithContext(ctx).Error(err.Error())
//		return err
//	}
//}
//
//func (ms *ModuleStore) SaveProjectAuthorizer(ctx context.Context, projectId string, authorizer functions.Authorizer, realStore ModuleStoreI) error {
//	logs.WithContext(ctx).Debug("SaveProjectAuthorizer - Start")
//	if _, ok := ms.Projects[projectId]; ok {
//		if ms.Projects[projectId].Authorizers == nil {
//			ms.Projects[projectId].Authorizers = make(map[string]functions.Authorizer)
//		}
//		ms.Projects[projectId].Authorizers[authorizer.AuthorizerName] = authorizer
//		return realStore.SaveStore(ctx, projectId,"", realStore)
//	} else {
//		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
//		logs.WithContext(ctx).Error(err.Error())
//		return err
//	}
//}
//
//func (ms *ModuleStore) RemoveProjectAuthorizer(ctx context.Context, projectId string, authorizerName string) error {
//	logs.WithContext(ctx).Debug("RemoveProjectAuthorizer - Start")
//	if _, ok := ms.Projects[projectId]; ok {
//		if _, authOk := ms.Projects[projectId].Authorizers[authorizerName]; authOk {
//			delete(ms.Projects[projectId].Authorizers, authorizerName)
//			return nil
//		} else {
//			err := errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
//			logs.WithContext(ctx).Error(err.Error())
//			return err
//		}
//	} else {
//		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
//		logs.WithContext(ctx).Error(err.Error())
//		return err
//	}
//}
//func (ms *ModuleStore) GetProjectAuthorizer(ctx context.Context, projectId string, authorizerName string) (functions.Authorizer, error) {
//	logs.WithContext(ctx).Debug("GetProjectAuthorizer - Start")
//	if _, ok := ms.Projects[projectId]; ok {
//		if _, authOk := ms.Projects[projectId].Authorizers[authorizerName]; authOk {
//			return ms.Projects[projectId].Authorizers[authorizerName], nil
//		} else {
//			err := errors.New(fmt.Sprint("Authorizer ", authorizerName, " not found"))
//			logs.WithContext(ctx).Error(err.Error())
//			return functions.Authorizer{}, err
//		}
//	} else {
//		err := errors.New(fmt.Sprint("Project ", projectId, " not found"))
//		logs.WithContext(ctx).Error(err.Error())
//		return functions.Authorizer{}, err
//	}
//}

func (ms *ModuleStore) RemoveProject(ctx context.Context, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveProject - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
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
func (ms *ModuleStore) GetExtendedProjectConfig(ctx context.Context, projectId string, realStore ModuleStoreI) (ePrj module_model.ExtendedProject, err error) {
	logs.WithContext(ctx).Debug("GetExtendedProjectConfig - Start")
	ePrj = module_model.ExtendedProject{}
	if prj, ok := ms.Projects[projectId]; ok {
		ePrj.Variables, err = realStore.FetchVars(ctx, projectId)
		ePrj.SecretManager, err = realStore.FetchSm(ctx, projectId)
		ePrj.ProjectId = prj.ProjectId
		ePrj.ProjectSettings = prj.ProjectSettings
		ePrj.Routes = prj.Routes
		ePrj.FuncGroups = prj.FuncGroups
		ePrj.Workflows = prj.Workflows
		return ePrj, nil
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return module_model.ExtendedProject{}, err
	}
}

func (ms *ModuleStore) GetProjectConfig(ctx context.Context, projectId string) (*module_model.Project, error) {
	logs.WithContext(ctx).Debug("GetProjectConfig - Start")
	if _, ok := ms.Projects[projectId]; ok {

		logs.WithContext(ctx).Info(fmt.Sprint(ms.Projects[projectId].Workflows))

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
		project["project_name"] = k
		//project["lastUpdateDate"] = time.Now()
		projects[i] = project
		i++
	}
	return projects
}

func (ms *ModuleStore) SaveRoute(ctx context.Context, routeObj functions.Route, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug("SaveRoute - Start")
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
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
		return realStore.SaveStore(ctx, projectId, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveRoute(ctx context.Context, routeName string, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug("RemoveRoute - Start")
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.Routes[routeName]; ok {
			delete(prg.Routes, routeName)
			logs.WithContext(ctx).Info("SaveStore called from RemoveRoute")
			return realStore.SaveStore(ctx, projectId, "", realStore)
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

func (ms *ModuleStore) GetAndValidateRoute(ctx context.Context, routeName string, projectId string, host string, url string, method string, headers http.Header, s ModuleStoreI) (route functions.Route, err error) {
	logs.WithContext(ctx).Debug("GetAndValidateRoute - Start")
	cloneRoute := functions.Route{}
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
		routeI = s.ReplaceVariables(ctx, projectId, routeI, nil)
		jmErr = json.Unmarshal(routeI, &cloneRoute)
		if jmErr != nil {
			err = errors.New("route unmarshal failed")
			logs.WithContext(ctx).Error(fmt.Sprint(err.Error(), " : ", jmErr.Error()))
			return cloneRoute, err
		}
		cloneRoute.TokenSecretKey = prg.ProjectSettings.ClaimsKey
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

func (ms *ModuleStore) GetAndValidateFunc(ctx context.Context, funcName string, projectId string, host string, url string, method string, headers http.Header, reqBody map[string]interface{}, s ModuleStoreI) (cloneFunc functions.FuncGroup, err error) {
	logs.WithContext(ctx).Debug("GetAndValidateFunc - Start")
	funcGroup := functions.FuncGroup{}
	if prg, ok := ms.Projects[projectId]; ok {
		if funcGroup, ok = prg.FuncGroups[funcName]; !ok {
			return funcGroup, errors.New(fmt.Sprint("Function ", funcName, " does not exists"))
		}
		return ms.ValidateFunc(ctx, funcGroup, projectId, host, url, method, headers, reqBody, s)
	}
	return
}

func (ms *ModuleStore) ValidateFunc(ctx context.Context, funcGroup functions.FuncGroup, projectId string, host string, url string, method string, headers http.Header, reqBody map[string]interface{}, s ModuleStoreI) (cloneFunc functions.FuncGroup, err error) {
	logs.WithContext(ctx).Debug("ValidateFunc - Start")
	if prg, ok := ms.Projects[projectId]; ok {
		FuncI, jmErr := json.Marshal(funcGroup)
		if jmErr != nil {
			err = errors.New("funcGroup marshal failed")
			logs.WithContext(ctx).Error(fmt.Sprint(err.Error(), " : ", jmErr.Error()))
			return cloneFunc, err
		}
		FuncI = s.ReplaceVariables(ctx, projectId, FuncI, reqBody)
		jmErr = json.Unmarshal(FuncI, &cloneFunc)
		if jmErr != nil {
			err = errors.New("funcGroup unmarshal failed")
			logs.WithContext(ctx).Error(fmt.Sprint(err.Error(), " : ", jmErr.Error()))
			return cloneFunc, err
		}
		cloneFunc.TokenSecretKey = prg.ProjectSettings.ClaimsKey
	} else {
		return cloneFunc, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}

	var errArray []string
	for k, v := range cloneFunc.FuncSteps {
		fs := cloneFunc.FuncSteps[k]
		fs.ParentFuncGroupName = cloneFunc.FuncGroupName
		err = ms.LoadRoutesForFunction(ctx, fs, v.RouteName, projectId, host, v.Path, method, headers, s, cloneFunc.TokenSecretKey, reqBody)
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

func (ms *ModuleStore) LoadRoutesForFunction(ctx context.Context, funcStep *functions.FuncStep, routeName string, projectId string, host string, url string, method string, headers http.Header, s ModuleStoreI, tokenHeaderKey string, reqBody map[string]interface{}) (err error) {
	logs.WithContext(ctx).Info(fmt.Sprint("loadRoutesForFunction - Start : ", funcStep.GetRouteName()))
	var errArray []string
	r := functions.Route{}

	logs.WithContext(ctx).Info(s.GetDbType())
	funcStep.FsDb = db.GetDb(s.GetDbType())
	funcStep.FsDb.SetConn(s.GetConn())
	if funcStep.AsyncEventName != "" {
		var eventI events.EventI
		eventI, err = s.FetchEvent(ctx, projectId, funcStep.AsyncEventName)
		if err != nil {
			return
		}
		funcStep.AsyncEvent = eventI
	}

	if funcStep.FunctionName != "" {
		logs.WithContext(ctx).Info(fmt.Sprint("funcStep.FunctionName called for ", funcStep.FunctionName))
		funcGroup, fgErr := ms.GetAndValidateFunc(ctx, funcStep.FunctionName, projectId, host, url, method, headers, reqBody, s)
		if fgErr != nil {
			err = fgErr
			return
		}

		tsk := ms.Projects[projectId].ProjectSettings.ClaimsKey
		if funcStep.Async {
			for k, _ := range funcGroup.FuncSteps {
				funcGroup.FuncSteps[k].Async = true
				funcGroup.FuncSteps[k].AsyncEvent = funcStep.AsyncEvent
				funcGroup.FuncSteps[k].AsyncMessage = funcStep.AsyncMessage
				funcGroup.FuncSteps[k].AsyncEventName = funcStep.AsyncEventName
				funcGroup.FuncSteps[k].Route.TokenSecretKey = tsk
			}
		}

		funcStep.FuncGroup = funcGroup
		logs.WithContext(ctx).Info(fmt.Sprint("FuncGroup set for  ", funcStep.FunctionName, " :", funcStep.FuncGroup))

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
			tg := functions.TargetHost{}
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
			logs.WithContext(ctx).Info(fmt.Sprint("making dummy route for api ", funcStep.GetRouteName(), " ", funcStep.FuncKey))
			r.RouteName = strings.Replace(strings.Replace(funcStep.Api.Host, ".", "", -1), ":", "", -1)
			r.RouteName = funcStep.GetRouteName()
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
		r.TokenSecretKey = ms.Projects[projectId].ProjectSettings.ClaimsKey
		funcStep.Route = r
	}
	for ck, cv := range funcStep.FuncSteps {
		fs := funcStep.FuncSteps[ck]
		fs.ParentFuncGroupName = funcStep.ParentFuncGroupName
		err = ms.LoadRoutesForFunction(ctx, fs, cv.RouteName, projectId, host, cv.Path, method, headers, s, tokenHeaderKey, reqBody)
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

func (ms *ModuleStore) SaveFunc(ctx context.Context, funcObj functions.FuncGroup, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug(fmt.Sprint("SaveFunc - Start"))
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	err = prj.AddFunc(ctx, funcObj)
	if persist == true {
		return realStore.SaveStore(ctx, projectId, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveFunc(ctx context.Context, funcName string, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug(fmt.Sprint("RemoveFunc - Start"))
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.FuncGroups[funcName]; ok {
			delete(prg.FuncGroups, funcName)
			logs.WithContext(ctx).Info(fmt.Sprint("SaveStore called from RemoveFunc"))
			return realStore.SaveStore(ctx, projectId, "", realStore)
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

func (ms *ModuleStore) GetFunctionNames(ctx context.Context, projectId string) (functions []string, err error) {
	logs.WithContext(ctx).Debug("GetFunctionNames - Start")
	if _, ok := ms.Projects[projectId]; ok {
		if ms.Projects[projectId].FuncGroups == nil {
			return
		} else {
			for k, _ := range ms.Projects[projectId].FuncGroups {
				functions = append(functions, k)
			}
			return
		}
	} else {
		err = errors.New(fmt.Sprint("Project ", projectId, " not found"))
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
		}
		return nil, err
	}
}

func (ms *ModuleStore) SaveWf(ctx context.Context, wfObj functions.Workflow, projectId string, realStore ModuleStoreI, persist bool) error {
	logs.WithContext(ctx).Debug(fmt.Sprint("SaveWf - Start"))
	if persist {
		realStore.GetMutex().Lock()
		defer realStore.GetMutex().Unlock()
	}
	prj, err := ms.GetProjectConfig(ctx, projectId)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	logs.WithContext(ctx).Info(fmt.Sprint(prj.Workflows))
	err = prj.AddWf(ctx, wfObj)
	logs.WithContext(ctx).Info(fmt.Sprint(prj.Workflows))
	if persist == true {
		return realStore.SaveStore(ctx, projectId, "", realStore)
	}
	return nil
}

func (ms *ModuleStore) RemoveWf(ctx context.Context, wfName string, projectId string, realStore ModuleStoreI) error {
	logs.WithContext(ctx).Debug(fmt.Sprint("RemoveWf - Start"))
	realStore.GetMutex().Lock()
	defer realStore.GetMutex().Unlock()
	if prg, ok := ms.Projects[projectId]; ok {
		if _, ok := prg.Workflows[wfName]; ok {
			delete(prg.Workflows, wfName)
			logs.WithContext(ctx).Info(fmt.Sprint("SaveStore called from RemoveWf"))
			return realStore.SaveStore(ctx, projectId, "", realStore)
		} else {
			err := errors.New(fmt.Sprint("Workflow ", wfName, " does not exists"))
			logs.WithContext(ctx).Error(err.Error())
			return err
		}
	} else {
		err := errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
}

func (ms *ModuleStore) GetWf(ctx context.Context, wfName string, projectId string, s ModuleStoreI) (cloneWf functions.Workflow, err error) {
	logs.WithContext(ctx).Debug("GetWf - Start")
	wfObj := functions.Workflow{}
	if prg, ok := ms.Projects[projectId]; ok {
		if wfObj, ok = prg.Workflows[wfName]; !ok {
			return wfObj, errors.New(fmt.Sprint("Workflow ", wfName, " does not exists"))
		}
		cloneWf, err = ms.GetWfCloneObject(ctx, projectId, wfObj, s)
		cloneWf.WfDb = db.GetDb(s.GetDbType())
		cloneWf.WfDb.SetConn(s.GetConn())
		return
	} else {
		return wfObj, errors.New(fmt.Sprint("Project ", projectId, " does not exists"))
	}
	return
}

func (ms *ModuleStore) GetWfCloneObject(ctx context.Context, projectId string, wfObj functions.Workflow, s ModuleStoreI) (cloneWf functions.Workflow, err error) {
	logs.WithContext(ctx).Debug("GetWfCloneObject - Start")

	wfObjJson, wfObjJsonErr := json.Marshal(wfObj)
	if wfObjJsonErr != nil {
		err = errors.New(fmt.Sprint("error while cloning wfObj (marshal)"))
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(wfObjJsonErr.Error())
		return
	}
	wfObjJson = s.ReplaceVariables(ctx, projectId, wfObjJson, nil)

	iCloneI := reflect.New(reflect.TypeOf(wfObj))
	wfObjCloneErr := json.Unmarshal(wfObjJson, iCloneI.Interface())
	if wfObjCloneErr != nil {
		err = errors.New(fmt.Sprint("error while cloning wfObj(unmarshal)"))
		logs.WithContext(ctx).Error(err.Error())
		logs.WithContext(ctx).Error(wfObjCloneErr.Error())
		return
	}
	return iCloneI.Elem().Interface().(functions.Workflow), nil
}

func (ms *ModuleStore) FetchAsyncEvent(ctx context.Context, asyncId string, asyncStatus string, s ModuleStoreI) (asyncFuncData AsyncFuncData, err error) {
	logs.WithContext(ctx).Debug("FetchAsyncEvent - Start")
	var selectQueries []*models.Queries
	selectQueryFuncAsync := models.Queries{}
	selectQueryFuncAsync.Query = db.GetDb(s.GetDbType()).GetDbQuery(ctx, SELECT_FUNC_ASYNC)
	selectQueryFuncAsync.Vals = append(selectQueryFuncAsync.Vals, asyncId, asyncStatus, asyncStatus)
	logs.WithContext(ctx).Info(fmt.Sprint(selectQueryFuncAsync.Vals))
	selectQueryFuncAsync.Rank = 1
	selectQueries = append(selectQueries, &selectQueryFuncAsync)
	selectOutput, err := eru_utils.ExecuteDbSave(ctx, s.GetConn(), selectQueries)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	var fVars functions.FuncTemplateVars
	if selectOutput[0] != nil {
		if selectOutput[0][0] != nil {
			fVarsBytes := []byte("")
			fVarsBytesOk := false
			if fVarsBytes, fVarsBytesOk = selectOutput[0][0]["event_msg"].([]byte); !fVarsBytesOk {
				logs.WithContext(ctx).Error(err.Error())
				return
			}
			err = json.Unmarshal(fVarsBytes, &fVars)
			if err != nil {
				logs.WithContext(ctx).Error(err.Error())
				return
			}
			asyncFuncData.FuncName = selectOutput[0][0]["func_name"].(string)
			asyncFuncData.FuncStepName = selectOutput[0][0]["func_step_name"].(string)
			asyncFuncData.AsyncId = selectOutput[0][0]["async_id"].(string)
			asyncFuncData.EventMsg = fVars
			asyncFuncData.EventRequest = selectOutput[0][0]["event_request"].(string)
			asyncFuncData.RequestId = selectOutput[0][0]["request_id"].(string)
		}
	}
	return
}

func (ms *ModuleStore) UpdateAsyncEvent(ctx context.Context, asyncId string, asyncStatus string, eventResponse string, s ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("UpdateAsyncEvent - Start")
	var updateQueries []*models.Queries
	updateQueryFuncAsync := models.Queries{}
	updateQueryFuncAsync.Query = db.GetDb(s.GetDbType()).GetDbQuery(ctx, UPDATE_FUNC_ASYNC)
	updateQueryFuncAsync.Vals = append(updateQueryFuncAsync.Vals, asyncStatus, eventResponse, asyncId)
	updateQueryFuncAsync.Rank = 1
	updateQueries = append(updateQueries, &updateQueryFuncAsync)
	_, err = eru_utils.ExecuteDbSave(ctx, s.GetConn(), updateQueries)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return
	}
	return
}

func (ms *ModuleStore) FetchProjectEvents(ctx context.Context, s ModuleStoreI) (err error) {
	logs.WithContext(ctx).Debug("FetchProjectEvents - Start")
	for _, p := range ms.Projects {
		if err != nil {
			return err
		}
		evts, err := s.FetchEvents(ctx, p.ProjectId)
		if err == nil {
			for _, e := range evts {
				err = ms.StartPolling(ctx, p.ProjectId, e, s)
				if err != nil {
					return err
				}
			}
		}
	}
	return
}

func (ms *ModuleStore) StartPolling(ctx context.Context, projectId string, event events.EventI, s ModuleStoreI) (err error) {
	eventName, _ := event.GetAttribute("event_name")
	logs.WithContext(ctx).Info(fmt.Sprint("StartPolling - Start : ", eventName))
	for {
		logs.WithContext(ctx).Info(fmt.Sprint("polling message for event : ", eventName))
		var eventJobs = make(chan functions.EventJob, 10)
		var eventResults = make(chan functions.EventResult, 10)
		//startTime := time.Now()
		go functions.AllocateEvent(ctx, event, eventJobs, EventThreads)
		done := make(chan bool)

		go func(done chan bool, eventResults chan functions.EventResult) {
			defer func() {
				if r := recover(); r != nil {
					logs.WithContext(ctx).Error(fmt.Sprint("goroutine panicked in StartPolling: ", r))
				}
			}()
			cnt := 0
			for res := range eventResults {
				logs.WithContext(ctx).Info(fmt.Sprint("result processing starting for job worker ", cnt))
				cnt = cnt + 1
				err = ms.ProcessEvents(ctx, projectId, res.EventMsgs, event, s, cnt)
				if err != nil {
					logs.WithContext(ctx).Error(err.Error())
					//ignore error and continue to poll
					err = nil
				}
				logs.WithContext(ctx).Info(fmt.Sprint("result processing ending for job worker ", cnt))
			}
			done <- true
		}(done, eventResults)

		//set it to one to run synchronously
		noOfWorkers := EventThreads
		functions.CreateWorkerPoolEvent(ctx, noOfWorkers, eventJobs, eventResults, s.GetConn())
		<-done
		/*
			if !msgRecd {
				ep, err := s.GetExtendedProjectConfig(ctx, projectId, s)
				var waitTime int32 = 5
				if err == nil {
					waitTime = ep.ProjectSettings.AsyncRepollWaitTime
				}
				logs.WithContext(ctx).Info(fmt.Sprint("waiting for next poll since no message retrived this time : ", waitTime))
				time.Sleep(time.Duration(waitTime) * time.Second)
			} else {
				logs.WithContext(ctx).Info(fmt.Sprint("next poll is immediate after processing the current messages : ", len(eventResults)))
			}
		*/
	}
}

func (ms *ModuleStore) ProcessEvents(ctx context.Context, projectId string, eventMsgs []events.EventMsg, event events.EventI, s ModuleStoreI, cnt int) (err error) {
	logs.WithContext(ctx).Debug("ProcessEvents - Start")
	aStatus := "PENDING"
	for i, m := range eventMsgs {
		startTime := time.Now()
		asyncStatus := "PROCESSED"
		var asyncFuncData AsyncFuncData
		logs.WithContext(ctx).Info("fetching event from db")
		logs.WithContext(ctx).Info(fmt.Sprint(m.Msg, " ", aStatus))
		failedCount := 0
		processedCount := 0
		asyncFuncData, err = ms.FetchAsyncEvent(ctx, m.Msg, aStatus, s)
		if err != nil || asyncFuncData.AsyncId == "" {
			failedCount = failedCount + 1
			asyncStatus = "FAILED"
			logs.WithContext(ctx).Error("event not found")
		} else {
			bodyMap := make(map[string]interface{})
			eventResponseBytes := []byte("")
			bodyMapOk := false
			requestBytes := []byte("")
			_ = requestBytes
			requestBytes, err = b64.StdEncoding.DecodeString(asyncFuncData.EventRequest)
			if err != nil {
				failedCount = failedCount + 1
				asyncStatus = "FAILED"
				logs.WithContext(ctx).Error("Function validation failed")
			} else {
				r := bufio.NewReader(bytes.NewBuffer(requestBytes))
				if eventReq, err := http.ReadRequest(r); err != nil { // deserialize request
					failedCount = failedCount + 1
					asyncStatus = "FAILED"
					logs.WithContext(ctx).Error(err.Error())
					logs.WithContext(ctx).Error("request deserialization failed")
				} else {
					if bodyMap, bodyMapOk = asyncFuncData.EventMsg.Vars.Body.(map[string]interface{}); !bodyMapOk {
						logs.WithContext(ctx).Error("Request Body count not be retrieved, setting it as blank")
					}
					funcGroup, err := ms.GetAndValidateFunc(ctx, asyncFuncData.FuncName, projectId, strings.Split(eventReq.Host, ":")[0], eventReq.URL.Path, eventReq.Method, eventReq.Header, bodyMap, s)
					if err != nil {
						failedCount = failedCount + 1
						asyncStatus = "FAILED"
						logs.WithContext(ctx).Error("Function validation failed")
					} else {
						reqBytes := []byte("")
						reqBytes, err = b64.StdEncoding.DecodeString(asyncFuncData.EventRequest)
						if err != nil {
							failedCount = failedCount + 1
							asyncStatus = "FAILED"
							logs.WithContext(ctx).Error("event request decoding failed")
						} else {
							var newReq *http.Request
							if newReq, err = http.ReadRequest(bufio.NewReader(bytes.NewReader(reqBytes))); err != nil { // deserialize request
								failedCount = failedCount + 1
								asyncStatus = "FAILED"
								logs.WithContext(ctx).Error("event request deserialization failed")
							}
							reqVars := make(map[string]*functions.TemplateVars)
							resVars := make(map[string]*functions.TemplateVars)
							if asyncFuncData.EventMsg.ReqVars != nil {
								reqVars = asyncFuncData.EventMsg.ReqVars
							}
							if asyncFuncData.EventMsg.ResVars != nil {
								resVars = asyncFuncData.EventMsg.ResVars
							}
							response, funcVarsMap, err := funcGroup.Execute(ctx, newReq, FuncThreads, LoopThreads, asyncFuncData.FuncStepName, "", true, reqVars, resVars)
							if err != nil {
								failedCount = failedCount + 1
								asyncStatus = "FAILED"
								logs.WithContext(ctx).Error(err.Error())
								logs.WithContext(ctx).Error("Function execution failed")
							} else {
								responseBytes := []byte("")
								responseBytes, err = io.ReadAll(response.Body)
								if err != nil {
									logs.WithContext(ctx).Error(err.Error())
									failedCount = failedCount + 1
									asyncStatus = "FAILED"
								} else {
									response.Body = io.NopCloser(bytes.NewBuffer(responseBytes))
									responseStr := string(responseBytes)
									eventResponse := make(map[string]interface{})
									eventResponse["response"] = responseStr
									eventResponse["func_vars"] = funcVarsMap
									eventResponseBytes, err = json.Marshal(eventResponse)
									if err != nil {
										logs.WithContext(ctx).Error(err.Error())
										failedCount = failedCount + 1
										asyncStatus = "FAILED"
									} else {
										logs.WithContext(ctx).Info(fmt.Sprint(response))
										eru_utils.PrintResponseBody(ctx, response, "printing response from async handler")
										processedCount = processedCount + 1
									}
								}
							}
							defer func() {
								if response != nil {
									response.Body.Close()
								}
							}()
						}
					}
				}
			}
			_ = s.UpdateAsyncEvent(ctx, m.Msg, asyncStatus, string(eventResponseBytes), s)
			_ = event.DeleteMessage(ctx, m.MsgIdentifer)
		}
		endTime := time.Now()
		diff := endTime.Sub(startTime)
		logs.WithContext(ctx).Info(fmt.Sprint("total time taken for message processing for job ", cnt, " and msg ", i, " is ", diff.Seconds(), "seconds"))
	}
	return
}
