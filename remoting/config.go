package remoting

import "google.golang.org/grpc"

type RemotingOption func(*remotingConfig)

func defaultRemoteConfig() *remotingConfig {
	return &remotingConfig{
		dialOptions: []grpc.DialOption{grpc.WithInsecure()},
		batchSize:   200,
	}
}

func WithBatchSize(batchSize int) RemotingOption {
	return func(config *remotingConfig) {
		config.batchSize = batchSize
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
	serverOptions []grpc.ServerOption
	callOptions   []grpc.CallOption
	dialOptions   []grpc.DialOption
	batchSize     int
}
