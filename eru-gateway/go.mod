module github.com/eru-tech/eru/eru-gateway

go 1.17

require (
	github.com/eru-tech/eru/eru-server v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-store v0.0.0-00010101000000-000000000000
	github.com/gorilla/mux v1.8.0
)

require (
	github.com/jmoiron/sqlx v1.3.4 // indirect
	github.com/lib/pq v1.2.0 // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/segmentio/ksuid v1.0.3 // indirect
)

replace (
	github.com/eru-tech/eru/eru-server => ../eru-server
	github.com/eru-tech/eru/eru-store => ../eru-store
	github.com/eru-tech/eru/eru-utils => ../eru-utils
)
