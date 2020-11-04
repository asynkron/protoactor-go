package remote

import (
	"fmt"
	"google.golang.org/grpc"
)

func defaultRemoteConfig() Config {
	return Config{
		advertisedHost:           "",
		dialOptions:              []grpc.DialOption{grpc.WithInsecure()},
		endpointWriterBatchSize:  1000,
		endpointManagerBatchSize: 1000,
		endpointWriterQueueSize:  1000000,
		endpointManagerQueueSize: 1000000,
	}
}

func (rc Config) WithEndpointWriterBatchSize(batchSize int) Config {
	rc.endpointWriterBatchSize = batchSize
	return rc
}

func (rc Config) WithEndpointWriterQueueSize(queueSize int) Config {
	rc.endpointWriterQueueSize = queueSize
	return rc
}

func (rc Config) WithEndpointManagerBatchSize(batchSize int) Config {
	rc.endpointManagerBatchSize = batchSize
	return rc
}

func (rc Config) WithEndpointManagerQueueSize(queueSize int) Config {
	rc.endpointManagerQueueSize = queueSize
	return rc
}

func (rc Config) WithDialOptions(options ...grpc.DialOption) Config {
	rc.dialOptions = options
	return rc
}

func (rc Config) WithServerOptions(options ...grpc.ServerOption) Config {
	rc.serverOptions = options
	return rc
}

func (rc Config) WithCallOptions(options ...grpc.CallOption) Config {
	rc.callOptions = options
	return rc
}

func (rc Config) WithAdvertisedHost(address string) Config {
	rc.advertisedHost = address
	return rc
}

func (rc Config) Address() string {
	return fmt.Sprintf("%v:%v", rc.host, rc.port)
}

func Configure(host string, port int) Config {
	c := defaultRemoteConfig()
	c.host = host
	c.port = port
	return c
}

type Config struct {
	host                     string
	port                     int
	advertisedHost           string
	serverOptions            []grpc.ServerOption
	callOptions              []grpc.CallOption
	dialOptions              []grpc.DialOption
	endpointWriterBatchSize  int
	endpointWriterQueueSize  int
	endpointManagerBatchSize int
	endpointManagerQueueSize int
}
