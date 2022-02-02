module github.com/AsynkronIT/protoactor-go

// because etcd/v3 need go 1.16 version
go 1.16

require (
	github.com/Workiva/go-datastructures v1.0.53
	github.com/armon/go-metrics v0.3.0 // indirect
	github.com/cespare/xxhash v1.1.0
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/couchbase/gocb v1.6.7
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f
	github.com/emirpasic/gods v1.12.0
	github.com/go-zookeeper/zk v1.0.2
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.2 // indirect
	github.com/google/uuid v1.3.0
	github.com/hashicorp/consul/api v1.12.0
	github.com/hashicorp/go-immutable-radix v1.1.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/labstack/echo v3.3.10+incompatible
	github.com/labstack/gommon v0.3.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/opentracing/opentracing-go v1.2.0
	github.com/orcaman/concurrent-map v0.0.0-20190107190726-7ed82d9cb717
	github.com/serialx/hashring v0.0.0-20180504054112-49a4782e9908
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.etcd.io/etcd/client/v3 v3.5.2
	go.opentelemetry.io/otel v1.3.0
	go.opentelemetry.io/otel/exporters/prometheus v0.26.0
	go.opentelemetry.io/otel/metric v0.26.0
	go.opentelemetry.io/otel/sdk/export/metric v0.26.0
	go.opentelemetry.io/otel/sdk/metric v0.26.0
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.44.0
	gopkg.in/couchbase/gocbcore.v7 v7.1.18 // indirect
	gopkg.in/couchbaselabs/gocbconnstr.v1 v1.0.4 // indirect
	gopkg.in/couchbaselabs/gojcbmock.v1 v1.0.4 // indirect
	gopkg.in/couchbaselabs/jsonx.v1 v1.0.0 // indirect
	k8s.io/api v0.23.3
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v0.23.3
)
