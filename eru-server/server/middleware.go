package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	server_handlers "github.com/eru-tech/eru/eru-server/server/handlers"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	claimsKey       string = "claims"
	eruqlbaseurlKey string = "eruqlbaseurl"
)

func requestIdMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			return
		}
		requestID := r.Header.Get(server_handlers.RequestIdKey)
		if requestID == "" {
			// set a new request id header of request
			requestID = uuid.New().String()
			r.Header.Set(server_handlers.RequestIdKey, requestID)
		}
		spanId := oteltrace.SpanFromContext(r.Context()).SpanContext().SpanID().String()
		traceId := oteltrace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
		if spanId == "0000000000000000" {
			spanId = ""
		}
		if traceId == "00000000000000000000000000000000" {
			traceId = ""
		}
		r = r.WithContext(logs.NewContext(r.Context(), zap.String(server_handlers.RequestIdKey, requestID), zap.String("spanID", spanId), zap.String("traceID", traceId)))
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func otelMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(server_handlers.RequestIdKey)
		pspan := oteltrace.SpanFromContext(r.Context())
		//if !span.IsRecording() {
		//	logs.WithContext(r.Context()).Info("Span not found - making new tracer")

		newCtx, span := otel.Tracer(server_handlers.ServerName).Start(r.Context(), "Initial", oteltrace.WithAttributes(attribute.String("requestID", requestID), attribute.String("traceID", pspan.SpanContext().TraceID().String()), attribute.String("spanID", pspan.SpanContext().SpanID().String())))
		defer span.End()

		newCtx = context.WithValue(newCtx, claimsKey, fmt.Sprint(r.Header.Get("claims")))
		newCtx = context.WithValue(newCtx, eruqlbaseurlKey, fmt.Sprint(server_handlers.EruqlBaseUrl))
		r = r.WithContext(newCtx)
		//} else {
		//	logs.WithContext(r.Context()).Info("making child span")
		//	logs.WithContext(r.Context()).Info(fmt.Sprint(span.TracerProvider()))
		//	newCtx, span := span.TracerProvider().Tracer(server_handlers.ServerName).Start(r.Context(), "Initial", oteltrace.WithAttributes(attribute.String("requestId", requestID)))
		//	defer span.End()
		//	r = r.WithContext(newCtx)

		//}
		w.Header().Set("trace_id", span.SpanContext().TraceID().String())
		w.Header().Set(server_handlers.RequestIdKey, requestID)
		next.ServeHTTP(w, r)
	})
}

func panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Log the panic with full context
				panicMsg := fmt.Sprintf("HTTP handler panic: %v\nStack trace:\n%s", rec, string(debug.Stack()))
				err := errors.New(panicMsg)
				if logs.Logger != nil {
					err = logs.Err(r.Context(), err, "")
				} else {
					fmt.Printf("ERROR: %s\n", err.Error())
				}

				// Return 500 error to client
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				errorResponse := map[string]interface{}{
					"error":   "Internal server error",
					"message": err.Error(),
				}

				if requestId := r.Context().Value("requestId"); requestId != nil {
					errorResponse["request_id"] = requestId
				}

				json.NewEncoder(w).Encode(errorResponse)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
