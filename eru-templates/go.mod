module github.com/eru-tech/eru/eru-templates

go 1.20

require (
	github.com/Masterminds/sprig/v3 v3.2.3
	github.com/eru-tech/eru/eru-crypto v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.3.0
	github.com/xuri/excelize/v2 v2.7.0
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.0 // indirect
	github.com/huandu/xstrings v1.3.3 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.3 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/stretchr/testify v1.8.2 // indirect
	github.com/xuri/efp v0.0.0-20220603152613-6918739fd470 // indirect
	github.com/xuri/nfp v0.0.0-20220409054826-5e722a1d9e22 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/crypto v0.5.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/text v0.7.0 // indirect
)

replace (
	github.com/eru-tech/eru/eru-crypto => ../eru-crypto
	github.com/eru-tech/eru/eru-logs => ../eru-logs
)
