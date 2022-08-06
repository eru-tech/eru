module github.com/eru-tech/eru/eru-security-rule

go 1.17

require github.com/eru-tech/eru/eru-templates v0.0.0-00010101000000-000000000000

require (
	github.com/eru-tech/eru/eru-crypto v0.0.0-00010101000000-000000000000 // indirect
	github.com/google/uuid v1.3.0 // indirect
)

replace (
	github.com/eru-tech/eru/eru-crypto => ../eru-crypto
	github.com/eru-tech/eru/eru-templates => ../eru-templates
)
