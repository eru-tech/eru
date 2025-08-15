package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/eru-tech/eru/eru-server/registration"
	handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/eru-tech/eru/eru-store/store"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Server struct {
	Store store.StoreI
}

func Launch(serverRouter *mux.Router, port string, store store.StoreI) {

	// Initialize logger with instance ID
	logs.LogInit(handlers.ServerName, handlers.InstanceId)

	// Service Registration
	if handlers.ServerName != "eru-gateway" { //no need to register gateway in gateway

		registryURL := os.Getenv("ERUGATEWAY_URL")

		if registryURL != "" {
			regClient, err := registration.NewRegistryClient(registryURL, handlers.ServerName, port, handlers.InstanceId, time.Now(), fmt.Sprintf("%v", store.GetUpdateTime()))
			if err != nil {
				logs.Logger.Error(fmt.Sprintf("Failed to create registry client: %v", err))
				// Continue without registration
			} else {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				err := regClient.Register(ctx)
				if err != nil {
					logs.Logger.Error(fmt.Sprintf("Failed to register service: %v", err))
					// Depending on requirements, you might want to exit here
				} else {
					// Start heartbeating in a separate goroutine
					go regClient.StartHeartbeat(ctx, 30*time.Second)

					// Deregister on shutdown
					defer func() {
						logs.Logger.Info("Attempting to deregister service...")
						deregisterCtx, deregisterCancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer deregisterCancel()
						if err := regClient.Deregister(deregisterCtx); err != nil {
							logs.Logger.Error(err.Error())
						}
					}()
				}
			}
		} else {
			logs.Logger.Warn("ERUGATEWAY_URL not set. Skipping service registration.")
		}
	}

	// Allow cors
	handlers.AllowedOrigins = os.Getenv("ALLOWED_ORIGINS")
	logs.Logger.Info(fmt.Sprint("AllowedOrigins = ", handlers.AllowedOrigins))
	corsObj := handlers.MakeCorsObject()
	corsObjAllow := handlers.AllowCorsObject()

	rr := otelhttp.NewHandler(corsObjAllow.Handler(requestIdMiddleWare(otelMiddleWare(serverRouter))), handlers.ServerName)
	http.Handle("/x/", rr)
	r := otelhttp.NewHandler(corsObj.Handler(requestIdMiddleWare(otelMiddleWare(serverRouter))), handlers.ServerName)
	http.Handle("/", r)
	logs.Logger.Info(fmt.Sprint("Starting server ", handlers.ServerName, " on ", port))
	err := http.ListenAndServe(":"+port, nil)
	logs.Logger.Error(fmt.Sprint("printing error of ListenAndServe = ", err.Error()))
}
func Init(store store.StoreI) (*mux.Router, *Server, error) {
	_ = store.LoadSmValue(context.Background(), "")
	_ = store.LoadEnvValue(context.Background(), "")

	//ignore error from LoadSmValue and LoadEnvValue as server has to start even if load has failed.
	store.SetServiceName(handlers.ServerName)
	store.SetInstanceId(handlers.InstanceId)
	store.SetBaseUrl(handlers.BaseUrl)
	handlers.ConfigSyncEvent = os.Getenv("CONFIG_SYNC_EVENT")
	store.SetConfigSyncEvent(handlers.ConfigSyncEvent)

	logs.Logger.Info(fmt.Sprintf("Config sync event: %s", handlers.ConfigSyncEvent))
	logs.Logger.Info(fmt.Sprintf("Base url: %s", handlers.BaseUrl))
	if handlers.ConfigSyncEvent != "unknown" && handlers.BaseUrl != "" {
		project_id := ""
		event_name := ""

		splitEventText := strings.Split(handlers.ConfigSyncEvent, "__")
		if len(splitEventText) == 2 {
			project_id = splitEventText[0]
			event_name = splitEventText[1]
		}
		configEvent, err := store.FetchEvent(context.Background(), project_id, event_name)
		if err != nil {
			logs.Logger.Error(fmt.Sprintf("Failed to fetch config event: %v", err))
			err = nil
		} else {

			fp := map[string][]string{
				"service_name": {handlers.ServerName},
			}
			fpJson, err := json.Marshal(fp)
			if err != nil {
				logs.Logger.Error(fmt.Sprintf("Failed to marshal filter policy: %v", err))
				err = nil
			}
			logs.Logger.Info(fmt.Sprintf("Config sync endpoint: %s/%s?instance_id=%s", handlers.BaseUrl, handlers.ConfigSyncEvent, handlers.InstanceId))
			subscription := map[string]interface{}{
				"protocol":      "https",
				"endpoint":      fmt.Sprintf("%s/%s?instance_id=%s", handlers.BaseUrl, handlers.ConfigSyncEvent, handlers.InstanceId),
				"filter_policy": string(fpJson),
			}

			configEvent.Subscribe(context.Background(), subscription)
		}
	}

	s := new(Server)
	s.Store = store
	serverRouter := s.GetRouter()
	return serverRouter, s, nil
}
