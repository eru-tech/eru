module github.com/eru-tech/eru/eru-alerts

go 1.17

require (
	github.com/eru-tech/eru/eru-routes v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-server v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-store v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-utils v0.0.0-00010101000000-000000000000
	github.com/gorilla/mux v1.8.0
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
)

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/eru-tech/eru/eru-crypto v0.0.0-00010101000000-000000000000 // indirect
	github.com/eru-tech/eru/eru-templates v0.0.0-00010101000000-000000000000 // indirect
	github.com/goccy/go-json v0.9.6 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.1 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/jmoiron/sqlx v1.3.4 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.0 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.1 // indirect
	github.com/lestrrat-go/jwx v1.2.23 // indirect
	github.com/lestrrat-go/option v1.0.0 // indirect
	github.com/lib/pq v1.2.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.3 // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/segmentio/ksuid v1.0.3 // indirect
	github.com/xuri/efp v0.0.0-20220603152613-6918739fd470 // indirect
	github.com/xuri/excelize/v2 v2.7.0 // indirect
	github.com/xuri/nfp v0.0.0-20220409054826-5e722a1d9e22 // indirect
	golang.org/x/crypto v0.5.0 // indirect
	golang.org/x/net v0.5.0 // indirect
	golang.org/x/text v0.6.0 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
)

replace (
	github.com/eru-tech/eru/eru-crypto => ../eru-crypto
	github.com/eru-tech/eru/eru-routes => ../eru-routes
	github.com/eru-tech/eru/eru-server => ../eru-server
	github.com/eru-tech/eru/eru-store => ../eru-store
	github.com/eru-tech/eru/eru-templates => ../eru-templates
	github.com/eru-tech/eru/eru-utils => ../eru-utils
)
