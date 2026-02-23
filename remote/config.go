package remote

import (
	"fmt"
	"time"

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
		RetryBaseDelay:           2 * time.Second,
		RetryMaxDelay:            10 * time.Second,
		ShutdownTimeout:          10 * time.Second,
		SupervisorRestartWindow: 60 * time.Second,
		SupervisorMaxRestarts:   5,
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
	if c.RetryBaseDelay <= 0 {
		return fmt.Errorf("RetryBaseDelay must be > 0, got %v", c.RetryBaseDelay)
	}
	if c.RetryMaxDelay <= 0 {
		return fmt.Errorf("RetryMaxDelay must be > 0, got %v", c.RetryMaxDelay)
	}
	if c.RetryMaxDelay < c.RetryBaseDelay {
		return fmt.Errorf("RetryMaxDelay must be >= RetryBaseDelay, got %v < %v", c.RetryMaxDelay, c.RetryBaseDelay)
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("ShutdownTimeout must be > 0, got %v", c.ShutdownTimeout)
	}
	if c.SupervisorRestartWindow <= 0 {
		return fmt.Errorf("SupervisorRestartWindow must be > 0, got %v", c.SupervisorRestartWindow)
	}
	if c.SupervisorMaxRestarts <= 0 {
		return fmt.Errorf("SupervisorMaxRestarts must be > 0, got %d", c.SupervisorMaxRestarts)
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
	// RetryBaseDelay is the base delay between connection retry attempts.
	RetryBaseDelay time.Duration
	// RetryMaxDelay is the maximum delay between connection retry attempts.
	// Delays increase exponentially from RetryBaseDelay up to this cap.
	RetryMaxDelay time.Duration
	// ShutdownTimeout is the maximum time to wait for a graceful shutdown
	// before forcing a hard stop.
	ShutdownTimeout time.Duration
	// SupervisorRestartWindow is the time window for counting child failures.
	SupervisorRestartWindow time.Duration
	// SupervisorMaxRestarts is the maximum number of child restarts allowed
	// within SupervisorRestartWindow before the supervisor stops the child.
	SupervisorMaxRestarts int
}
