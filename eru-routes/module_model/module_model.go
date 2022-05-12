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
	ProjectId     string                  `eru:"required"`
	Routes        map[string]routes.Route `eru:"required"`
	ProjectConfig ProjectConfig
}

type TemplateVars struct {
	Headers          map[string]interface{}
	FormData         map[string]interface{}
	Params           map[string]interface{}
	Vars             map[string]interface{}
	Body             interface{}
	Token            interface{}
	FormDataKeyArray []string
}

func (prg *Project) AddRoute(routeObj routes.Route) error {
	prg.Routes[routeObj.RouteName] = routeObj
	log.Println(prg)
	return nil
}
