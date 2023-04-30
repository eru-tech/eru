package server

import (
	handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/gorilla/mux"
	"net/http"
)

func (s *Server) GetRouter() *mux.Router {
	router := mux.NewRouter()
	router.Methods(http.MethodGet).Path("/hello").HandlerFunc(handlers.HelloHandler)
	router.Methods(http.MethodGet).Path("/echo").HandlerFunc(handlers.EchoHandler)

	router.Methods(http.MethodGet).Path("/store/{project}/variables/list").HandlerFunc(handlers.FetchVarsHandler(s.Store))
	router.Methods(http.MethodPost).Path("/store/{project}/variables/savevar").HandlerFunc(handlers.SaveVarHandler(s.Store))
	router.Methods(http.MethodDelete).Path("/store/{project}/variables/removevar/{key}").HandlerFunc(handlers.RemoveVarHandler(s.Store))
	router.Methods(http.MethodPost).Path("/store/{project}/variables/saveenvvar").HandlerFunc(handlers.SaveEnvVarHandler(s.Store))
	router.Methods(http.MethodDelete).Path("/store/{project}/variables/removeenvvar/{key}").HandlerFunc(handlers.RemoveEnvVarHandler(s.Store))
	router.Methods(http.MethodPost).Path("/store/{project}/variables/savesecret").HandlerFunc(handlers.SaveSecretHandler(s.Store))
	router.Methods(http.MethodDelete).Path("/store/{project}/variables/removesecret/{key}").HandlerFunc(handlers.RemoveSecretHandler(s.Store))

	//router.Methods(http.MethodPost).Path("/store/project/save/{project}").HandlerFunc(handlers.ProjectSaveHandler(s.Store))
	//router.Methods(http.MethodDelete).Path("/store/project/remove/{project}").HandlerFunc(handlers.ProjectRemoveHandler(s.Store))
	//router.Methods(http.MethodGet).Path("/store/project/list").HandlerFunc(handlers.ProjectListHandler(s.Store))
	//router.Methods(http.MethodGet).Path("/store/project/config/{project}").HandlerFunc(handlers.ProjectConfigHandler(s.Store))

	//router.Methods(http.MethodPost).Path("/file/upload").HandlerFunc(handlers.FileUploadHandler(s.Store))
	return router
}
