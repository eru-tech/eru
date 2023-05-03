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
		spanId := oteltrace.SpanFromContext(r.Context()).SpanContext().SpanID().String()
		traceId := oteltrace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
		if spanId == "0000000000000000" {
			spanId = ""
		}
		if traceId == "00000000000000000000000000000000" {
			traceId = ""
		}
		r = r.WithContext(logs.NewContext(r.Context(), zap.String(server_handlers.RequestIdKey, requestID), zap.String("spanID", spanId), zap.String("traceID", traceId)))
		next.ServeHTTP(w, r)
	})
}

func otelMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(server_handlers.RequestIdKey)
		span := oteltrace.SpanFromContext(r.Context())
		//if !span.IsRecording() {
		//	logs.WithContext(r.Context()).Info("Span not found - making new tracer")
		newCtx, span := otel.Tracer(server_handlers.ServerName).Start(r.Context(), "Initial", oteltrace.WithAttributes(attribute.String("requestId", requestID)))
		defer span.End()
		r = r.WithContext(newCtx)
		//} else {
		//	logs.WithContext(r.Context()).Info("making child span")
		//	logs.WithContext(r.Context()).Info(fmt.Sprint(span.TracerProvider()))
		//	newCtx, span := span.TracerProvider().Tracer(server_handlers.ServerName).Start(r.Context(), "Initial", oteltrace.WithAttributes(attribute.String("requestId", requestID)))
		//	defer span.End()
		//	r = r.WithContext(newCtx)

		//}
		w.Header().Set("trace_id", span.SpanContext().TraceID().String())
		next.ServeHTTP(w, r)
	})
}
