module github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv

go 1.25.3

require (
	github.com/asynkron/protoactor-go v0.0.0
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats-server/v2 v2.11.4
	github.com/nats-io/nats.go v1.48.0
	github.com/stretchr/testify v1.11.1
	github.com/testcontainers/testcontainers-go v0.40.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/asynkron/protoactor-go => ../../../
