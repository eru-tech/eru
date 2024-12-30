package module_server

import (
	"net/http"

	module_handlers "github.com/eru-tech/eru/eru-ai/module_server/handlers"
	"github.com/eru-tech/eru/eru-ai/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-ai"
}
func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {

	//store functions specific to files

	storeRouter := serverRouter.PathPrefix("/store").Subrouter()
	storeRouter.Methods(http.MethodPost).Path("/{project}/compare").HandlerFunc(module_handlers.StoreCompareHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/{tenant}/save/model").HandlerFunc(module_handlers.ModelSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/{tenant}/remove/model/{modelid}").HandlerFunc(module_handlers.ModelRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/settings/save").HandlerFunc(module_handlers.ProjectSetingsSaveHandler(sh.Store))

	// functions for ai events
	aiRouter := serverRouter.PathPrefix("/{project}").Subrouter()
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/{model}/query").HandlerFunc(module_handlers.ModelQueryHandler(sh.Store))
	aiRouter.Methods(http.MethodPost).PathPrefix("/{tenant}/{model}/{tool}/query").HandlerFunc(module_handlers.ModelQueryHandler(sh.Store))
}
