package server

import (
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"net/http"
)

func requestIdMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(server_handlers.RequestIdKey)
		if requestID == "" {
			// set a new request id header of request
			requestID = uuid.New().String()
			r.Header.Set(server_handlers.RequestIdKey, requestID)
		}
		r = r.WithContext(logs.NewContext(r.Context(), zap.String(server_handlers.RequestIdKey, requestID)))
		next.ServeHTTP(w, r)
	})
}

func otelMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(server_handlers.RequestIdKey)
		newCtx, span := otel.Tracer(server_handlers.ServerName).Start(r.Context(), "Initial", oteltrace.WithAttributes(attribute.String("requestId", requestID)))
		defer span.End()
		r = r.WithContext(newCtx)
		next.ServeHTTP(w, r)
	})
}
