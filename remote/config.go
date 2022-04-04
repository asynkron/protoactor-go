package remote

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/grpc"
)

func defaultConfig() *Config {
	return &Config{
		AdvertisedHost:           "",
		DialOptions:              []grpc.DialOption{grpc.WithInsecure()},
		EndpointWriterBatchSize:  1000,
		EndpointManagerBatchSize: 1000,
		EndpointWriterQueueSize:  1000000,
		EndpointManagerQueueSize: 1000000,
		Kinds:                    make(map[string]*actor.Props),
	}
}

type ConfigOption func(config *Config)

func newConfig(options ...ConfigOption) *Config {
	config := defaultConfig()
	for _, option := range options {
		option(config)
	}
	return config
}

// WithEndpointWriterBatchSize sets the batch size for the endpoint writer
func WithEndpointWriterBatchSize(batchSize int) ConfigOption {
	return func(config *Config) {
		config.EndpointWriterBatchSize = batchSize
	}
}

// WithEndpointWriterQueueSize sets the queue size for the endpoint writer
func WithEndpointWriterQueueSize(queueSize int) ConfigOption {
	return func(config *Config) {
		config.EndpointWriterQueueSize = queueSize
	}
}

// WithEndpointManagerBatchSize sets the batch size for the endpoint manager
func WithEndpointManagerBatchSize(batchSize int) ConfigOption {
	return func(config *Config) {
		config.EndpointManagerBatchSize = batchSize
	}
}

// WithEndpointManagerQueueSize sets the queue size for the endpoint manager
func WithEndpointManagerQueueSize(queueSize int) ConfigOption {
	return func(config *Config) {
		config.EndpointManagerQueueSize = queueSize
	}
}

// WithDialOptions sets the dial options for the remote
func WithDialOptions(options ...grpc.DialOption) ConfigOption {
	return func(config *Config) {
		config.DialOptions = options
	}
}

// WithServerOptions sets the server options for the remote
func WithServerOptions(options ...grpc.ServerOption) ConfigOption {
	return func(config *Config) {
		config.ServerOptions = options
	}
}

// WithCallOptions sets the call options for the remote
func WithCallOptions(options ...grpc.CallOption) ConfigOption {
	return func(config *Config) {
		config.CallOptions = options
	}
}

// WithAdvertisedHost sets the advertised host for the remote
func WithAdvertisedHost(address string) ConfigOption {
	return func(config *Config) {
		config.AdvertisedHost = address
	}
}

// WithKinds adds the kinds to the remote
func WithKinds(kinds ...*Kind) ConfigOption {
	return func(config *Config) {
		for _, k := range kinds {
			config.Kinds[k.Kind] = k.Props
		}
	}
}

// Address returns the address of the remote
func (rc Config) Address() string {
	return fmt.Sprintf("%v:%v", rc.Host, rc.Port)
}

// Configure configures the remote
func Configure(host string, port int, options ...ConfigOption) *Config {
	c := newConfig(options...)
	c.Host = host
	c.Port = port

	return c
}

// Config is the configuration for the remote
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

// Kind is the configuration for a kind
type Kind struct {
	Kind  string
	Props *actor.Props
}

// NewKind creates a new kind configuration
func NewKind(kind string, props *actor.Props) *Kind {
	return &Kind{
		Kind:  kind,
		Props: props,
	}
}
