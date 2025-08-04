package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"

	"github.com/eru-tech/eru/eru-cache/cache"
	"github.com/eru-tech/eru/eru-gateway/module_server"
	"github.com/eru-tech/eru/eru-gateway/module_server/handlers"
	"github.com/eru-tech/eru/eru-gateway/module_store"
	"github.com/eru-tech/eru/eru-gateway/registry"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	eruotel "github.com/eru-tech/eru/eru-logs/eru-otel"
	"github.com/eru-tech/eru/eru-server/server"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
)

var port = "8086"

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
	traceUrl := os.Getenv("TRACE_URL")
	if traceUrl != "" {
		tp, err := eruotel.TracerTempoInit(traceUrl)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err = tp.Shutdown(context.Background()); err != nil {
				logs.Logger.Error(fmt.Sprint("Error shutting down tracer provider: %v", err.Error()))
			}
		}()
	}
	envPort := os.Getenv("ERUGATEWAYPORT")
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

	// Create the service registry
	registryType := os.Getenv("REGISTRY_TYPE")
	if registryType == "" {
		registryType = "INMEMORY" // Default to in-memory if not specified or invalid
	}
	logs.Logger.Info(fmt.Sprintf("Using %s registry", registryType))

	registryCache := cache.GetCacheStore(registryType)
	logs.Logger.Info(fmt.Sprintf("registryCache: %v", registryCache))
	if registryCache == nil {
		logs.Logger.Error(fmt.Sprintf("failed to connect to registry cache: %v - fallback to in-memory", registryType))
		registryCache = cache.GetCacheStore("INMEMORY")
	}
	serviceRegistry := registry.NewRegistry(registryCache, 90*time.Second)
	rh := &handlers.RegistryHandler{Registry: serviceRegistry}

	sr, _, e := server.Init(sh.Store)
	module_server.AddModuleRoutes(sr, sh, rh)
	if e != nil {
		logs.Logger.Error(e.Error())
	}
	server.Launch(sr, port)
}
