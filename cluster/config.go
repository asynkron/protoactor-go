package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"

	"github.com/AsynkronIT/protoactor-go/remote"
)

type Config struct {
	Name                                         string
	Address                                      string
	ClusterProvider                              ClusterProvider
	Identitylookup                               IdentityLookup
	RemoteConfig                                 remote.Config
	RequestTimeoutTime                           time.Duration
	RequestsLogThrottlePeriod                    time.Duration
	MaxNumberOfEventsInRequestLogThrottledPeriod int
	ClusterContextProducer                       ClusterContextProducer
	MemberStrategyBuilder                        func(kind string) MemberStrategy
	Kinds                                        map[string]*actor.Props
}

func Configure(clusterName string, clusterProvider ClusterProvider, identityLookup IdentityLookup, remoteConfig remote.Config, kinds ...*Kind) *Config {
	config := &Config{
		Name:                      clusterName,
		ClusterProvider:           clusterProvider,
		Identitylookup:            identityLookup,
		RequestTimeoutTime:        defaultActorRequestTimeout,
		RequestsLogThrottlePeriod: defaultRequestsLogThrottlePeriod,
		MemberStrategyBuilder:     newDefaultMemberStrategy,
		RemoteConfig:              remoteConfig,
		Kinds:                     make(map[string]*actor.Props),
		ClusterContextProducer:    newDefaultClusterContext,
		MaxNumberOfEventsInRequestLogThrottledPeriod: defaultMaxNumberOfEvetsInRequestLogThrottledPeriod,
	}

	for _, kind := range kinds {
		config.Kinds[kind.Kind] = kind.Props
	}

	return config
}

func (c *Config) WithRequestTimeout(t time.Duration) *Config {
	c.RequestTimeoutTime = t
	return c
}

// Sets the given request log throttle period duration
// and returns itself back
func (c *Config) WithRequestsLogThrottlePeriod(period time.Duration) *Config {

	c.RequestsLogThrottlePeriod = period
	return c
}

// Sets the given context producer and returns itself back
func (c *Config) WithClusterContextProducer(producer ClusterContextProducer) *Config {

	c.ClusterContextProducer = producer
	return c
}

// Sets the given max number of events in requests log throttle period
// and returns itself back
func (c *Config) WithMaxNumberOfEventsInRequestLogThrottlePeriod(maxNumber int) *Config {

	c.MaxNumberOfEventsInRequestLogThrottledPeriod = maxNumber
	return c
}

// Converts this Cluster config ClusterContext parameters
// into a valid ClusterContextConfig value and returns a pointer to its memory
func (c *Config) ToClusterContextConfig() *ClusterContextConfig {

	clusterContextConfig := ClusterContextConfig{
		ActorRequestTimeout:                          c.RequestTimeoutTime,
		RequestsLogThrottlePeriod:                    c.RequestsLogThrottlePeriod,
		MaxNumberOfEventsInRequestLogThrottledPeriod: c.MaxNumberOfEventsInRequestLogThrottledPeriod,
	}
	return &clusterContextConfig
}

type Kind struct {
	Kind  string
	Props *actor.Props
}

func NewKind(kind string, props *actor.Props) *Kind {
	return &Kind{
		Kind:  kind,
		Props: props,
	}
}
