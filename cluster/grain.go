package cluster

import (
	"time"

	"github.com/awevoke/protoactor-go/actor"
)

type GrainCallConfig struct {
	RetryCount  int
	Timeout     time.Duration
	RetryAction func(n int) int
	Context     actor.SenderContext
	Headers     map[string]string
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
		// TODO: set default in config
		RetryCount: 3,
		Context:    cluster.ActorSystem.Root,
		Timeout:    cluster.Config.RequestTimeoutTime,
		RetryAction: func(i int) int {
			i++
			time.Sleep(time.Duration(i * i * 50))
			return i
		},
	}
}

func WithTimeout(timeout time.Duration) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.Timeout = timeout
	}
}

func WithRetryCount(count int) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.RetryCount = count
	}
}

func WithRetryAction(act func(i int) int) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.RetryAction = act
	}
}

func WithContext(ctx actor.SenderContext) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.Context = ctx
	}
}

// WithHeaders attaches the given headers to the outgoing request message.
// If the caller also passes a *actor.MessageEnvelope as the message argument,
// header values set directly on the envelope win on key conflict.
func WithHeaders(headers map[string]string) GrainCallOption {
	return func(config *GrainCallConfig) {
		config.Headers = headers
	}
}

type ClusterInit struct {
	Identity *ClusterIdentity
	Cluster  *Cluster
}
