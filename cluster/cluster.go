// Package cluster enables distributed actors and grain management.
package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/asynkron/gofun/set"

	"github.com/awevoke/protoactor-go/actor"
	clustermetrics "github.com/awevoke/protoactor-go/cluster/metrics"
	"github.com/awevoke/protoactor-go/extensions"
	"github.com/awevoke/protoactor-go/remote"
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
	kindsMu        sync.RWMutex
	kinds          map[string]*ActivatedKind
	provider       ClusterProvider
	context        Context

	metrics             *clustermetrics.ClusterMetrics
	metricsEnabled      bool
	deactivationReasons *deactivationReasons
	grainMetrics        *grainMetricsStore // nil unless WithGrainMetrics() is set
	grainReg            *GrainRegistry
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
	c.PidCache = NewPidCacheWithTTL(config.PidCacheTTL)
	c.deactivationReasons = newDeactivationReasons()
	if config.GrainMetricsEnabled {
		c.grainMetrics = newGrainMetricsStore()
	}
	c.grainReg = &GrainRegistry{cluster: c}
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
				_ctx := context.Background()
				attrs := actor.SystemLabels(c.ActorSystem)
				c.metrics.ClusterMembersCount.Set(int64(len(clusterTopology.Members)))
				c.metrics.ClusterTopologyUpdateCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
				c.metrics.ClusterMemberJoinCount.Add(_ctx, int64(len(clusterTopology.Joined)), metric.WithAttributes(attrs...))
				c.metrics.ClusterMemberLeaveCount.Add(_ctx, int64(len(clusterTopology.Left)), metric.WithAttributes(attrs...))
			}
		}
	})
}

func (c *Cluster) MetricsEnabled() bool { return c.metricsEnabled }

func (c *Cluster) Metrics() *clustermetrics.ClusterMetrics { return c.metrics }

