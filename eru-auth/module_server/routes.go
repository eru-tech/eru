package module_server

import (
	"net/http"

	module_handlers "github.com/eru-tech/eru/eru-auth/module_server/handlers"
	"github.com/eru-tech/eru/eru-auth/module_store"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
)

func SetServiceName() {
	server_handlers.ServerName = "eru-auth"
}
func AddModuleRoutes(serverRouter *mux.Router, sh *module_store.StoreHolder) {

	//store functions specific to files
	//serverRouter.Path("/auth/google/login").HandlerFunc(module_handlers.OauthGoogleLogin())
	//serverRouter.Path("/auth/google/callback").HandlerFunc(module_handlers.OauthGoogleCallback())
	//serverRouter.Path("/auth/openid/login").HandlerFunc(module_handlers.OpenIdLoginHandler(sh.Store))
	//serverRouter.Path("/auth/openid/callback").HandlerFunc(module_handlers.OpenIdCallbackHandler(sh.Store))
	//serverRouter.Path("/auth/openid/getloginflow/{loginchallenge}").HandlerFunc(module_handlers.GetLoginFlowHandlerandler(sh.Store))
	serverRouter.Methods(http.MethodPost).Path("/{event_name}").HandlerFunc(module_handlers.ConfigSyncHandler(sh))
	storeRouter := serverRouter.PathPrefix("/store").Subrouter()
	storeRouter.Methods(http.MethodGet).Path("/load").HandlerFunc(module_handlers.StoreLoadHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/compare").HandlerFunc(module_handlers.StoreCompareHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save").HandlerFunc(module_handlers.ProjectSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove").HandlerFunc(module_handlers.ProjectRemoveHandler(sh))
	storeRouter.Methods(http.MethodGet).Path("/project/list").HandlerFunc(module_handlers.ProjectListHandler(sh))
	storeRouter.Methods(http.MethodGet).Path("/{project}/config").HandlerFunc(module_handlers.ProjectConfigHandler(sh))
	//storeRouter.Methods(http.MethodPost).Path("/{project}/save/smsgateway/{gatewayname}").HandlerFunc(module_handlers.SmsGatewaySaveHandler(sh.Store))
	//storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/smsgateway/{gatewayname}").HandlerFunc(module_handlers.SmsGatewayRemoveHandler(sh.Store))
	//storeRouter.Methods(http.MethodPost).Path("/{project}/save/emailgateway/{gatewayname}").HandlerFunc(module_handlers.EmailGatewaySaveHandler(sh.Store))
	//storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/emailgateway/{gatewayname}").HandlerFunc(module_handlers.EmailGatewayRemoveHandler(sh.Store))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/messagetemplate").HandlerFunc(module_handlers.MessageTemplateSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/messagetemplate/{templatename}").HandlerFunc(module_handlers.MessageTemplateRemoveHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/gateway/{gatewaytype}/{channel}").HandlerFunc(module_handlers.GatewaySaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/gateway/{gatewayname}/{gatewaytype}/{channel}").HandlerFunc(module_handlers.GatewayRemoveHandler(sh))
	storeRouter.Methods(http.MethodPost).Path("/{project}/save/auth").HandlerFunc(module_handlers.AuthSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/auth/{authname}").HandlerFunc(module_handlers.AuthRemoveHandler(sh))

	storeRouter.Methods(http.MethodPost).Path("/{project}/save/kid").HandlerFunc(module_handlers.KidSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/remove/kid/{kid}").HandlerFunc(module_handlers.KidRemoveHandler(sh))

	storeRouter.Methods(http.MethodPost).Path("/{project}/create/api_token").HandlerFunc(module_handlers.ApiTokenSaveHandler(sh))
	storeRouter.Methods(http.MethodDelete).Path("/{project}/revoke/api_token/{token_id}").HandlerFunc(module_handlers.ApiTokenRemoveHandler(sh))
	storeRouter.Methods(http.MethodGet).Path("/{project}/list/api_token/{identity_id}").HandlerFunc(module_handlers.ApiTokenListHandler(sh))

	storeRouter.Methods(http.MethodPost).Path("/{project}/settings/save").HandlerFunc(module_handlers.ProjectSetingsSaveHandler(sh))
	storeRouter.Methods(http.MethodGet).Path("/{project}/func/list").HandlerFunc(module_handlers.ProjectFunctionListHandler(sh))

	// functions for file events
	authRouter := serverRouter.PathPrefix("/{project}").Subrouter()
	authRouter.Methods(http.MethodGet).PathPrefix("/generateotp/{gatewaytype}/{channel}/{messagetype}").HandlerFunc(module_handlers.GenerateOtpHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/getrecoverycode").HandlerFunc(module_handlers.GetRecoveryCodeHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/getverifycode").HandlerFunc(module_handlers.GetVerifyCodeHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/verifyrecoverycode").HandlerFunc(module_handlers.VerifyRecoveryCodeHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/checkverifycode").HandlerFunc(module_handlers.CheckVerifyCodeHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/completerecovery").HandlerFunc(module_handlers.CompleteRecoveryHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/login/api").HandlerFunc(module_handlers.LoginApiHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/login").HandlerFunc(module_handlers.LoginHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/idptoken/{renew}").HandlerFunc(module_handlers.IdpTokenHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/idptoken").HandlerFunc(module_handlers.IdpTokenHandler(sh))
	authRouter.Methods(http.MethodDelete).PathPrefix("/{authname}/logout").HandlerFunc(module_handlers.LogoutHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/verify/{tokentype}").HandlerFunc(module_handlers.VerifyTokenHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/userinfo").HandlerFunc(module_handlers.UserInfoHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/fetchtokens").HandlerFunc(module_handlers.FetchTokensHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/gettokens").HandlerFunc(module_handlers.GetTokensHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/getusertoken").HandlerFunc(module_handlers.GetUserTokensHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/generatetempcode").HandlerFunc(module_handlers.GenerateTempCodeHandler(sh))
	authRouter.Methods(http.MethodGet).PathPrefix("/{authname}/getuser").HandlerFunc(module_handlers.GetUserHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/updateuser").HandlerFunc(module_handlers.UpdateUserHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/edituser").HandlerFunc(module_handlers.EditUserHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/changepassword").HandlerFunc(module_handlers.ChangePasswordHandler(sh))
	authRouter.Methods(http.MethodGet).PathPrefix("/{authname}/getssourl").HandlerFunc(module_handlers.GetSsoUrlHandler(sh))
	authRouter.Methods(http.MethodPost).PathPrefix("/{authname}/register").HandlerFunc(module_handlers.RegisterHandler(sh))
	authRouter.Methods(http.MethodDelete).PathPrefix("/{authname}/removeidentity").HandlerFunc(module_handlers.RemoveIdentityHandler(sh))
	authRouter.Methods(http.MethodGet).Path("/.well-known/jwks.json/{kid}").HandlerFunc(module_handlers.JWKHandler(sh))
}
