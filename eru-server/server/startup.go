package server

import (
	"fmt"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-store/store"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"log"
	"net/http"
)

type Server struct {
	Store store.StoreI
}

func Launch(serverRouter *mux.Router, port string) {
	// Allow cors
	corsObj := handlers.MakeCorsObject()
	r := otelhttp.NewHandler(corsObj.Handler(requestIdMiddleWare(otelMiddleWare(serverRouter))), handlers.ServerName)
	http.Handle("/", r)
	logs.Logger.Info(fmt.Sprint("Starting server ", handlers.ServerName, " on ", port))
	err := http.ListenAndServe(":"+port, nil)
	log.Println("printing error of ListenAndServe")
	log.Println(err.Error())
	logs.Logger.Fatal(err.Error())

}
func Init(store store.StoreI) (*mux.Router, *Server, error) {
	s := new(Server)
	s.Store = store
	serverRouter := s.GetRouter()
	return serverRouter, s, nil
}
