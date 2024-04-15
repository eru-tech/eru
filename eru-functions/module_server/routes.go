package module_server

import (
	module_handlers "github.com/eru-tech/eru/eru-functions/module_server/handlers"
	"github.com/eru-tech/eru/eru-functions/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"net/http"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-functions"
}
func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {

	//store functions specific to files
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()

	storeRouter.Methods(http.MethodPost).Path("/{project}/compare").HandlerFunc(module_handlers.StoreCompareHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/route/save").HandlerFunc(module_handlers.RouteSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/route/remove/{routename}").HandlerFunc(module_handlers.RouteRemoveHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/{project}/func/save").HandlerFunc(module_handlers.FuncSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/func/remove/{funcname}").HandlerFunc(module_handlers.FuncRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/func/run").HandlerFunc(module_handlers.FuncRunHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/{project}/wf/save").HandlerFunc(module_handlers.WfSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/wf/remove/{wfname}").HandlerFunc(module_handlers.WfRemoveHandler(sh.Store))
	//storeRouter.Methods(http.MethodPost).Path("/{project}/wf/run").HandlerFunc(module_handlers.WfRunHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/settings/save").HandlerFunc(module_handlers.ProjectSetingsSaveHandler(sh.Store))
	//storeRouter.Methods(http.MethodPost).Path("/{project}/authorizer/save").HandlerFunc(module_handlers.ProjectAuthorizerSaveHandler(sh.Store))
	//storeRouter.Methods(http.MethodDelete).Path("/{project}/authorizer/remove/{authorizername}").HandlerFunc(module_handlers.ProjectAuthorizerRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/template/execute").HandlerFunc(module_handlers.ExecuteTemplateHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/myquery/list").HandlerFunc(module_handlers.ProjectMyQueryListNamesHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/func/list").HandlerFunc(module_handlers.ProjectFunctionListHandler(sh.Store))
	// Adding routing handler to track all incoming requests
	serverRouter.PathPrefix("/{project}/route/{routename}").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	//serverRouter.PathPrefix("/public/{project}/route/{routename}").HandlerFunc(module_handlers.RouteHandler(sh.Store))

	serverRouter.PathPrefix("/{project}/func/{funcname}/{funcstepname}").HandlerFunc(module_handlers.FuncHandler(sh.Store))
	serverRouter.PathPrefix("/{project}/func/{funcname}").HandlerFunc(module_handlers.FuncHandler(sh.Store))
	//serverRouter.PathPrefix("/public/{project}/func/{funcname}").HandlerFunc(module_handlers.FuncHandler(sh.Store))

	serverRouter.PathPrefix("/{project}/wf/{wfname}").HandlerFunc(module_handlers.WfHandler(sh.Store))

	serverRouter.PathPrefix("/asynctest").HandlerFunc(module_handlers.RouteAsyncTestHandler(sh.Store))
	serverRouter.PathPrefix("/").HandlerFunc(module_handlers.RouteForwardHandler(sh.Store))

}
