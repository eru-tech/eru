module github.com/eru-tech/eru/eru-db

go 1.22
require (
	github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
	github.com/jmoiron/sqlx v1.3.4
)

replace (
	github.com/eru-tech/eru/eru-logs => ../eru-logs
	)