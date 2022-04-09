package remote

import "google.golang.org/grpc"

type ConfigOption func(config *Config)

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
