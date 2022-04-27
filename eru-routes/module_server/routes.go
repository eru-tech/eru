package module_server

import (
	module_handlers "github.com/eru-tech/eru/eru-routes/module_server/handlers"
	"github.com/eru-tech/eru/eru-routes/module_store"
	"github.com/gorilla/mux"
	"net/http"
)

func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {
	//store routes specific to files
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()

	storeRouter.Methods(http.MethodPost).Path("/{project}/route/save").HandlerFunc(module_handlers.RouteSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/route/remove/{routename}").HandlerFunc(module_handlers.RouteRemoveHandler(sh.Store))

	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh.Store))

	// Adding routing handler to track all incoming requests
	serverRouter.PathPrefix("/{project}/route/{routename}").HandlerFunc(module_handlers.RouteHandler(sh.Store))
	serverRouter.PathPrefix("/public/{project}/route/{routename}").HandlerFunc(module_handlers.RouteHandler(sh.Store))

}
