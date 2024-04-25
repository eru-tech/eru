module github.com/eru-tech/eru/eru-cache

go 1.22.0

replace github.com/eru-tech/eru/eru-logs => ../eru-logs

require github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000

require (
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
)
