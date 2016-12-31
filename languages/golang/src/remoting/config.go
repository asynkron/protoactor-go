package remoting

import "google.golang.org/grpc"

type RemotingOption func(*remotingConfig)

func defaultRemoteConfig() *remotingConfig {
	return &remotingConfig{
		dialOptions:              []grpc.DialOption{grpc.WithInsecure()},
		endpointWriterBatchSize:  1000,
		endpointManagerBatchSize: 1000,
		endpointWriterQueueSize:  1000000,
		endpointManagerQueueSize: 1000000,
	}
}

func WithEndpointWriterBatchSize(batchSize int) RemotingOption {
	return func(config *remotingConfig) {
		config.endpointWriterBatchSize = batchSize
	}
}

func WithEndpointWriterQueueSize(queueSize int) RemotingOption {
	return func(config *remotingConfig) {
		config.endpointWriterQueueSize = queueSize
	}
}

func WithEndpointManagerBatchSize(batchSize int) RemotingOption {
	return func(config *remotingConfig) {
		config.endpointManagerBatchSize = batchSize
	}
}

func WithEndpointManagerQueueSize(queueSize int) RemotingOption {
	return func(config *remotingConfig) {
		config.endpointManagerQueueSize = queueSize
	}
}

func WithDialOptions(options ...grpc.DialOption) RemotingOption {
	return func(config *remotingConfig) {
		config.dialOptions = options
	}
}

func WithServerOptions(options ...grpc.ServerOption) RemotingOption {
	return func(config *remotingConfig) {
		config.serverOptions = options
	}
}

func WithCallOptions(options ...grpc.CallOption) RemotingOption {
	return func(config *remotingConfig) {
		config.callOptions = options
	}
}

type remotingConfig struct {
	serverOptions            []grpc.ServerOption
	callOptions              []grpc.CallOption
	dialOptions              []grpc.DialOption
	endpointWriterBatchSize  int
	endpointWriterQueueSize  int
	endpointManagerBatchSize int
	endpointManagerQueueSize int
}
