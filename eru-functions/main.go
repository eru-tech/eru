package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/eru-tech/eru/eru-functions/module_server"
	"github.com/eru-tech/eru/eru-functions/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eruotel "github.com/eru-tech/eru/eru-logs/eru-otel"
	"github.com/eru-tech/eru/eru-server/server"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
)

var port = "8083"

func main() {
	defer func() {
		if r := recover(); r != nil {
			logs.Logger.Error(fmt.Sprint("Panic: ", r, " : ", string(debug.Stack())))
			os.Exit(1)
		}
	}()
	module_server.SetServiceName()
	server_handlers.SetInstanceId()
	logs.LogInit(server_handlers.ServerName, server_handlers.InstanceId)
	logs.Logger.Info(fmt.Sprint("inside main of ", server_handlers.ServerName))

	server_handlers.BaseUrl = os.Getenv("ERUFUNCTIONS_BASE_URL")

	traceUrl := os.Getenv("TRACE_URL")
	if traceUrl != "" {
		tp, err := eruotel.TracerTempoInit(traceUrl)

		if err != nil {
			logs.Logger.Fatal(err.Error())
		}
		defer func() {
			if err = tp.Shutdown(context.Background()); err != nil {
				logs.Logger.Error(fmt.Sprint("Error shutting down tracer provider: %v", err.Error()))
			}
		}()
	}
	envPort := os.Getenv("ERUFUNCTIONSPORT")
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
	sr, _, e := server.Init(sh.Store)
	module_server.AddModuleRoutes(sr, sh)
	if e != nil {
		logs.Logger.Error(e.Error())
	}

	asyncEventsList := []string{}
	asyncEvents := os.Getenv("ASYNC_EVENTS")
	if asyncEvents != "" {
		asyncEventsList = strings.Split(asyncEvents, ",")
	}
	logs.WithContext(context.Background()).Info(fmt.Sprint("asyncEventsList: ", asyncEventsList))
	if len(asyncEventsList) > 0 {
		for i := 0; i < module_store.EventThreads; i++ {
			err := sh.Store.FetchProjectEvents(context.Background(), sh.Store, i+1, asyncEventsList)
			if err != nil {
				logs.Logger.Error(err.Error())
			}
		}
	}

	server.Launch(sr, port, sh.Store)
}
