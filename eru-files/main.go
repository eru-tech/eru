package main

import (
	"github.com/eru-tech/eru/eru-files/file_server"
	"github.com/eru-tech/eru/eru-files/module_store"
	"github.com/eru-tech/eru/eru-server/server"
	"log"
	"os"
)

var port = "8082"

func main() {
	file_server.SetServiceName()
	log.Println("inside main of eru-files")
	envPort := os.Getenv("ERUFILESPORT")
	if envPort != "" {
		port = envPort
	}
	store, e := file_server.StartUp()
	if e != nil {
		log.Println(e)
		log.Println("Failed to Start Server - error while setting up config store")
		return
	}
	sh := new(module_store.StoreHolder)
	sh.Store = store
	sr, _, e := server.Init()
	file_server.AddFileRoutes(sr, sh)
	if e != nil {
		log.Print(e)
	}
	server.Launch(sr, port)
}
