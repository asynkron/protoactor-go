package remote

import (
	"fmt"

	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func defaultConfig() *Config {
	return &Config{
		AdvertisedHost:           "",
		DialOptions:              []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
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

// validate checks the remote configuration for invalid values and returns an
// error if any field is out of range.
func (c *Config) validate() error {
	if c.Host == "" {
		return fmt.Errorf("host must not be empty")
	}
	if c.Port < 0 {
		return fmt.Errorf("port must be >= 0, got %d", c.Port)
	}
	if c.EndpointWriterBatchSize <= 0 {
		return fmt.Errorf("EndpointWriterBatchSize must be > 0, got %d", c.EndpointWriterBatchSize)
	}
	if c.EndpointWriterQueueSize <= 0 {
		return fmt.Errorf("EndpointWriterQueueSize must be > 0, got %d", c.EndpointWriterQueueSize)
	}
	if c.EndpointManagerBatchSize <= 0 {
		return fmt.Errorf("EndpointManagerBatchSize must be > 0, got %d", c.EndpointManagerBatchSize)
	}
	if c.EndpointManagerQueueSize <= 0 {
		return fmt.Errorf("EndpointManagerQueueSize must be > 0, got %d", c.EndpointManagerQueueSize)
	}
	return nil
}

// ConfigureWithError configures the remote and validates the resulting
// configuration. It returns an error if the configuration is invalid.
func ConfigureWithError(host string, port int, options ...ConfigOption) (*Config, error) {
	c := newConfig(options...)
	c.Host = host
	c.Port = port

	if err := c.validate(); err != nil {
		return nil, err
	}

	return c, nil
}

// Configure configures the remote. It panics if the configuration is invalid.
// Use ConfigureWithError for a non-panicking variant.
func Configure(host string, port int, options ...ConfigOption) *Config {
	c, err := ConfigureWithError(host, port, options...)
	if err != nil {
		panic(err)
	}

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