func (c *Cluster) VirtualActorCount() int64 {
	c.kindsMu.RLock()
	defer c.kindsMu.RUnlock()
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
	c.provider = cfg.ClusterProvider
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
	c.kindsMu.RLock()
	defer c.kindsMu.RUnlock()
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
		time.Sleep(time.Millisecond * 1000)
		c.MemberList.stopMemberList()
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

// Peek checks whether a grain activation exists without triggering activation.
// Returns a PeekResult with liveness status. See PeekStatus for possible states.
func (c *Cluster) Peek(identity, kind string) (*PeekResult, error) {
	if c.IdentityLookup == nil {
		return nil, fmt.Errorf("cluster not started")
	}
	return c.IdentityLookup.Peek(NewClusterIdentity(identity, kind))
}

func (c *Cluster) Request(identity string, kind string, message any, option ...GrainCallOption) (any, error) {
	return c.context.Request(identity, kind, message, option...)
}

func (c *Cluster) RequestFuture(identity string, kind string, message any, option ...GrainCallOption) (actor.Future, error) {
	return c.context.RequestFuture(identity, kind, message, option...)
}

func (c *Cluster) GetClusterKind(kind string) *ActivatedKind {
	c.kindsMu.RLock()
	defer c.kindsMu.RUnlock()
	k, ok := c.kinds[kind]
	if !ok {
		c.Logger().Error("Invalid kind", slog.String("kind", kind))
		return nil
	}
	return k
}

func (c *Cluster) TryGetClusterKind(kind string) (*ActivatedKind, bool) {
	c.kindsMu.RLock()
	defer c.kindsMu.RUnlock()
	k, ok := c.kinds[kind]
	return k, ok
}

func (c *Cluster) initKinds() {
	activated := make(map[string]*ActivatedKind, len(c.Config.Kinds))
	for name, kind := range c.Config.Kinds {
		activated[name] = kind.Build(c)
	}

	c.kindsMu.Lock()
	defer c.kindsMu.Unlock()
	for name, ak := range activated {
		c.kinds[name] = ak
	}
	c.ensureTopicKindRegisteredLocked()
}

// InitKindsForTest builds and registers the given kinds without starting
// the full cluster. This is intended for unit tests that exercise
// individual components (e.g. the placement actor) in isolation.
func (c *Cluster) InitKindsForTest(kinds ...*Kind) {
	activated := make(map[string]*ActivatedKind, len(kinds))
	for _, kind := range kinds {
		activated[kind.Kind] = kind.Build(c)
	}

	c.kindsMu.Lock()
	defer c.kindsMu.Unlock()
	for name, ak := range activated {
		c.kinds[name] = ak
	}
}

// RegisterKind registers a new Kind with the cluster at runtime.
// The Kind becomes available for local activation immediately.
// Other cluster members discover the new Kind on the next topology
// update cycle (typically within one heartbeat interval).
//
// Returns an error if a Kind with the same name is already registered.
func (c *Cluster) RegisterKind(kind *Kind) error {
	// Build outside the lock — Build may call StrategyBuilder(c)
	// which may acquire kindsMu.RLock via TryGetClusterKind.
	activated := kind.Build(c)

	c.kindsMu.Lock()
	defer c.kindsMu.Unlock()

	if _, exists := c.kinds[kind.Kind]; exists {
		return fmt.Errorf("kind %q is already registered", kind.Kind)
	}

	c.kinds[kind.Kind] = activated
	c.notifyKindUpdate()
	return nil
}

// RegisterKinds registers multiple Kinds with the cluster in a single
// batch, triggering only one topology notification at the end. This
// avoids the topology churn that occurs when registering kinds one by
// one, which can cause singleton re-placement during the update window.
//
// Returns an error if any Kind name is already registered; in that case
// no kinds from the batch are registered (all-or-nothing).
func (c *Cluster) RegisterKinds(kinds []*Kind) error {
	if len(kinds) == 0 {
		return nil
	}

	// Build all kinds outside the lock.
	type built struct {
		name      string
		activated *ActivatedKind
	}
	activated := make([]built, len(kinds))
	for i, k := range kinds {
		activated[i] = built{name: k.Kind, activated: k.Build(c)}
	}

	c.kindsMu.Lock()
	defer c.kindsMu.Unlock()

	// Check for duplicates first (all-or-nothing).
	for _, b := range activated {
		if _, exists := c.kinds[b.name]; exists {
			return fmt.Errorf("kind %q is already registered", b.name)
		}
	}

	// Register all.
	for _, b := range activated {
		c.kinds[b.name] = b.activated
	}

	// Single topology notification.
	c.notifyKindUpdate()
	return nil
}

// DeregisterKind removes a Kind from the cluster. Returns an error if
// the Kind doesn't exist. The TopicActorKind cannot be deregistered.
func (c *Cluster) DeregisterKind(kindName string) error {
	c.kindsMu.Lock()
	defer c.kindsMu.Unlock()

	if kindName == TopicActorKind {
		return fmt.Errorf("kind %q is reserved and cannot be deregistered", kindName)
	}

	if _, exists := c.kinds[kindName]; !exists {
		return fmt.Errorf("kind %q is not registered", kindName)
	}

	delete(c.kinds, kindName)
	c.notifyKindUpdate()
	return nil
}

// getClusterKindsLocked returns kind names. Caller must hold kindsMu.
func (c *Cluster) getClusterKindsLocked() []string {
	keys := make([]string, 0, len(c.kinds))
	for k := range c.kinds {
		keys = append(keys, k)
	}
	return keys
}

func (c *Cluster) notifyKindUpdate() {
	// kindsMu must be held by caller.
	if c.provider == nil {
		return
	}
	if updater, ok := c.provider.(KindUpdater); ok {
		kinds := c.getClusterKindsLocked()
		if err := updater.UpdateKinds(kinds); err != nil {
			c.Logger().Error("Failed to notify provider of kind update",
				slog.Any("error", err))
		}
	}
}

// RegisterSingletonScheduler registers a RoleChangedListener with the cluster provider.
// The listener will be notified of leadership role changes. If the provider is already
// the leader, the listener is immediately notified.
// Returns an error if the cluster provider does not support singleton scheduling.
// The Cluster must have been created (via cluster.Configure) before calling this method,
// but it may be called before or after StartMember.
func (c *Cluster) RegisterSingletonScheduler(listener RoleChangedListener) error {
	prov := c.provider
	if prov == nil {
		if c.Config == nil || c.Config.ClusterProvider == nil {
			return fmt.Errorf("cluster provider not configured")
		} else {
			prov = c.Config.ClusterProvider
		}
	}
	if registrar, ok := prov.(SingletonSchedulerRegistrar); ok {
		registrar.RegisterSingletonScheduler(listener)
		return nil
	}
	return fmt.Errorf("cluster provider %T does not support singleton scheduling", c.provider)
}

// ensureTopicKindRegisteredLocked ensures that the topic kind is registered.
// Caller must hold kindsMu.Lock.
func (c *Cluster) ensureTopicKindRegisteredLocked() {
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

// SetDeactivationReason records why a grain is being deactivated.
// This must be called before stopping the grain actor. The reason is
// consumed by the handleStopped middleware and published in GrainDeactivated.
func (c *Cluster) SetDeactivationReason(pid *actor.PID, reason DeactivationReason) {
	c.deactivationReasons.Set(pid, reason)
}

// GrainRegistry returns the cluster's grain registry for introspection.
func (c *Cluster) GrainRegistry() *GrainRegistry {
	return c.grainReg
}
