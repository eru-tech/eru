package main

import (
	"context"
	"fmt"
	"github.com/eru-tech/eru/eru-alerts/module_server"
	"github.com/eru-tech/eru/eru-alerts/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eruotel "github.com/eru-tech/eru/eru-logs/eru-otel"
	"github.com/eru-tech/eru/eru-server/server"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"log"
	"os"
)

var port = "8088"

func main() {
	module_server.SetServiceName()
	logs.LogInit(server_handlers.ServerName)
	logs.Logger.Info(fmt.Sprint("inside main of ", server_handlers.ServerName))

	tp, err := eruotel.TracerInit()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = tp.Shutdown(context.Background()); err != nil {
			logs.Logger.Error(fmt.Sprint("Error shutting down tracer provider: %v", err.Error()))
		}
	}()

	envPort := os.Getenv("ERUALERTSPORT")
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
