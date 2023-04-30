package module_server

import (
	module_handlers "github.com/eru-tech/eru/eru-gateway/module_server/handlers"
	"github.com/eru-tech/eru/eru-gateway/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"net/http"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-gateway"
}
func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {

	//store routes specific to files
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()
	storeRouter.Methods(http.MethodPost).Path("/listenerrule/compare").HandlerFunc(module_handlers.StoreCompareHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/listenerrule/save").HandlerFunc(module_handlers.SaveListenerRuleHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/listenerrule/remove/{listenerrulename}").HandlerFunc(module_handlers.RemoveListenerRuleHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/listenerrule/list").HandlerFunc(module_handlers.GetListenerRulesHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/authorizer/save").HandlerFunc(module_handlers.SaveAuthorizerHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/authorizer/remove/{authorizername}").HandlerFunc(module_handlers.RemoveAuthorizerHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/authorizer/list").HandlerFunc(module_handlers.GetAuthorizerHandler(sh.Store))

	serverRouter.PathPrefix("/").HandlerFunc(module_handlers.RouteHandler(sh.Store))
}
