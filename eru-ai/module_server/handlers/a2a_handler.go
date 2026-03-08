package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/eru-tech/eru/eru-ai/agents"
	"github.com/eru-tech/eru/eru-ai/agents/a2a"
	"github.com/eru-tech/eru/eru-ai/module_store"
	function_module_store "github.com/eru-tech/eru/eru-functions/module_store"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/google/uuid"
)

func A2ATaskSubmitHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("A2ATaskSubmitHandler - Start")
		defer r.Body.Close()
		ctx := context.WithValue(r.Context(), "allowed_origins", server_handlers.AllowedOrigins)
		ctx = context.WithValue(ctx, "origin", r.Header.Get("Origin"))

		var body struct {
			Agent          string              `json:"agent"`
			Project        string              `json:"project"`
			Tenant         string              `json:"tenant"`
			ConversationId string              `json:"conversation_id"`
			Message        agents.AgentMessage `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}

		if claims := r.Header.Get("claims"); claims != "" {
			ctx = context.WithValue(ctx, "claims", claims)
		} else {
			ctx = context.WithValue(ctx, "claims", "")
		}
		ctx = context.WithValue(ctx, function_module_store.ContextKeyEruaibaseurl, module_store.Eruaibaseurl)
		ctx = context.WithValue(ctx, function_module_store.ContextKeyEruqlbaseurl, module_store.Eruqlbaseurl)

		conversationId := body.ConversationId
		if conversationId == "" {
			conversationId = uuid.New().String()
		}
		if body.Message.MessageId == "" {
			body.Message.MessageId = uuid.New().String()
		}

		agent, err := sh.Store.GetAgent(ctx, body.Project, body.Tenant, conversationId, body.Agent, sh.Store)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		res, err := agent.Execute(ctx, body.Message, conversationId, body.Project, body.Tenant)
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

		projectId := r.URL.Query().Get("project")
		tenantId := r.URL.Query().Get("tenant")

		descriptions, err := sh.Store.GetAgentDescriptions(ctx, projectId, tenantId)
		if err != nil {
			server_handlers.FormatResponse(w, http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		cards := make([]a2a.AgentCard, 0, len(descriptions))
		for name, desc := range descriptions {
			cards = append(cards, a2a.AgentCard{
				Name:        name,
				Description: desc,
				URL:         module_store.Eruaibaseurl + "/" + projectId + "/" + tenantId + "/execute/agent/" + name,
			})
		}
		server_handlers.FormatResponse(w, http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"agents": cards})
	}
}

func A2AWellKnownHandler(sh *module_store.StoreHolder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs.WithContext(r.Context()).Debug("A2AWellKnownHandler - Start")
		card := map[string]interface{}{
			"name":        "eru-ai",
			"description": "Eru AI Agent Service - multi-tenant AI agent execution platform",
			"url":         module_store.Eruaibaseurl,
			"version":     "1.0.0",
			"capabilities": map[string]interface{}{
				"streaming":         false,
				"pushNotifications": false,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(card)
	}
}
