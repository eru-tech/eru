module github.com/eru-tech/eru/eru-server

go 1.20

require (
	github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-store v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-secret-manager v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/rs/cors v1.7.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.42.0
	go.opentelemetry.io/otel v1.16.0
	go.opentelemetry.io/otel/trace v1.16.0
	go.uber.org/zap v1.24.0
)

require (
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/jmoiron/sqlx v1.3.4 // indirect
	github.com/lib/pq v1.2.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
)

replace (
	github.com/eru-tech/eru/eru-logs => ../eru-logs
	github.com/eru-tech/eru/eru-store => ../eru-store
	github.com/eru-tech/eru/eru-repos => ../eru-repos
	github.com/eru-tech/eru/eru-secret-manager => ../eru-secret-manager
)
