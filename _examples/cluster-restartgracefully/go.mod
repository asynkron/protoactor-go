module cluster-restartgracefully

go 1.13

replace github.com/AsynkronIT/protoactor-go => ../..

replace google.golang.org/grpc => google.golang.org/grpc v1.26.0

require (
	github.com/AsynkronIT/goconsole v0.0.0-20160504192649-bfa12eebf716
	github.com/AsynkronIT/protoactor-go v0.0.0-00010101000000-000000000000
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/gogo/protobuf v1.3.1
	github.com/onsi/ginkgo v1.15.2 // indirect
	github.com/onsi/gomega v1.11.0 // indirect
)
