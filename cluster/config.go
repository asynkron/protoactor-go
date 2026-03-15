package cluster

import (
	"fmt"
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
	GrainMetricsEnabled                          bool
	// PidCacheTTL sets the time-to-live for PID cache entries. Expiration is
	// lazy (on-read): stale entries are evicted when Get() is called after the
	// TTL has elapsed, not by a background reaper. Zero means no expiry.
	PidCacheTTL time.Duration
}

// validate checks the cluster configuration for invalid values and returns an
// error if any field is out of range or a required dependency is nil.
func (c *Config) validate() error {
	if c.Name == "" {
		return fmt.Errorf("cluster name must not be empty")
	}
	if c.ClusterProvider == nil {
		return fmt.Errorf("ClusterProvider must not be nil")
	}
	if c.IdentityLookup == nil {
		return fmt.Errorf("IdentityLookup must not be nil")
	}
	if c.RemoteConfig == nil {
		return fmt.Errorf("RemoteConfig must not be nil")
	}
	if c.RequestTimeoutTime <= 0 {
		return fmt.Errorf("RequestTimeoutTime must be > 0")
	}
	return nil
}

// ConfigureWithError creates a new cluster configuration and validates it.
// It returns an error if the configuration is invalid.
func ConfigureWithError(clusterName string, clusterProvider ClusterProvider, identityLookup IdentityLookup, remoteConfig *remote.Config, options ...ConfigOption) (*Config, error) {
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

	if err := config.validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Configure creates a new cluster configuration. It panics if the
// configuration is invalid. Use ConfigureWithError for a non-panicking variant.
func Configure(clusterName string, clusterProvider ClusterProvider, identityLookup IdentityLookup, remoteConfig *remote.Config, options ...ConfigOption) *Config {
	config, err := ConfigureWithError(clusterName, clusterProvider, identityLookup, remoteConfig, options...)
	if err != nil {
		panic(err)
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
				handleGrainMetrics(c, envelope.Message)
				next(c, envelope)
			}

		}
	})
}

func handleStopped(c actor.ReceiverContext, next actor.ReceiverFunc, envelope *actor.MessageEnvelope) {
	cl := GetCluster(c.ActorSystem())
	identity := GetClusterIdentity(c)

	if identity != nil {
		// Existing event — kept for backward compatibility.
		cl.ActorSystem.EventStream.Publish(&ActivationTerminating{
			Pid:             c.Self(),
			ClusterIdentity: identity,
		})
		cl.PidCache.RemoveByValue(identity.Identity, identity.Kind, c.Self())

		// New enriched event with deactivation reason.
		reason := cl.deactivationReasons.Pop(c.Self())
		cl.ActorSystem.EventStream.Publish(&GrainDeactivated{
			ClusterIdentity: identity,
			PID:             c.Self(),
			Reason:          reason,
		})

		// Clean up grain metrics entry.
		if cl.grainMetrics != nil {
			cl.grainMetrics.Remove(identity.AsKey())
		}
	}

	next(c, envelope)
}

func handleGrainMetrics(c actor.ReceiverContext, msg any) {
	// Skip system and auto-receive messages (Stopping, Restarting, etc.)
	// and cluster-internal messages (ClusterInit). Only count user messages.
	switch msg.(type) {
	case actor.AutoReceiveMessage, *ClusterInit:
		return
	}

	cl := GetCluster(c.ActorSystem())
	if cl == nil || cl.grainMetrics == nil {
		return
	}
	identity := GetClusterIdentity(c)
	if identity == nil {
		return
	}
	cl.grainMetrics.Record(identity.AsKey())
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

	if identity != nil {
		cl.ActorSystem.EventStream.Publish(&GrainActivated{
			ClusterIdentity: identity,
			PID:             c.Self(),
		})
	}
}
