package main

import (
	"fmt"
	"github.com/eru-tech/eru/eru-auth/module_server"
	"github.com/eru-tech/eru/eru-auth/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-server/server"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"os"
)

var port = "8085"

func main() {
	module_server.SetServiceName()
	logs.LogInit(server_handlers.ServerName)
	logs.Logger.Info(fmt.Sprint("inside main of ", server_handlers.ServerName))

	envPort := os.Getenv("ERUAUTHPORT")
	if envPort != "" {
		port = envPort
	}
	store, e := module_server.StartUp()
	if e != nil {
		logs.Logger.Error(e.Error())
		logs.Logger.Error("Failed to Start Server - error while setting up config store")
		return
	}
	sh := new(module_store.StoreHolder)
	sh.Store = store
	sr, _, e := server.Init()
	module_server.AddModuleRoutes(sr, sh)
	if e != nil {
		logs.Logger.Error(e.Error())
	}
	server.Launch(sr, port)
}
