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
		MaxRetryCount:            5,
	}
}

func newConfig(options ...ConfigOption) *Config {
	config := defaultConfig()
	for _, option := range options {
		option(config)
	}
	return config
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
	MaxRetryCount            int
}
