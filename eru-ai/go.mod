module github.com/eru-tech/eru/eru-ai

go 1.24

toolchain go1.24.5

require (
	github.com/aws/aws-sdk-go-v2 v1.38.0
	github.com/aws/aws-sdk-go-v2/config v1.31.0
	github.com/aws/aws-sdk-go-v2/service/bedrockruntime v1.7.0
	github.com/eru-tech/eru/eru-db v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-models v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-repos v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-scheduler v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-secret-manager v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-server v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-store v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-templates v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-utils v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-vectorstore v0.0.0-00010101000000-000000000000
	github.com/google/go-cmp v0.7.0
	github.com/gorilla/mux v1.8.0
	github.com/playwright-community/playwright-go v0.5001.0
	github.com/xeipuuv/gojsonschema v1.2.0
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/semver/v3 v3.2.0 // indirect
	github.com/Masterminds/sprig/v3 v3.2.3 // indirect
	github.com/amikos-tech/chroma-go v0.1.4 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.18.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/kms v1.31.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3vectors v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.28.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sns v1.36.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.32.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.28.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.33.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.37.0 // indirect
	github.com/aws/smithy-go v1.22.5 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/deckarep/golang-set/v2 v2.7.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/eru-tech/eru/eru-cache v0.0.0-00010101000000-000000000000 // indirect
	github.com/eru-tech/eru/eru-crypto v0.0.0-00010101000000-000000000000 // indirect
	github.com/eru-tech/eru/eru-events v0.0.0-00010101000000-000000000000 // indirect
	github.com/eru-tech/eru/eru-read-write v0.0.0-00010101000000-000000000000 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-jose/go-jose/v3 v3.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/goccy/go-json v0.9.6 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.1 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/huandu/xstrings v1.3.3 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jmoiron/sqlx v1.3.5 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.0 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.1 // indirect
	github.com/lestrrat-go/jwx v1.2.23 // indirect
	github.com/lestrrat-go/option v1.0.0 // indirect
	github.com/lib/pq v1.2.0 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.0 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/redis/go-redis/v9 v9.11.0 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.4 // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/tidwall/gjson v1.17.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tiendc/go-deepcopy v1.6.0 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/excelize/v2 v2.9.1 // indirect
	github.com/xuri/nfp v0.0.1 // indirect
	go.etcd.io/etcd/api/v3 v3.6.4 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.6.4 // indirect
	go.etcd.io/etcd/client/v3 v3.6.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.21.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.21.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.21.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto v0.0.0-20230331144136-dcfb400f0633 // indirect
	google.golang.org/grpc v1.74.2 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
)

replace (
	github.com/eru-tech/eru/eru-cache => ../eru-cache
	github.com/eru-tech/eru/eru-crypto => ../eru-crypto
	github.com/eru-tech/eru/eru-db => ../eru-db
	github.com/eru-tech/eru/eru-events => ../eru-events
	github.com/eru-tech/eru/eru-functions => ../eru-functions
	github.com/eru-tech/eru/eru-logs => ../eru-logs
	github.com/eru-tech/eru/eru-models => ../eru-models
	github.com/eru-tech/eru/eru-read-write => ../eru-read-write
	github.com/eru-tech/eru/eru-repos => ../eru-repos
	github.com/eru-tech/eru/eru-scheduler => ../eru-scheduler
	github.com/eru-tech/eru/eru-secret-manager => ../eru-secret-manager
	github.com/eru-tech/eru/eru-server => ../eru-server
	github.com/eru-tech/eru/eru-store => ../eru-store
	github.com/eru-tech/eru/eru-templates => ../eru-templates
	github.com/eru-tech/eru/eru-utils => ../eru-utils
	github.com/eru-tech/eru/eru-vectorstore => ../eru-vectorstore
)
