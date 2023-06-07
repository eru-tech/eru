package server

import (
	"fmt"
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
		if r.Method == "OPTIONS" {
			return
		}
		logs.Logger.Info("requestIdMiddleWare called")
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
		logs.Logger.Info("w.Header() from requestIdMiddleWare")
		logs.Logger.Info(fmt.Sprint(w.Header()))
		next.ServeHTTP(w, r)
	})
}

func otelMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logs.Logger.Info("otelMiddleWare called")
		requestID := r.Header.Get(server_handlers.RequestIdKey)
		pspan := oteltrace.SpanFromContext(r.Context())
		//if !span.IsRecording() {
		//	logs.WithContext(r.Context()).Info("Span not found - making new tracer")

		newCtx, span := otel.Tracer(server_handlers.ServerName).Start(r.Context(), "Initial", oteltrace.WithAttributes(attribute.String("requestID", requestID), attribute.String("traceID", pspan.SpanContext().TraceID().String()), attribute.String("spanID", pspan.SpanContext().SpanID().String())))
		defer span.End()
		r = r.WithContext(newCtx)
		//} else {
		//	logs.WithContext(r.Context()).Info("making child span")
		//	logs.WithContext(r.Context()).Info(fmt.Sprint(span.TracerProvider()))
		//	newCtx, span := span.TracerProvider().Tracer(server_handlers.ServerName).Start(r.Context(), "Initial", oteltrace.WithAttributes(attribute.String("requestId", requestID)))
		//	defer span.End()
		//	r = r.WithContext(newCtx)

		//}
		//w.Header().Set("trace_id", span.SpanContext().TraceID().String())
		logs.Logger.Info("w.Header() from otelMiddleWare")
		logs.Logger.Info(fmt.Sprint(w.Header()))
		next.ServeHTTP(w, r)
	})
}
