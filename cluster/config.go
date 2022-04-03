package cluster

import (
	"sync/atomic"
	"time"

	"github.com/asynkron/protoactor-go/actor"

	"github.com/asynkron/protoactor-go/remote"
)

type Config struct {
	Name                                         string
	Address                                      string
	ClusterProvider                              ClusterProvider
	IdentityLookup                               IdentityLookup
	RemoteConfig                                 *remote.Config
	RequestTimeoutTime                           time.Duration
	RequestsLogThrottlePeriod                    time.Duration
	MaxNumberOfEventsInRequestLogThrottledPeriod int
	ClusterContextProducer                       ClusterContextProducer
	MemberStrategyBuilder                        func(cluster *Cluster, kind string) MemberStrategy
	Kinds                                        map[string]*Kind

	TimeoutTime          time.Duration
	GossipInterval       time.Duration
	GossipRequestTimeout time.Duration
	GossipFanOut         int
	GossipMaxSend        int
}

type ConfigOption func(config *Config)

func Configure(clusterName string, clusterProvider ClusterProvider, identityLookup IdentityLookup, remoteConfig *remote.Config, options ...ConfigOption) *Config {
	config := &Config{
		Name:                      clusterName,
		ClusterProvider:           clusterProvider,
		IdentityLookup:            identityLookup,
		RequestTimeoutTime:        defaultActorRequestTimeout,
		RequestsLogThrottlePeriod: defaultRequestsLogThrottlePeriod,
		MemberStrategyBuilder:     newDefaultMemberStrategy,
		RemoteConfig:              remoteConfig,
		Kinds:                     make(map[string]*Kind),
		ClusterContextProducer:    newDefaultClusterContext,
		MaxNumberOfEventsInRequestLogThrottledPeriod: defaultMaxNumberOfEvetsInRequestLogThrottledPeriod,
		TimeoutTime:          time.Second * 5,
		GossipInterval:       time.Millisecond * 300,
		GossipRequestTimeout: time.Millisecond * 500,
		GossipFanOut:         3,
		GossipMaxSend:        50,
	}

	for _, option := range options {
		option(config)
	}

	return config
}

// WithRequestTimeout sets the request timeout
func WithRequestTimeout(t time.Duration) ConfigOption {
	return func(c *Config) {
		c.RequestTimeoutTime = t
	}
}

// WithRequestsLogThrottlePeriod sets the requests log throttle period
func WithRequestsLogThrottlePeriod(period time.Duration) ConfigOption {
	return func(c *Config) {
		c.RequestsLogThrottlePeriod = period
	}
}

// WithClusterContextProducer sets the cluster context producer
func WithClusterContextProducer(producer ClusterContextProducer) ConfigOption {
	return func(c *Config) {
		c.ClusterContextProducer = producer
	}
}

// WithMaxNumberOfEventsInRequestLogThrottlePeriod sets the max number of events in request log throttled period
func WithMaxNumberOfEventsInRequestLogThrottlePeriod(maxNumber int) ConfigOption {
	return func(c *Config) {
		c.MaxNumberOfEventsInRequestLogThrottledPeriod = maxNumber
	}
}

func WithKinds(kinds ...*Kind) ConfigOption {
	return func(c *Config) {
		for _, kind := range kinds {
			c.Kinds[kind.Kind] = kind
		}
	}
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

// Represents the kinds of actors a cluster can manage
type Kind struct {
	Kind            string
	Props           *actor.Props
	StrategyBuilder func(*Cluster) MemberStrategy
}

// Creates a new instance of a kind
func NewKind(kind string, props *actor.Props) *Kind {
	props.WithOptions(clusterReceiveMiddleware())
	return &Kind{
		Kind:            kind,
		Props:           props,
		StrategyBuilder: nil,
	}
}

func clusterReceiveMiddleware() actor.PropsOption {
	return actor.WithReceiverMiddleware(func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(c actor.ReceiverContext, envelope *actor.MessageEnvelope) {

			//the above code as a type switch
			switch envelope.Message.(type) {
			case *actor.Started:
				handleStarted(c, next, envelope)
			case *actor.Stopped:
				handleStopped(c, next, envelope)
			default:
				next(c, envelope)
			}

			return
		}
	})
}

//TODO: implement
func handleStopped(c actor.ReceiverContext, next actor.ReceiverFunc, envelope *actor.MessageEnvelope) {
	next(c, envelope)

	/*
		   clusterKind.Dec();
		   var cluster = ctx.System.Cluster();
		   var identity = ctx.Get<ClusterIdentity>();

		   if (identity is not null)
		   {
			   ctx.System.EventStream.Publish(new ActivationTerminating
			   {
				   Pid = ctx.Self,
				   ClusterIdentity = identity,
			   });
			   cluster.PidCache.RemoveByVal(identity, ctx.Self);
		   }

		   await baseReceive(ctx, stopEnvelope);
	*/
}

//TODO: implement
func handleStarted(c actor.ReceiverContext, next actor.ReceiverFunc, envelope *actor.MessageEnvelope) {
	next(c, envelope)
	/*
		await baseReceive(ctx, startEnvelope);
		var identity = ctx.Get<ClusterIdentity>();
		var cluster = ctx.System.Cluster();
		#pragma warning disable 618
		var grainInit = new ClusterInit(identity!, cluster);
		#pragma warning restore 618
		var grainInitEnvelope = new MessageEnvelope(grainInit, null);
		clusterKind.Inc();
		await baseReceive(ctx, grainInitEnvelope);
	*/

	//env := actor.WrapEnvelope(&ClusterInit{})
	//next(c, env)
}

func (k *Kind) WithMemberStrategy(strategyBuilder func(*Cluster) MemberStrategy) {
	k.StrategyBuilder = strategyBuilder
}

func (k *Kind) Build(cluster *Cluster) *ActivatedKind {

	var strategy MemberStrategy = nil
	if k.StrategyBuilder != nil {
		strategy = k.StrategyBuilder(cluster)
	}

	return &ActivatedKind{
		Kind:     k.Kind,
		Props:    k.Props,
		Strategy: strategy,
	}
}

type ActivatedKind struct {
	Kind     string
	Props    *actor.Props
	Strategy MemberStrategy
	count    int32
}

func (ak *ActivatedKind) Inc() {
	atomic.AddInt32(&ak.count, 1)
}

func (ak *ActivatedKind) Dev() {
	atomic.AddInt32(&ak.count, -1)
}
