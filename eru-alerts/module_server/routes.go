package module_server

import (
	module_handlers "github.com/eru-tech/eru/eru-alerts/module_server/handlers"
	"github.com/eru-tech/eru/eru-alerts/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"net/http"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-alerts"
}
func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {
	//store functions specific to files

	storeRouter := serverRouter.PathPrefix("/store").Subrouter()

	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/messagetemplate").HandlerFunc(module_handlers.MessageTemplateSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/messagetemplate/{templatename}").HandlerFunc(module_handlers.MessageTemplateRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/channel/{channeltype}").HandlerFunc(module_handlers.ChannelSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/channel/{channelname}").HandlerFunc(module_handlers.ChannelRemoveHandler(sh.Store))

	// functions for alert events
	apiRouter := serverRouter.PathPrefix("/alerts/{project}").Subrouter()
	apiRouter.Path("/{channelname}/{messagetemplate}").HandlerFunc(module_handlers.ExecuteAlertHandler(sh.Store))

	_ = apiRouter
	//apiRouter.Methods(http.MethodPost).Path("/{storagename}/upload").HandlerFunc(file_handlers.FileUploadHandler(sh.Store))

}
