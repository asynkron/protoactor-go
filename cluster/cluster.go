// Package cluster enables distributed actors and grain management.
package cluster

import (
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/asynkron/gofun/set"

	"github.com/asynkron/protoactor-go/actor"
	clustermetrics "github.com/asynkron/protoactor-go/cluster/metrics"
	"github.com/asynkron/protoactor-go/extensions"
	"github.com/asynkron/protoactor-go/remote"
)

var extensionID = extensions.NextExtensionID()

type Cluster struct {
	ActorSystem    *actor.ActorSystem
	Config         *Config
	Gossip         *Gossiper
	PubSub         *PubSub
	Remote         *remote.Remote
	PidCache       *PidCacheValue
	MemberList     *MemberList
	IdentityLookup IdentityLookup
	kinds          map[string]*ActivatedKind
	context        Context

	metrics        *clustermetrics.ClusterMetrics
	metricsEnabled bool
}

var _ extensions.Extension = &Cluster{}

// NewCluster creates a new Cluster instance.
func NewCluster(actorSystem *actor.ActorSystem, config *Config) *Cluster {
	c := &Cluster{
		ActorSystem: actorSystem,
		Config:      config,
		kinds:       map[string]*ActivatedKind{},
	}
	actorSystem.Extensions.Register(c)

	if actorSystem.Config.MetricsEnabled {
		c.metrics = clustermetrics.NewClusterMetrics(actorSystem.Logger())
		c.metricsEnabled = true
	}

	c.context = config.ClusterContextProducer(c)
	c.PidCache = NewPidCache()
	c.MemberList = NewMemberList(c)
	c.subscribeToTopologyEvents()

	actorSystem.Extensions.Register(c)

	var err error
	c.Gossip, err = newGossiper(c)
	c.PubSub = NewPubSub(c)

	if err != nil {
		panic(err)
	}

	return c
}

// New creates a new Cluster instance.
// Deprecated: Use NewCluster instead.
func New(actorSystem *actor.ActorSystem, config *Config) *Cluster {
	return NewCluster(actorSystem, config)
}

func (c *Cluster) subscribeToTopologyEvents() {
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if clusterTopology, ok := evt.(*ClusterTopology); ok {
			for _, member := range clusterTopology.Left {
				c.PidCache.RemoveByMember(member)
			}
			if c.metricsEnabled {
				c.metrics.ClusterMembersCount.Set(int64(len(clusterTopology.Members)))
			}
		}
	})
}

func (c *Cluster) MetricsEnabled() bool { return c.metricsEnabled }

func (c *Cluster) Metrics() *clustermetrics.ClusterMetrics { return c.metrics }

func (c *Cluster) VirtualActorCount() int64 {
	var total int64
	for _, k := range c.kinds {
		total += int64(k.Count())
	}
	return total
}

func (c *Cluster) ExtensionID() extensions.ExtensionID {
	return extensionID
}

//goland:noinspection GoUnusedExportedFunction
func GetCluster(actorSystem *actor.ActorSystem) *Cluster {
	r := actorSystem.Extensions.Get(extensionID)
	if r == nil {
		return nil
	}
	c, ok := r.(*Cluster)
	if !ok {
		return nil
	}
	return c
}

func (c *Cluster) GetBlockedMembers() set.Set[string] {
	return c.Remote.BlockList().BlockedMembers()
}

func (c *Cluster) StartMember() error {
	cfg := c.Config
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)

	c.initKinds()

	// TODO: make it possible to become a cluster even if remoting is already started
	if err := c.Remote.Start(); err != nil {
		return fmt.Errorf("failed to start remote: %w", err)
	}

	address := c.ActorSystem.Address()
	c.Logger().Info("Starting Proto.Actor cluster member", slog.String("address", address))

	c.IdentityLookup = cfg.IdentityLookup
	c.IdentityLookup.Setup(c, c.GetClusterKinds(), false)

	// TODO: Disable Gossip for now until API changes are done
	// gossiper must be started whenever any topology events starts flowing
	if err := c.Gossip.StartGossiping(); err != nil {
		return fmt.Errorf("failed to start gossiping: %w", err)
	}
	if err := c.PubSub.Start(); err != nil {
		return fmt.Errorf("failed to start PubSub: %w", err)
	}
	c.MemberList.InitializeTopologyConsensus()

	if err := cfg.ClusterProvider.StartMember(c); err != nil {
		return fmt.Errorf("failed to start cluster provider member: %w", err)
	}

	time.Sleep(1 * time.Second)
	return nil
}

