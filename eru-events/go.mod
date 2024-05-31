module github.com/eru-tech/eru/eru-events

go 1.22.0

require (
	github.com/aws/aws-sdk-go-v2 v1.27.0
	github.com/aws/aws-sdk-go-v2/config v1.27.11
	github.com/aws/aws-sdk-go-v2/credentials v1.17.11
	github.com/aws/aws-sdk-go-v2/service/sqs v1.32.3
	github.com/eru-tech/eru/eru-logs v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-utils v0.0.0-00010101000000-000000000000
	github.com/eru-tech/eru/eru-models v0.0.0-00010101000000-000000000000
)

replace (
	github.com/eru-tech/eru/eru-logs => ../eru-logs
	github.com/eru-tech/eru/eru-utils => ../eru-utils
	github.com/eru-tech/eru/eru-models => ../eru-models
)
