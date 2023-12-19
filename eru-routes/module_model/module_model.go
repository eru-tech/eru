package module_model

import (
	"context"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/routes"
	utils "github.com/eru-tech/eru/eru-utils"
	"github.com/google/go-cmp/cmp"
)

type StoreCompare struct {
	DeleteRoutes   []string
	NewRoutes      []string
	MismatchRoutes map[string]interface{}
	DeleteFuncs    []string
	NewFuncs       []string
	MismatchFuncs  map[string]interface{}
}

type ModuleProjectI interface {
	AddFunc(ctx context.Context, funcObj routes.FuncGroup) error
	AddRoute(ctx context.Context, routeObj routes.Route) error
	CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error)
}

//type ProjectConfig struct {
//	//AesKey         AesKey
//	TokenSecret routes.TokenSecret
//	//ProjectGitRepo ProjectGitRepo
//}

type ProjectSettings struct {
	ClaimsKey string `json:"claims_key" eru:"required"`
}

type Project struct {
	ProjectId       string                      `eru:"required"`
	Routes          map[string]routes.Route     `eru:"required"`
	FuncGroups      map[string]routes.FuncGroup `eru:"required"`
	ProjectSettings ProjectSettings             `json:"project_settings"`
	//Authorizers   map[string]routes.Authorizer
}

func (prj *Project) AddRoute(ctx context.Context, routeObj routes.Route) error {
	logs.WithContext(ctx).Debug("AddRoute - Start")
	if prj.Routes == nil {
		prj.Routes = make(map[string]routes.Route)
	}
	prj.Routes[routeObj.RouteName] = routeObj
	return nil
}

func (prj *Project) AddFunc(ctx context.Context, funcObj routes.FuncGroup) error {
	logs.WithContext(ctx).Debug("AddFunc - Start")
	if prj.FuncGroups == nil {
		prj.FuncGroups = make(map[string]routes.FuncGroup)
	}
	prj.FuncGroups[funcObj.FuncGroupName] = funcObj
	return nil
}

func (prj *Project) CompareProject(ctx context.Context, compareProject Project) (StoreCompare, error) {
	logs.WithContext(ctx).Debug("CompareProject - Start")
	storeCompare := StoreCompare{}
	for _, mr := range prj.Routes {
		var diffR utils.DiffReporter
		rFound := false
		for _, cr := range compareProject.Routes {
			if mr.RouteName == cr.RouteName {
				rFound = true
				if !cmp.Equal(mr, cr, cmp.Reporter(&diffR)) {
					if storeCompare.MismatchRoutes == nil {
						storeCompare.MismatchRoutes = make(map[string]interface{})
					}
					storeCompare.MismatchRoutes[mr.RouteName] = diffR.Output()
				}
				break
			}
		}
		if !rFound {
			storeCompare.DeleteRoutes = append(storeCompare.DeleteRoutes, mr.RouteName)
		}
	}

	for _, cr := range compareProject.Routes {
		rFound := false
		for _, mr := range prj.Routes {
			if mr.RouteName == cr.RouteName {
				rFound = true
				break
			}
		}
		if !rFound {
			storeCompare.NewRoutes = append(storeCompare.NewRoutes, cr.RouteName)
		}
	}

	//compare funcs
	for _, mf := range prj.FuncGroups {
		var diffR utils.DiffReporter
		fFound := false
		for _, cf := range compareProject.FuncGroups {
			if mf.FuncGroupName == cf.FuncGroupName {
				fFound = true
				if !cmp.Equal(mf, cf, cmp.Reporter(&diffR)) {
					if storeCompare.MismatchFuncs == nil {
						storeCompare.MismatchFuncs = make(map[string]interface{})
					}
					storeCompare.MismatchFuncs[mf.FuncGroupName] = diffR.Output()

				}
				break
			}
		}
		if !fFound {
			storeCompare.DeleteFuncs = append(storeCompare.DeleteFuncs, mf.FuncGroupName)
		}
	}

	for _, cf := range compareProject.FuncGroups {
		fFound := false
		for _, mf := range prj.FuncGroups {
			if mf.FuncGroupName == cf.FuncGroupName {
				fFound = true
				break
			}
		}
		if !fFound {
			storeCompare.NewFuncs = append(storeCompare.NewFuncs, cf.FuncGroupName)
		}
	}
	return storeCompare, nil
}
