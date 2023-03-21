package main

import (
	"github.com/eru-tech/eru/eru-rules/module_server"
	"github.com/eru-tech/eru/eru-rules/module_store"
	"github.com/eru-tech/eru/eru-server/server"
	"log"
	"os"
)

var port = "8084"

func main() {
	module_server.SetServiceName()
	log.Println("inside main of eru-rules")
	envPort := os.Getenv("ERURULESPORT")
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
