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
	//router.Methods(http.MethodPost).Path("/store/project/save/{project}").HandlerFunc(handlers.ProjectSaveHandler(s.Store))
	//router.Methods(http.MethodDelete).Path("/store/project/remove/{project}").HandlerFunc(handlers.ProjectRemoveHandler(s.Store))
	//router.Methods(http.MethodGet).Path("/store/project/list").HandlerFunc(handlers.ProjectListHandler(s.Store))
	//router.Methods(http.MethodGet).Path("/store/project/config/{project}").HandlerFunc(handlers.ProjectConfigHandler(s.Store))

	//router.Methods(http.MethodPost).Path("/file/upload").HandlerFunc(handlers.FileUploadHandler(s.Store))
	return router
}
