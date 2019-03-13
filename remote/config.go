package remote

import "google.golang.org/grpc"

// RemotingOption configures how the remote infrastructure is started
type RemotingOption func(*remoteConfig)

func defaultRemoteConfig() *remoteConfig {
	return &remoteConfig{
		dialOptions:              []grpc.DialOption{grpc.WithInsecure()},
		endpointWriterBatchSize:  1000,
		endpointManagerBatchSize: 1000,
		endpointWriterQueueSize:  1000000,
		endpointManagerQueueSize: 1000000,
		connectionDecider:        func(string) bool { return true },
	}
}

func WithEndpointWriterBatchSize(batchSize int) RemotingOption {
	return func(config *remoteConfig) {
		config.endpointWriterBatchSize = batchSize
	}
}

func WithEndpointWriterQueueSize(queueSize int) RemotingOption {
	return func(config *remoteConfig) {
		config.endpointWriterQueueSize = queueSize
	}
}

func WithEndpointManagerBatchSize(batchSize int) RemotingOption {
	return func(config *remoteConfig) {
		config.endpointManagerBatchSize = batchSize
	}
}

func WithEndpointManagerQueueSize(queueSize int) RemotingOption {
	return func(config *remoteConfig) {
		config.endpointManagerQueueSize = queueSize
	}
}

func WithDialOptions(options ...grpc.DialOption) RemotingOption {
	return func(config *remoteConfig) {
		config.dialOptions = options
	}
}

func WithServerOptions(options ...grpc.ServerOption) RemotingOption {
	return func(config *remoteConfig) {
		config.serverOptions = options
	}
}

func WithCallOptions(options ...grpc.CallOption) RemotingOption {
	return func(config *remoteConfig) {
		config.callOptions = options
	}
}

func WithConnectionDecider(decider func(string) bool) RemotingOption {
	return func(config *remoteConfig) {
		config.connectionDecider = decider
	}
}

type remoteConfig struct {
	serverOptions            []grpc.ServerOption
	callOptions              []grpc.CallOption
	dialOptions              []grpc.DialOption
	endpointWriterBatchSize  int
	endpointWriterQueueSize  int
	endpointManagerBatchSize int
	endpointManagerQueueSize int
	connectionDecider        func(string) bool
}
