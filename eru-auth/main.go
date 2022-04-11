package main

import (
	"github.com/eru-tech/eru/eru-auth/module_server"
	"github.com/eru-tech/eru/eru-auth/module_store"
	"github.com/eru-tech/eru/eru-server/server"
	"log"
	"os"
)

var port = "8085"

func main() {
	log.Println("inside main of eru-auth")
	envPort := os.Getenv("ERUAUTHPORT")
	if envPort != "" {
		port = envPort
	}
	store, e := module_server.StartUp()
	if e != nil {
		log.Println(e)
		log.Println("Failed to Start Server - error while setting up config store")
		return
	}
	sh := new(module_store.StoreHolder)
	sh.Store = store
	sr, _, e := server.Init()
	module_server.AddModuleRoutes(sr, sh)
	if e != nil {
		log.Print(e)
	}
	server.Launch(sr, port)
}
