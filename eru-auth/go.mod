module github.com/eru-tech/eru/eru-auth

go 1.17

require (
	github.com/eru-tech/eru/eru-crypto v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-server v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-store v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-utils v0.0.0-00010101000000-000000000000
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/sessions v1.2.1
	golang.org/x/oauth2 v0.0.0-20210323180902-22b0adad7558
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
)

require (
	cloud.google.com/go v0.65.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/goccy/go-json v0.9.6 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.1 // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/gorilla/securecookie v1.1.1 // indirect
	github.com/jmoiron/sqlx v1.3.4 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.0 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.1 // indirect
	github.com/lestrrat-go/jwx v1.2.23 // indirect
	github.com/lestrrat-go/option v1.0.0 // indirect
	github.com/lib/pq v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/segmentio/ksuid v1.0.3 // indirect
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292 // indirect
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
)

replace (
	github.com/eru-tech/eru/eru-crypto => ../eru-crypto
	github.com/eru-tech/eru/eru-server => ../eru-server
	github.com/eru-tech/eru/eru-store => ../eru-store
	github.com/eru-tech/eru/eru-utils => ../eru-utils
)
