module github.com/eru-tech/eru/eru-repos

go 1.20

require (
github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
github.com/eru-tech/eru/eru-utils v0.0.0-00010101000000-000000000000
)

require (
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
)

replace (
github.com/eru-tech/eru/eru-logs => ../eru-logs
github.com/eru-tech/eru/eru-utils => ../eru-utils
)
