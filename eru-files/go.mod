module github.com/eru-tech/eru/eru-files

go 1.17

require (
	github.com/aws/aws-sdk-go v1.43.19
	github.com/eru-tech/eru/eru-crypto v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-server v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-store v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-utils v0.0.0-00010101000000-000000000000
	github.com/gorilla/mux v1.8.0
	github.com/segmentio/ksuid v1.0.3
)

require (
	github.com/gabriel-vasile/mimetype v1.4.1
	github.com/gobwas/glob v0.2.3
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jmoiron/sqlx v1.3.4 // indirect
	github.com/lib/pq v1.2.0 // indirect
	github.com/rs/cors v1.7.0 // indirect
	golang.org/x/net v0.0.0-20220624214902-1bab6f366d9e // indirect
)

replace (
	github.com/eru-tech/eru/eru-crypto => ../eru-crypto
	github.com/eru-tech/eru/eru-server => ../eru-server
	github.com/eru-tech/eru/eru-store => ../eru-store
	github.com/eru-tech/eru/eru-utils => ../eru-utils
)
