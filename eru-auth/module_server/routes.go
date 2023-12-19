package module_server

import (
	module_handlers "github.com/eru-tech/eru/eru-auth/module_server/handlers"
	"github.com/eru-tech/eru/eru-auth/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"net/http"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-auth"
}
func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {

	//store routes specific to files
	//serverRouter.Path("/auth/google/login").HandlerFunc(module_handlers.OauthGoogleLogin())
	//serverRouter.Path("/auth/google/callback").HandlerFunc(module_handlers.OauthGoogleCallback())
	//serverRouter.Path("/auth/openid/login").HandlerFunc(module_handlers.OpenIdLoginHandler(sh.Store))
	//serverRouter.Path("/auth/openid/callback").HandlerFunc(module_handlers.OpenIdCallbackHandler(sh.Store))
	//serverRouter.Path("/auth/openid/getloginflow/{loginchallenge}").HandlerFunc(module_handlers.GetLoginFlowHandlerandler(sh.Store))

	storeRouter := serverRouter.PathPrefix("/store").Subrouter()
	storeRouter.Methods(http.MethodPost).Path("/{project}/compare").HandlerFunc(module_handlers.StoreCompareHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh.Store))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh.Store))
	//storeRouter.Methods(http.MethodPost).Path("/{project}/save/smsgateway/{gatewayname}").HandlerFunc(module_handlers.SmsGatewaySaveHandler(sh.Store))
	//storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/smsgateway/{gatewayname}").HandlerFunc(module_handlers.SmsGatewayRemoveHandler(sh.Store))
	//storeRouter.Methods(http.MethodPost).Path("/{project}/save/emailgateway/{gatewayname}").HandlerFunc(module_handlers.EmailGatewaySaveHandler(sh.Store))
	//storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/emailgateway/{gatewayname}").HandlerFunc(module_handlers.EmailGatewayRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/messagetemplate").HandlerFunc(module_handlers.MessageTemplateSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/messagetemplate/{templatename}").HandlerFunc(module_handlers.MessageTemplateRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/gateway/{gatewaytype}/{channel}").HandlerFunc(module_handlers.GatewaySaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/gateway/{gatewayname}/{gatewaytype}/{channel}").HandlerFunc(module_handlers.GatewayRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/auth").HandlerFunc(module_handlers.AuthSaveHandler(sh.Store))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/auth/{authname}").HandlerFunc(module_handlers.AuthRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/testemail").HandlerFunc(module_handlers.TestEmail(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/settings/save").HandlerFunc(module_handlers.ProjectSetingsSaveHandler(sh.Store))

	// routes for file events
	authRouter := serverRouter.PathPrefix("/{project}").Subrouter()
	authRouter.Methods(http.MethodGet).PathPrefix("/generateotp/{gatewaytype}/{channel}/{messagetype}").HandlerFunc(module_handlers.GenerateOtpHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/getrecoverycode").HandlerFunc(module_handlers.GetRecoveryCodeHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/getverifycode").HandlerFunc(module_handlers.GetVerifyCodeHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/verifyrecoverycode").HandlerFunc(module_handlers.VerifyRecoveryCodeHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/checkverifycode").HandlerFunc(module_handlers.CheckVerifyCodeHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/completerecovery").HandlerFunc(module_handlers.CompleteRecoveryHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/login").HandlerFunc(module_handlers.LoginHandler(sh.Store))
	authRouter.Methods(http.MethodDelete).PathPrefix("/{authname}/logout").HandlerFunc(module_handlers.LogoutHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/verify/{tokentype}").HandlerFunc(module_handlers.VerifyTokenHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/userinfo").HandlerFunc(module_handlers.UserInfoHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/fetchtokens").HandlerFunc(module_handlers.FetchTokensHandler(sh.Store))
	authRouter.Methods(http.MethodGet).PathPrefix("/{authname}/getuser").HandlerFunc(module_handlers.GetUserHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/updateuser").HandlerFunc(module_handlers.UpdateUserHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/changepassword").HandlerFunc(module_handlers.ChangePasswordHandler(sh.Store))
	authRouter.Methods(http.MethodGet).PathPrefix("/{authname}/getssourl").HandlerFunc(module_handlers.GetSsoUrlHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/register").HandlerFunc(module_handlers.RegisterHandler(sh.Store))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/removeidentity").HandlerFunc(module_handlers.RemoveIdentityHandler(sh.Store))

}
