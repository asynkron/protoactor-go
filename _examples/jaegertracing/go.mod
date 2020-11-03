module jaegertracing

go 1.13

replace github.com/AsynkronIT/protoactor-go => ../..

require (
	github.com/AsynkronIT/goconsole v0.0.0-20160504192649-bfa12eebf716
	github.com/AsynkronIT/protoactor-go v0.0.0-00010101000000-000000000000
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible
	golang.org/x/lint v0.0.0-20190930215403-16217165b5de // indirect
	golang.org/x/tools v0.0.0-20191029041327-9cc4af7d6b2c // indirect
)
