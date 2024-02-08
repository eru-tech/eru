module github.com/eru-tech/eru/eru-crypto

go 1.20

require (
	github.com/golang-jwt/jwt/v4 v4.4.1
	github.com/lestrrat-go/jwx v1.2.23
	github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
)

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/goccy/go-json v0.9.6 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.0 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.1 // indirect
	github.com/lestrrat-go/option v1.0.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292 // indirect
)

replace (
	github.com/eru-tech/eru/eru-logs => ../eru-logs
)