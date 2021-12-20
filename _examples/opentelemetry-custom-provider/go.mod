module opentelemetry-custom-provider

go 1.16

replace github.com/AsynkronIT/protoactor-go => ../..

require (
	github.com/AsynkronIT/goconsole v0.0.0-20160504192649-bfa12eebf716
	github.com/AsynkronIT/protoactor-go v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v0.25.0
	go.opentelemetry.io/otel/metric v0.26.0
	go.opentelemetry.io/otel/sdk/metric v0.26.0
)
