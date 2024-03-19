package cluster

import (
	"log/slog"
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
	RequestLog                                   bool
	MaxNumberOfEventsInRequestLogThrottledPeriod int
	ClusterContextProducer                       ContextProducer
	MemberStrategyBuilder                        func(cluster *Cluster, kind string) MemberStrategy
	Kinds                                        map[string]*Kind
	TimeoutTime                                  time.Duration
	GossipInterval                               time.Duration
	GossipRequestTimeout                         time.Duration
	GossipFanOut                                 int
	GossipMaxSend                                int
	HeartbeatExpiration                          time.Duration // Gossip heartbeat timeout. If the member does not update its heartbeat within this period, it will be added to the BlockList
	PubSubConfig                                 *PubSubConfig
}

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
		HeartbeatExpiration:  time.Second * 20,
		PubSubConfig:         newPubSubConfig(),
	}

	for _, option := range options {
		option(config)
	}

	return config
}

// ToClusterContextConfig converts this cluster Config Context parameters
// into a valid ClusterContextConfig value and returns a pointer to its memory
func (c *Config) ToClusterContextConfig(logger *slog.Logger) *ClusterContextConfig {
	clusterContextConfig := ClusterContextConfig{
		RequestsLogThrottlePeriod:                    c.RequestsLogThrottlePeriod,
		MaxNumberOfEventsInRequestLogThrottledPeriod: c.MaxNumberOfEventsInRequestLogThrottledPeriod,

		requestLogThrottle: actor.NewThrottleWithLogger(logger,
			int32(defaultMaxNumberOfEvetsInRequestLogThrottledPeriod),
			defaultRequestsLogThrottlePeriod,
			func(logger *slog.Logger, i int32) {
				logger.Info("Throttled %d Request logs", slog.Int("count", int(i)))
			},
		),
	}
	return &clusterContextConfig
}

func WithClusterIdentity(props *actor.Props, ci *ClusterIdentity) *actor.Props {
	// inject the cluster identity into the actor context
	p := props.Clone(
		actor.WithOnInit(func(ctx actor.Context) {
			SetClusterIdentity(ctx, ci)
		}))
	return p
}

func withClusterReceiveMiddleware() actor.PropsOption {
	return actor.WithReceiverMiddleware(func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(c actor.ReceiverContext, envelope *actor.MessageEnvelope) {
			// the above code as a type switch
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

func handleStopped(c actor.ReceiverContext, next actor.ReceiverFunc, envelope *actor.MessageEnvelope) {
	/*
	   clusterKind.Dec();
	*/
	cl := GetCluster(c.ActorSystem())
	identity := GetClusterIdentity(c)

	if identity != nil {
		cl.ActorSystem.EventStream.Publish(&ActivationTerminating{
			Pid:             c.Self(),
			ClusterIdentity: identity,
		})
		cl.PidCache.RemoveByValue(identity.Identity, identity.Kind, c.Self())
	}

	next(c, envelope)
}

func handleStarted(c actor.ReceiverContext, next actor.ReceiverFunc, envelope *actor.MessageEnvelope) {
	next(c, envelope)
	cl := GetCluster(c.ActorSystem())
	identity := GetClusterIdentity(c)

	grainInit := &ClusterInit{
		Identity: identity,
		Cluster:  cl,
	}

	ge := actor.WrapEnvelope(grainInit)
	next(c, ge)
}
