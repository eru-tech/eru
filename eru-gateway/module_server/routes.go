package module_server

import (
	module_handlers "github.com/eru-tech/eru/eru-gateway/module_server/handlers"
	"github.com/eru-tech/eru/eru-gateway/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"net/http"
)

func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {
	server_handlers.ServerName = "gateway"
	//store routes specific to files
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()

	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh.Store))

	// routes for file events
	apiRouter := serverRouter.PathPrefix("/routes/{project}").Subrouter()
	_ = apiRouter
	//apiRouter.Methods(http.MethodPost).Path("/{storagename}/upload").HandlerFunc(file_handlers.FileUploadHandler(sh.Store))

}
