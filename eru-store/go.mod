module github.com/eru-tech/eru/eru-store

go 1.20

require (
	github.com/jmoiron/sqlx v1.3.4
	github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
)

replace (
	github.com/eru-tech/eru/eru-logs => ../eru-logs
)