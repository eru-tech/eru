package module_model

import (
	"context"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-routes/routes"
)

type ModuleProjectI interface {
}

type ProjectConfig struct {
	//AesKey         AesKey
	TokenSecret routes.TokenSecret
	//ProjectGitRepo ProjectGitRepo
}

type Project struct {
	ProjectId     string                      `eru:"required"`
	Routes        map[string]routes.Route     `eru:"required"`
	FuncGroups    map[string]routes.FuncGroup `eru:"required"`
	ProjectConfig ProjectConfig
	Authorizers   map[string]routes.Authorizer
}

func (prg *Project) AddRoute(ctx context.Context, routeObj routes.Route) error {
	logs.WithContext(ctx).Debug("AddRoute - Start")
	if prg.Routes == nil {
		prg.Routes = make(map[string]routes.Route)
	}
	prg.Routes[routeObj.RouteName] = routeObj
	return nil
}

func (prg *Project) AddFunc(ctx context.Context, funcObj routes.FuncGroup) error {
	logs.WithContext(ctx).Debug("AddFunc - Start")
	if prg.FuncGroups == nil {
		prg.FuncGroups = make(map[string]routes.FuncGroup)
	}
	prg.FuncGroups[funcObj.FuncGroupName] = funcObj
	return nil
}
