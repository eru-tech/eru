module github.com/eru-tech/eru/eru-db

go 1.22

require (
	github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
	github.com/jmoiron/sqlx v1.3.4
)

require (
	github.com/google/uuid v1.3.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
)

replace github.com/eru-tech/eru/eru-logs => ../eru-logs
