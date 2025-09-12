module github.com/eru-tech/eru/eru-store

go 1.24

require (
	github.com/eru-tech/eru/eru-cache v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-db v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-events v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-models v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-read-write v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-repos v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-scheduler v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-secret-manager v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-utils v0.0.0-00010101000000-000000000000
	github.com/google/go-cmp v0.7.0
	github.com/jmoiron/sqlx v1.3.5
	github.com/lib/pq v1.2.0
	github.com/tidwall/gjson v1.17.1
)

require (
	github.com/aws/aws-sdk-go-v2 v1.37.2 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.11 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.11 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/kms v1.31.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.28.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sns v1.36.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.32.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.20.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.23.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.28.6 // indirect
	github.com/aws/smithy-go v1.22.5 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/redis/go-redis/v9 v9.11.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	go.etcd.io/etcd/api/v3 v3.6.4 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.6.4 // indirect
	go.etcd.io/etcd/client/v3 v3.6.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.55.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250721164621-a45f3dfb1074 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250728155136-f173205681a0 // indirect
	google.golang.org/grpc v1.74.2 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
)

replace (
	github.com/eru-tech/eru/eru-cache => ../eru-cache
	github.com/eru-tech/eru/eru-db => ../eru-db
	github.com/eru-tech/eru/eru-events => ../eru-events
	github.com/eru-tech/eru/eru-events/events => ../eru-events/events
	github.com/eru-tech/eru/eru-logs => ../eru-logs
	github.com/eru-tech/eru/eru-models => ../eru-models
	github.com/eru-tech/eru/eru-read-write => ../eru-read-write
	github.com/eru-tech/eru/eru-repos => ../eru-repos
	github.com/eru-tech/eru/eru-scheduler => ../eru-scheduler
	github.com/eru-tech/eru/eru-secret-manager => ../eru-secret-manager
	github.com/eru-tech/eru/eru-utils => ../eru-utils
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20250721164621-a45f3dfb1074
)
