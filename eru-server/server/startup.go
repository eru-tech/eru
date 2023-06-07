package server

import (
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-store/store"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"net/http"
	"os"
)

type Server struct {
	Store store.StoreI
}

func Launch(serverRouter *mux.Router, port string) {
	// Allow cors
	handlers.AllowedOrigins = os.Getenv("ALLOWED_ORIGINS")
	corsObj := handlers.MakeCorsObject()
	r := otelhttp.NewHandler(corsObj.Handler(requestIdMiddleWare(otelMiddleWare(serverRouter))), handlers.ServerName)
	//r := corsObj.Handler(serverRouter)
	http.Handle("/", r)
	logs.Logger.Info(fmt.Sprint("Starting server ", handlers.ServerName, " on ", port))
	err := http.ListenAndServe(":"+port, nil)
	logs.Logger.Error(fmt.Sprint("printing error of ListenAndServe = ", err.Error()))
}
func Init(store store.StoreI) (*mux.Router, *Server, error) {
	s := new(Server)
	s.Store = store
	serverRouter := s.GetRouter()
	return serverRouter, s, nil
}
