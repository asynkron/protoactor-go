package remote

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/grpc"
)

func defaultConfig() Config {
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

type ConfigOption func(config Config) Config

func newConfig(options ...ConfigOption) Config {
	config := defaultConfig()
	for _, option := range options {
		config = option(config)
	}
	return config
}

// WithEndpointWriterBatchSize sets the batch size for the endpoint writer
func WithEndpointWriterBatchSize(batchSize int) ConfigOption {
	return func(config Config) Config {
		config.EndpointWriterBatchSize = batchSize
		return config
	}
}

// WithEndpointWriterQueueSize sets the queue size for the endpoint writer
func WithEndpointWriterQueueSize(queueSize int) ConfigOption {
	return func(config Config) Config {
		config.EndpointWriterQueueSize = queueSize
		return config
	}
}

// WithEndpointManagerBatchSize sets the batch size for the endpoint manager
func WithEndpointManagerBatchSize(batchSize int) ConfigOption {
	return func(config Config) Config {
		config.EndpointManagerBatchSize = batchSize
		return config
	}
}

// WithEndpointManagerQueueSize sets the queue size for the endpoint manager
func WithEndpointManagerQueueSize(queueSize int) ConfigOption {
	return func(config Config) Config {
		config.EndpointManagerQueueSize = queueSize
		return config
	}
}

// WithDialOptions sets the dial options for the remote
func WithDialOptions(options ...grpc.DialOption) ConfigOption {
	return func(config Config) Config {
		config.DialOptions = options
		return config
	}
}

// WithServerOptions sets the server options for the remote
func WithServerOptions(options ...grpc.ServerOption) ConfigOption {
	return func(config Config) Config {
		config.ServerOptions = options
		return config
	}
}

// WithCallOptions sets the call options for the remote
func WithCallOptions(options ...grpc.CallOption) ConfigOption {
	return func(config Config) Config {
		config.CallOptions = options
		return config
	}
}

// WithAdvertisedHost sets the advertised host for the remote
func WithAdvertisedHost(address string) ConfigOption {
	return func(config Config) Config {
		config.AdvertisedHost = address
		return config
	}
}

// WithKinds adds the kinds to the remote
func WithKinds(kinds ...*Kind) ConfigOption {
	return func(config Config) Config {
		for _, k := range kinds {
			config.Kinds[k.Kind] = k.Props
		}
		return config
	}
}

// WithKind adds a kind to the remote
func WithKind(kind string, props *actor.Props) ConfigOption {
	return func(config Config) Config {
		config.Kinds[kind] = props
		return config
	}
}

// Address returns the address of the remote
func (rc Config) Address() string {
	return fmt.Sprintf("%v:%v", rc.Host, rc.Port)
}

// Configure configures the remote
func Configure(host string, port int, options ...ConfigOption) Config {
	c := newConfig(options...)
	c.Host = host
	c.Port = port

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
