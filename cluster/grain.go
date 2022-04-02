package cluster

import "time"

type Grain struct {
	ci      *ClusterIdentity
	cluster *Cluster
}

func (g *Grain) Identity() string {
	return g.ci.Identity
}

func (g *Grain) Kind() string {
	return g.ci.Kind
}

func (g *Grain) Cluster() *Cluster {
	return g.cluster
}

func (g *Grain) Init(ci *ClusterIdentity, cluster *Cluster) {
	g.ci = ci
	g.cluster = cluster
}

type GrainCallOptions struct {
	RetryCount  int
	Timeout     time.Duration
	RetryAction func(n int)
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

type ClusterInit struct {
	Identity *ClusterIdentity
	Cluster  *Cluster
}
