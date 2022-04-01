package module_model

import (
	"github.com/eru-tech/eru/eru-routes/routes"
	"log"
)

type ModuleProjectI interface {
}

type Project struct {
	ProjectId string                  `eru:"required"`
	Routes    map[string]routes.Route `eru:"required"`
}

func (prg *Project) AddRoute(routeObj routes.Route) error {
	prg.Routes[routeObj.RouteName] = routeObj
	log.Println(prg)
	return nil
}
