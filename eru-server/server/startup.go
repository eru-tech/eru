package server

import (
	"fmt"
	handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-store/store"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type Server struct {
	Store store.StoreI
}

func Launch(serverRouter *mux.Router, port string) {
	// Allow cors
	corsObj := handlers.MakeCorsObject()
	r := corsObj.Handler(serverRouter)
	//log.Print(s)
	//r := s.GetRouter()
	http.Handle("/", r)
	log.Println(fmt.Sprint("Starting server ", handlers.ServerName, " on ", port))
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
func Init() (*mux.Router, *Server, error) {
	s := new(Server)
	//err := s.startUp()
	//if err != nil {
	//	log.Print(err)
	//	return nil , nil,  err
	//}
	serverRouter := s.GetRouter()
	return serverRouter, s, nil
}
