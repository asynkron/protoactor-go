module jaegertracing

go 1.16

replace github.com/asynkron/protoactor-go => ../..

require (
	github.com/HdrHistogram/hdrhistogram-go v1.1.0 // indirect
	github.com/asynkron/goconsole v0.0.0-20160504192649-bfa12eebf716
	github.com/asynkron/protoactor-go v0.0.0-00010101000000-000000000000
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible
)
