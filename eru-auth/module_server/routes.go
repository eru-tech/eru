package module_server

import (
	module_handlers "github.com/eru-tech/eru/eru-auth/module_server/handlers"
	"github.com/eru-tech/eru/eru-auth/module_store"
	"github.com/gorilla/mux"
	"net/http"
)

func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {
	//store routes specific to files
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()

	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/smsgateway/{gatewayname}").HandlerFunc(module_handlers.SmsGatewaySaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/smsgateway/{gatewayname}").HandlerFunc(module_handlers.SmsGatewayRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/emailgateway/{gatewayname}").HandlerFunc(module_handlers.EmailGatewaySaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/emailgateway/{gatewayname}").HandlerFunc(module_handlers.EmailGatewayRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/messagetemplate").HandlerFunc(module_handlers.MessageTemplateSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/messagetemplate/{templatename}").HandlerFunc(module_handlers.MessageTemplateRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/gateway/{gatewaytype}/{channel}").HandlerFunc(module_handlers.GatewaySaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/gateway/{gatewayname}/{gatewaytype}/{channel}").HandlerFunc(module_handlers.GatewayRemoveHandler(sh.Store))

	// routes for file events
	authRouter := serverRouter.PathPrefix("/auth/{project}").Subrouter()
	authRouter.Methods(http.MethodGet).PathPrefix("/generateotp/{gatewaytype}/{channel}/{messagetype}").HandlerFunc(module_handlers.GenerateOtpHandler(sh.Store))
}
