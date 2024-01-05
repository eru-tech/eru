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

	//overwriting the handler of eru-server as gateway does not need variables and has to route to different services
	serverRouter.Get("variables_list").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("variables_savevar").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("variables_removevar").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("variables_saveenvvar").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("variables_removeenvvar").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("variables_savesecret").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("variables_removesecret").HandlerFunc(module_handlers.RouteHandler(sh.Store))

	serverRouter.Get("repo_list").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("repo_save").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("repo_save_token").HandlerFunc(module_handlers.RouteHandler(sh.Store))

	serverRouter.Get("sm").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("sm_list").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.Get("sm_value").HandlerFunc(module_handlers.RouteHandler(sh.Store))

	//store routes specific to files
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()

	storeRouter.Methods(http.MethodPost).Path("/compare").HandlerFunc(module_handlers.StoreCompareHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/config").HandlerFunc(module_handlers.GetConfigHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/listenerrule/save").HandlerFunc(module_handlers.SaveListenerRuleHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/listenerrule/remove/{listenerrulename}").HandlerFunc(module_handlers.RemoveListenerRuleHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/listenerrule/list").HandlerFunc(module_handlers.GetListenerRulesHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/authorizer/save").HandlerFunc(module_handlers.SaveAuthorizerHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/authorizer/remove/{authorizername}").HandlerFunc(module_handlers.RemoveAuthorizerHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/authorizer/list").HandlerFunc(module_handlers.GetAuthorizerHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/settings/save").HandlerFunc(module_handlers.ProjectSetingsSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/settings").HandlerFunc(module_handlers.GetProjectSetingsHandler(sh.Store))

	serverRouter.PathPrefix("/").HandlerFunc(module_handlers.RouteHandler(sh.Store))
}
