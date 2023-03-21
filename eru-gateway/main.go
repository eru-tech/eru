package main

import (
	"github.com/eru-tech/eru/eru-gateway/module_server"
	"github.com/eru-tech/eru/eru-gateway/module_store"
	"github.com/eru-tech/eru/eru-server/server"
	"log"
	"os"
)

var port = "8086"

func main() {
	module_server.SetServiceName()
	log.Println("inside main of eru-gateway")
	envPort := os.Getenv("ERUGATEWAYPORT")
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
