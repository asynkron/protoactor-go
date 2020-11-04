package remote

import (
	"fmt"
	"github.com/AsynkronIT/protoactor-go/actor"
	"google.golang.org/grpc"
)

func defaultRemoteConfig() Config {
	return Config{
		AdvertisedHost:           "",
		DialOptions:              []grpc.DialOption{grpc.WithInsecure()},
		EndpointWriterBatchSize:  1000,
		EndpointManagerBatchSize: 1000,
		EndpointWriterQueueSize:  1000000,
		EndpointManagerQueueSize: 1000000,
		Kinds:                    make(map[string]*actor.Props),
	}
}

func (rc Config) WithEndpointWriterBatchSize(batchSize int) Config {
	rc.EndpointWriterBatchSize = batchSize
	return rc
}

func (rc Config) WithEndpointWriterQueueSize(queueSize int) Config {
	rc.EndpointWriterQueueSize = queueSize
	return rc
}

func (rc Config) WithEndpointManagerBatchSize(batchSize int) Config {
	rc.EndpointManagerBatchSize = batchSize
	return rc
}

func (rc Config) WithEndpointManagerQueueSize(queueSize int) Config {
	rc.EndpointManagerQueueSize = queueSize
	return rc
}

func (rc Config) WithDialOptions(options ...grpc.DialOption) Config {
	rc.DialOptions = options
	return rc
}

func (rc Config) WithServerOptions(options ...grpc.ServerOption) Config {
	rc.ServerOptions = options
	return rc
}

func (rc Config) WithCallOptions(options ...grpc.CallOption) Config {
	rc.CallOptions = options
	return rc
}

func (rc Config) WithAdvertisedHost(address string) Config {
	rc.AdvertisedHost = address
	return rc
}

func (rc Config) Address() string {
	return fmt.Sprintf("%v:%v", rc.Host, rc.Port)
}

func Configure(host string, port int, kinds ...*Kind) Config {
	c := defaultRemoteConfig()
	c.Host = host
	c.Port = port

	for _, kind := range kinds {
		c.Kinds[kind.Kind] = kind.Props
	}
	return c
}

type Config struct {
	Host                     string
	Port                     int
	AdvertisedHost           string
	ServerOptions            []grpc.ServerOption
	CallOptions              []grpc.CallOption
	DialOptions              []grpc.DialOption
	EndpointWriterBatchSize  int
	EndpointWriterQueueSize  int
	EndpointManagerBatchSize int
	EndpointManagerQueueSize int
	Kinds                    map[string]*actor.Props
}

type Kind struct {
	Kind  string
	Props *actor.Props
}

func NewKind(kind string, props *actor.Props) *Kind {
	return &Kind{
		Kind:  kind,
		Props: props,
	}
}
