module github.com/eru-tech/eru/eru-server

go 1.17

require (
	github.com/eru-tech/eru/eru-store v0.0.0-00010101000000-000000000000
	github.com/rs/cors v1.7.0
	github.com/segmentio/ksuid v1.0.3
	github.com/gorilla/mux v1.8.0
	github.com/eru-tech/eru/eru-utils v0.0.0-00010101000000-000000000000
)

replace (
	github.com/eru-tech/eru/eru-store => ../eru-store
	github.com/eru-tech/eru/eru-utils => ../eru-utils
)