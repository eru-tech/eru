package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	events "github.com/eru-tech/eru/eru-events/events"
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
	LaunchWithContext(context.Background(), serverRouter, port, store)
}

func LaunchWithContext(ctx context.Context, serverRouter *mux.Router, port string, store store.StoreI) {

	// Initialize logger with instance ID
	logs.LogInit(handlers.ServerName, handlers.InstanceId)

	// Get singleton goroutine manager
	gm := GetGlobalGoroutineManager(ctx)

	// Channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	var regClient *registration.RegistryClient
	var serverCleanupOnce sync.Once

	// Setup HTTP server with graceful shutdown
	server := &http.Server{
		Addr:    ":" + port,
		Handler: nil,
	}

	// Service Registration
	if handlers.ServerName != "eru-gateway" {
		registryURL := os.Getenv("ERUGATEWAY_URL")

		if registryURL != "" {
			var err error
			regClient, err = registration.NewRegistryClient(registryURL, handlers.ServerName, port, handlers.InstanceId, time.Now(), fmt.Sprintf("%v", store.GetUpdateTime()))
			if err != nil {
				logs.Logger.Error(fmt.Sprintf("Failed to create registry client: %v", err))
				err = nil
			} else {
				err := regClient.Register(gm.Context())
				if err != nil {
					logs.Logger.Error(fmt.Sprintf("Failed to register service: %v", err))
					err = nil
				} else {
					// Start heartbeating with non-critical restart behavior - keep service alive even if heartbeat fails
					gm.SafeGoWithRestartBehavior("heartbeat", func(ctx context.Context) {
						interval := 1 * time.Hour
						if os.Getenv("HEARTBEAT_INTERVAL") != "" {
							//expected format: 30s, 1m, 1h, 1d
							//if not valid, use default 1h
							interval, err = time.ParseDuration(os.Getenv("HEARTBEAT_INTERVAL"))
							if err != nil {
								logs.Logger.Error(fmt.Sprintf("Failed to parse heartbeat interval: %v", err))
								err = nil
							}
						}
						err = regClient.StartHeartbeat(ctx, interval)
						if err != nil {
							logs.Logger.Error(fmt.Sprintf("Failed to start heartbeat: %v", err))
							err = nil
						}
					}, ContinueOnMaxRetries)
				}
			}
		} else {
			logs.Logger.Warn("ERUGATEWAY_URL not set. Skipping service registration.")
		}
	}

	// Setup CORS and middleware
	handlers.AllowedOrigins = os.Getenv("ALLOWED_ORIGINS")
	logs.Logger.Info(fmt.Sprint("AllowedOrigins = ", handlers.AllowedOrigins))
	corsObj := handlers.MakeCorsObject()

	r := otelhttp.NewHandler(corsObj.Handler(panicRecoveryMiddleware(requestIdMiddleWare(otelMiddleWare(serverRouter)))), handlers.ServerName)
	http.Handle("/", r)

	// Start HTTP server with critical restart behavior - shutdown service if server fails
	gm.SafeGoWithRestartBehavior("http-server", func(ctx context.Context) {
		logs.Logger.Info(fmt.Sprint("Starting server ", handlers.ServerName, " on ", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logs.Logger.Error(fmt.Sprintf("HTTP server error: %v", err))
		}
	}, ShutdownOnMaxRetries)

	// Wait for shutdown signal
	select {
	case sig := <-stop:
		logs.Logger.Info(fmt.Sprintf("Received signal %v, initiating graceful shutdown", sig))
	case <-gm.Context().Done():
		logs.Logger.Info("Context cancelled, initiating graceful shutdown")
	}

	// Cleanup function
	cleanup := func() {
		// Shutdown HTTP server
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logs.Logger.Error(fmt.Sprintf("HTTP server shutdown error: %v", err))
		}

		// Deregister from gateway if applicable
		if regClient != nil {
			logs.Logger.Info("Attempting to deregister service and unsubscribe from config sync event")
			deregisterCtx, deregisterCancel := context.WithTimeout(ctx, 5*time.Second)
			defer deregisterCancel()

			if err := regClient.Deregister(deregisterCtx); err != nil {
				logs.Logger.Error(fmt.Sprintf("Deregistration error: %v", err))
			}

			// Config sync cleanup
			project_id := ""
			event_name := ""

			splitEventText := strings.Split(handlers.ConfigSyncEvent, "__")
			if len(splitEventText) == 2 {
				project_id = splitEventText[0]
				event_name = splitEventText[1]
			}

			if project_id != "" && event_name != "" {
				eventI, err := store.FetchEvent(ctx, project_id, event_name)
				if err != nil {
					logs.Logger.Error(fmt.Sprintf("Failed to fetch event for cleanup: %v", err))
				} else {
					err = eventI.Unsubscribe(ctx, handlers.InstanceId)
					if err != nil {
						logs.Logger.Error(fmt.Sprintf("Failed to unsubscribe from config sync: %v", err))
					} else {
						err = store.SaveStore(ctx, "", "", store)
						if err != nil {
							logs.Logger.Error(fmt.Sprintf("Failed to save store during cleanup: %v", err))
						}
					}
				}
			}
		}

		// Shutdown all goroutines
		if err := gm.Shutdown(15 * time.Second); err != nil {
			logs.Logger.Error(fmt.Sprintf("Goroutine shutdown error: %v", err))
		}

		// Reset singleton for potential restart
		ResetGlobalGoroutineManager()
	}

	serverCleanupOnce.Do(cleanup)
	logs.Logger.Info("Server shutdown completed")
}

func InitConfigSync(ctx context.Context, store store.StoreI, configEvent events.EventI, subscription map[string]interface{}, project_id string) {
	gm := GetGlobalGoroutineManager(ctx)

	gm.SafeGoWithRestartBehavior("config-sync-subscription", func(ctx context.Context) {
		time.Sleep(10 * time.Second)
		logs.Logger.Info(fmt.Sprintf("Subscribing to config sync event: %v", subscription))

		err := configEvent.Subscribe(ctx, subscription)
		if err != nil {
			logs.Logger.Error(fmt.Sprintf("Failed to subscribe to config sync event: %v", err))
			return
		}

		logs.WithContext(ctx).Info(fmt.Sprintf("Subscribed to config sync event: %v", configEvent))
		err = store.SaveStore(ctx, project_id, "", store)
		if err != nil {
			logs.Logger.Error(fmt.Sprintf("Failed to save store after subscribing to %s event: %v", handlers.ConfigSyncEvent, err))
		}

	}, ShutdownOnMaxRetries)
}
func Init(ctx context.Context, store store.StoreI) (*mux.Router, *Server, error) {
	_ = store.LoadSmValue(ctx, "")
	_ = store.LoadEnvValue(ctx, "")

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
		configEvent, err := store.FetchEvent(ctx, project_id, event_name)
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

			// Use InitConfigSync to handle subscription properly
			InitConfigSync(ctx, store, configEvent, subscription, project_id)
		}
	}

	s := new(Server)
	s.Store = store
	serverRouter := s.GetRouter()
	return serverRouter, s, nil
}
