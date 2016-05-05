package remoting

import "google.golang.org/grpc"

type RemotingOption func(*RemotingConfig)

func defaultRemoteConfig() *RemotingConfig {
	return &RemotingConfig{
		dialOptions: []grpc.DialOption{grpc.WithInsecure()},
		batchSize:   200,
	}
}

func WithBatchSize(batchSize int) RemotingOption {
	return func(config *RemotingConfig) {
		config.batchSize = batchSize
	}
}

func WithDialOptions(options ...grpc.DialOption) RemotingOption {
	return func(config *RemotingConfig) {
		config.dialOptions = options
	}
}

func WithServerOptions(options ...grpc.ServerOption) RemotingOption {
	return func(config *RemotingConfig) {
		config.serverOptions = options
	}
}

func WithCallOptions(options ...grpc.CallOption) RemotingOption {
	return func(config *RemotingConfig) {
		config.callOptions = options
	}
}

type RemotingConfig struct {
	serverOptions []grpc.ServerOption
	callOptions   []grpc.CallOption
	dialOptions   []grpc.DialOption
	batchSize     int
}
