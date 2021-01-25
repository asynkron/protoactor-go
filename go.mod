module github.com/AsynkronIT/protoactor-go

replace (
	// etcd/clientv3
	// @see https://github.com/etcd-io/etcd/issues/12124
	go.etcd.io/bbolt => github.com/coreos/bbolt v1.3.5
	google.golang.org/grpc => google.golang.org/grpc v1.26.0
)

require (
	github.com/AsynkronIT/goconsole v0.0.0-20160504192649-bfa12eebf716 // indirect
	github.com/AsynkronIT/gonet v0.0.0-20161127091928-0553637be225
	github.com/Workiva/go-datastructures v1.0.52
	github.com/armon/go-metrics v0.3.0 // indirect
	github.com/cespare/xxhash v1.1.0
	github.com/chzyer/logex v1.1.10 // indirect
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/chzyer/test v0.0.0-20180213035817-a1ea475d72b1 // indirect
	github.com/coreos/etcd v3.3.25+incompatible
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/couchbase/gocb v1.6.7
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f
	github.com/emirpasic/gods v1.12.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.3
	github.com/golang/snappy v0.0.2 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/hashicorp/consul/api v1.8.1
	github.com/hashicorp/go-immutable-radix v1.1.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/go.net v0.0.1 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/labstack/echo v3.3.10+incompatible
	github.com/labstack/gommon v0.3.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0
	github.com/orcaman/concurrent-map v0.0.0-20190107190726-7ed82d9cb717
	github.com/serialx/hashring v0.0.0-20180504054112-49a4782e9908
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/uber/jaeger-client-go v2.25.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/crypto v0.0.0-20191117063200-497ca9f6d64f // indirect
	golang.org/x/net v0.0.0-20191116160921-f9c825593386
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	google.golang.org/grpc v1.35.0
	gopkg.in/couchbase/gocbcore.v7 v7.1.18 // indirect
	gopkg.in/couchbaselabs/gocbconnstr.v1 v1.0.4 // indirect
	gopkg.in/couchbaselabs/jsonx.v1 v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
)

go 1.13
