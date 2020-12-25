module cluster-restartgracefully

go 1.13

replace github.com/AsynkronIT/protoactor-go => ../..

require (
	github.com/AsynkronIT/goconsole v0.0.0-20160504192649-bfa12eebf716
	github.com/AsynkronIT/protoactor-go v0.0.0-00010101000000-000000000000
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/coreos/etcd v3.3.25+incompatible // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/gogo/protobuf v1.3.1
	go.uber.org/zap v1.16.0 // indirect
	google.golang.org/grpc v1.25.1
)
