package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/agents/a2a"
	"github.com/eru-tech/eru/eru-ai/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
)

type A2AHandlers struct {
	Adapter  *a2a.Adapter
	Registry *agents.AgentRegistry
}

func getAdapter(ctx context.Context, sh *module_store.StoreHolder) *A2AHandlers {
	// Registry can be global/singleton; for now instantiate a new one
	reg := agents.NewAgentRegistry()
	// Register PlanningAgent under a default name
	pa := &agents.Agent{}
	reg.Register(ctx, "planning", pa)
	return &A2AHandlers{Adapter: a2a.NewAdapter(reg), Registry: reg}
}

func A2ATaskSubmitHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("A2ATaskSubmitHandler - Start")
		defer r.Body.Close()
		ctx := context.WithValue(r.Context(), "allowed_origins", server_handlers.AllowedOrigins)
		ctx = context.WithValue(ctx, "origin", r.Header.Get("Origin"))

		var body struct {
			Goal    string                 `json:"goal"`
			Plan    map[string]interface{} `json:"plan"`
			Context map[string]interface{} `json:"context"`
			Agent   string                 `json:"agent"`
			Project string                 `json:"project"`
			Tenant  string                 `json:"tenant"`
			Message agents.AgentMessage    `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}

		a := getAdapter(ctx, sh)
		agentName := body.Agent
		if agentName == "" {
			agentName = "planning"
		}

		// inject auth and trace headers for downstream propagation
		if auth := r.Header.Get("Authorization"); auth != "" {
			ctx = context.WithValue(ctx, "authorization", auth)
		}
		if tp := r.Header.Get("traceparent"); tp != "" {
			ctx = context.WithValue(ctx, "traceparent", tp)
		}

		res, err := a.Adapter.SubmitTask(ctx, agentName, body.Message, body.Project+":"+body.Tenant, body.Project, body.Tenant)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(res)
	}
}

func A2ATaskStatusHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("A2ATaskStatusHandler - Start")
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "unknown"})
	}
}

func A2AAgentDiscoverHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("A2AAgentDiscoverHandler - Start")
		ctx := context.WithValue(r.Context(), "allowed_origins", server_handlers.AllowedOrigins)
		ctx = context.WithValue(ctx, "origin", r.Header.Get("Origin"))
		a := getAdapter(ctx, sh)
		cards, _ := a.Adapter.Discover(ctx)
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"agents": cards})
	}
}
