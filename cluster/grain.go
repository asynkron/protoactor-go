package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
	"time"
)

//type Grain struct {
//}
//
//func (g *Grain) Init(ctx GrainContext) {
//
//}

type GrainCallOptions struct {
	RetryCount  int
	Timeout     time.Duration
	RetryAction func(n int)
	Context     actor.SenderContext
}

var defaultGrainCallOptions *GrainCallOptions

func DefaultGrainCallOptions(cluster *Cluster) *GrainCallOptions {
	if defaultGrainCallOptions == nil {
		defaultGrainCallOptions = NewGrainCallOptions(cluster)
	}
	return defaultGrainCallOptions
}

func NewGrainCallOptions(cluster *Cluster) *GrainCallOptions {
	return &GrainCallOptions{
		RetryCount: 10,
		Timeout:    cluster.Config.RequestTimeoutTime,
		RetryAction: func(i int) {
			i++
			time.Sleep(time.Duration(i * i * 50))
		},
	}
}

func (config *GrainCallOptions) WithTimeout(timeout time.Duration) *GrainCallOptions {
	config.Timeout = timeout
	return config
}

func (config *GrainCallOptions) WithRetry(count int) *GrainCallOptions {
	config.RetryCount = count
	return config
}

func (config *GrainCallOptions) WithRetryAction(act func(i int)) *GrainCallOptions {
	config.RetryAction = act
	return config
}

func (config *GrainCallOptions) WithContext(ctx actor.SenderContext) *GrainCallOptions {
	config.Context = ctx
	return config
}

type ClusterInit struct {
	Identity *ClusterIdentity
	Cluster  *Cluster
}