func (c *Cluster) GetClusterKinds() []string {
	keys := make([]string, 0, len(c.kinds))
	for k := range c.kinds {
		keys = append(keys, k)
	}

	return keys
}

func (c *Cluster) StartClient() error {
	cfg := c.Config
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)

	if err := c.Remote.Start(); err != nil {
		return fmt.Errorf("failed to start remote: %w", err)
	}

	address := c.ActorSystem.Address()
	c.Logger().Info("Starting Proto.Actor cluster-client", slog.String("address", address))

	c.IdentityLookup = cfg.IdentityLookup
	c.IdentityLookup.Setup(c, c.GetClusterKinds(), true)

	if err := cfg.ClusterProvider.StartClient(c); err != nil {
		return fmt.Errorf("failed to start cluster provider client: %w", err)
	}
	if err := c.PubSub.Start(); err != nil {
		return fmt.Errorf("failed to start PubSub: %w", err)
	}
	return nil
}

func (c *Cluster) Shutdown(graceful bool) {
	c.Gossip.SetState(GracefullyLeftKey, &emptypb.Empty{})
	c.ActorSystem.Shutdown()
	if graceful {
		if err := c.Config.ClusterProvider.Shutdown(graceful); err != nil {
			c.Logger().Error("cluster provider shutdown failed", slog.Any("error", err))
		}
		c.IdentityLookup.Shutdown()
		// This is to wait ownership transferring complete.
		time.Sleep(time.Millisecond * 2000)
		c.MemberList.stopMemberList()
		c.IdentityLookup.Shutdown()
		c.Gossip.Shutdown()
	}

	c.Remote.Shutdown(graceful)

	address := c.ActorSystem.Address()
	c.Logger().Info("Stopped Proto.Actor cluster", slog.String("address", address))
}

// Get resolves the PID for the given identity and kind.
// It returns nil if the kind is not registered or the activation fails.
func (c *Cluster) Get(identity string, kind string) *actor.PID {
	return c.IdentityLookup.Get(NewClusterIdentity(identity, kind))
}

func (c *Cluster) Request(identity string, kind string, message any, option ...GrainCallOption) (any, error) {
	return c.context.Request(identity, kind, message, option...)
}

func (c *Cluster) RequestFuture(identity string, kind string, message any, option ...GrainCallOption) (actor.Future, error) {
	return c.context.RequestFuture(identity, kind, message, option...)
}

func (c *Cluster) GetClusterKind(kind string) *ActivatedKind {
	k, ok := c.kinds[kind]
	if !ok {
		c.Logger().Error("Invalid kind", slog.String("kind", kind))

		return nil
	}

	return k
}

func (c *Cluster) TryGetClusterKind(kind string) (*ActivatedKind, bool) {
	k, ok := c.kinds[kind]

	return k, ok
}

func (c *Cluster) initKinds() {
	for name, kind := range c.Config.Kinds {
		c.kinds[name] = kind.Build(c)
	}
	c.ensureTopicKindRegistered()
}

// InitKindsForTest builds and registers the given kinds without starting
// the full cluster. This is intended for unit tests that exercise
// individual components (e.g. the placement actor) in isolation.
func (c *Cluster) InitKindsForTest(kinds ...*Kind) {
	for _, kind := range kinds {
		c.kinds[kind.Kind] = kind.Build(c)
	}
}

// ensureTopicKindRegistered ensures that the topic kind is registered in the cluster
// if topic kind is not registered, it will be registered automatically
func (c *Cluster) ensureTopicKindRegistered() {
	hasTopicKind := false
	for name := range c.kinds {
		if name == TopicActorKind {
			hasTopicKind = true
			break
		}
	}
	if !hasTopicKind {
		store := &EmptyKeyValueStore[*Subscribers]{}
		storeTimeout := c.Config.PubSubConfig.SubscriptionStoreTimeout

		c.kinds[TopicActorKind] = NewKind(TopicActorKind, actor.PropsFromProducer(func() actor.Actor {
			return NewTopicActor(store, c.Logger(), storeTimeout)
		})).Build(c)
	}
}

func (c *Cluster) Logger() *slog.Logger {
	return c.ActorSystem.Logger()
}
