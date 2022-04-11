package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
	"time"
)

type GrainCallConfig struct {
	RetryCount  int
	Timeout     time.Duration
	RetryAction func(n int)
	Context     actor.SenderContext
}

type GrainCallOption func(config *GrainCallConfig)

var defaultGrainCallOptions *GrainCallConfig

func DefaultGrainCallConfig(cluster *Cluster) *GrainCallConfig {
	if defaultGrainCallOptions == nil {
		defaultGrainCallOptions = NewGrainCallOptions(cluster)
	}
	return defaultGrainCallOptions
}

func NewGrainCallOptions(cluster *Cluster) *GrainCallConfig {
	return &GrainCallConfig{
		RetryCount: 10,
		Timeout:    cluster.Config.RequestTimeoutTime,
		RetryAction: func(i int) {
			i++
			time.Sleep(time.Duration(i * i * 50))
		},
	}
}

func WithTimeout(timeout time.Duration) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.Timeout = timeout
	}
}

func WithRetry(count int) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.RetryCount = count
	}
}

func WithRetryAction(act func(i int)) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.RetryAction = act
	}
}

func WithContext(ctx actor.SenderContext) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.Context = ctx
	}
}

type ClusterInit struct {
	Identity *ClusterIdentity
	Cluster  *Cluster
}
