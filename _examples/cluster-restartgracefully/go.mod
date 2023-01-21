module cluster-restartgracefully

go 1.16

replace github.com/asynkron/protoactor-go => ../..

require (
	github.com/asynkron/goconsole v0.0.0-20160504192649-bfa12eebf716
	github.com/asynkron/protoactor-go v0.0.0-00010101000000-000000000000
	github.com/go-redis/redis v6.15.9+incompatible
	google.golang.org/protobuf v1.28.1
)
