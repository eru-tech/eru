package module_model

import (
	"github.com/eru-tech/eru/eru-routes/routes"
	"log"
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

func (prg *Project) AddRoute(routeObj routes.Route) error {
	if prg.Routes == nil {
		prg.Routes = make(map[string]routes.Route)
	}
	prg.Routes[routeObj.RouteName] = routeObj
	log.Println(prg)
	return nil
}

func (prg *Project) AddFunc(funcObj routes.FuncGroup) error {
	if prg.FuncGroups == nil {
		prg.FuncGroups = make(map[string]routes.FuncGroup)
	}
	prg.FuncGroups[funcObj.FuncGroupName] = funcObj
	//log.Println(prg)
	return nil
}
