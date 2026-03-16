package natskv

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	natskvmetrics "github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv/metrics"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/metric"
)

// Compile-time interface check.
var _ cluster.ClusterProvider = (*Provider)(nil)

// Provider uses NATS JetStream KV for cluster membership discovery,
// health checking, and leader election.
type Provider struct {
	cluster      *cluster.Cluster
	clusterName  string
	config       *config
	js           jetstream.JetStream
	nc           *nats.Conn // raw NATS connection; nil when created via NewFromJetStream
	memberBucket jetstream.KeyValue
	leaderBucket jetstream.KeyValue
	self         *Node
	isMember     bool
	members      map[string]*Node
	membersMu    sync.RWMutex
	shutdown     atomic.Bool
	clusterError error
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc

	// Leader election
	role                RoleType
	roleMu              sync.Mutex
	roleChangedChan     chan RoleType
	roleChangedListener RoleChangedListener
	schedulers          []*SingletonScheduler
	leaderFuncs         []func(isLeader bool)
	isLeader            atomic.Bool

	// Integrated identity lookup
	identity *IdentityLookup

	// Metrics
	providerMetrics *natskvmetrics.NatsKVMetrics
	metricsEnabled  bool
}

// New creates a Provider using a NATS connection. It creates a JetStream
// context from the connection and delegates to NewFromJetStream.
func New(conn *nats.Conn, opts ...Option) (*Provider, error) {
	js, err := jetstream.New(conn)
	if err != nil {
		return nil, fmt.Errorf("natskv: create JetStream context: %w", err)
	}
	p, err := NewFromJetStream(js, opts...)
	if err != nil {
		return nil, err
	}
	p.nc = conn
	return p, nil
}

// NewFromJetStream creates a Provider using an existing JetStream handle.
func NewFromJetStream(js jetstream.JetStream, opts ...Option) (*Provider, error) {
	cfg := newDefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	p := &Provider{
		config:              cfg,
		js:                  js,
		members:             make(map[string]*Node),
		role:                Follower,
		roleChangedChan:     make(chan RoleType, 1),
		roleChangedListener: cfg.RoleChanged,
	}

	p.identity = newIdentityLookup(p)

	return p, nil
}

// IdentityLookup returns the integrated identity lookup.
func (p *Provider) IdentityLookup() *IdentityLookup {
	return p.identity
}

// GetHealthStatus returns an error if the cluster health status has problems.
func (p *Provider) GetHealthStatus() error {
	return p.clusterError
}

// RegisterSingletonScheduler adds a singleton scheduler to be notified on role changes.
// Must be called before StartMember.
func (p *Provider) RegisterSingletonScheduler(scheduler *SingletonScheduler) {
	p.schedulers = append(p.schedulers, scheduler)
}

// RegisterLeaderFunc registers a callback function that is invoked when
// the node's leadership role changes. The callback receives true when the
// node becomes leader and false when it becomes follower. This provides a
// provider-agnostic way to react to leadership changes without importing
// provider-specific types like RoleType or SingletonScheduler.
// May be called before or after StartMember. If called after StartMember
// and the provider is already the leader, use IsLeader() to check and
// manually invoke the callback.
func (p *Provider) RegisterLeaderFunc(fn func(isLeader bool)) {
	p.leaderFuncs = append(p.leaderFuncs, fn)
}

// IsLeader returns true if this provider currently holds the leader role.
// This is useful when RegisterLeaderFunc is called after StartMember —
// the caller can check IsLeader() and manually trigger the callback if
// the leader election already occurred.
func (p *Provider) IsLeader() bool {
	return p.isLeader.Load()
}

// init extracts host, port, memberID, and kinds from the cluster and builds the self node.
func (p *Provider) init(c *cluster.Cluster) error {
	p.cluster = c
	p.clusterName = c.Config.Name
	addr := c.ActorSystem.Address()
	host, port, err := splitHostPort(addr)
	if err != nil {
		return err
	}

	memberID := c.ActorSystem.ID
	knownKinds := c.GetClusterKinds()
	nodeName := fmt.Sprintf("%v_%v", p.clusterName, memberID)
	p.self = NewNode(nodeName, host, port, knownKinds)
	return nil
}

// StartMember registers the node in NATS KV and starts watching for updates.
func (p *Provider) StartMember(c *cluster.Cluster) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.isMember = true

	if err := p.init(c); err != nil {
		return err
	}

	p.providerMetrics = natskvmetrics.NewNatsKVMetrics(c.Logger())
	p.metricsEnabled = c.MetricsEnabled()

	if err := p.createMemberBucket(); err != nil {
		return err
	}

	if err := p.createLeaderBucket(); err != nil {
		return err
	}

	if err := p.registerSelf(); err != nil {
		return err
	}

	p.startRoleChangedNotifyLoop()

	// Load existing members
	if err := p.loadInitialMembers(); err != nil {
		return err
	}

	p.publishClusterTopologyEvent()
	p.startWatching()
	p.startLeaderWatching()
	p.startRefresh()
	p.attemptLeaderElection()

	return nil
}

// StartClient initializes the provider without registering the node.
// It watches for member changes but does not register itself, refresh, or
// participate in leader election.
func (p *Provider) StartClient(c *cluster.Cluster) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())

	if err := p.init(c); err != nil {
		return err
	}

	p.providerMetrics = natskvmetrics.NewNatsKVMetrics(c.Logger())
	p.metricsEnabled = c.MetricsEnabled()

	if err := p.createMemberBucket(); err != nil {
		return err
	}

	if err := p.loadInitialMembers(); err != nil {
		return err
	}

	p.publishClusterTopologyEvent()
	p.startWatching()

	return nil
}

// Shutdown deregisters the node and stops background tasks.
func (p *Provider) Shutdown(_ bool) error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil
	}

	if p.cancel != nil {
		p.cancel()
	}

	// Delete own member key
	if p.self != nil && p.memberBucket != nil {
		key := p.memberKey(p.self.ID)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.memberBucket.Delete(ctx, key)

		// If leader, delete leader key
		if p.isLeader.Load() && p.leaderBucket != nil {
			_ = p.leaderBucket.Delete(ctx, p.leaderKey())
		}
	}

	// Wait for all goroutines to finish
	p.wg.Wait()

	return nil
}

// UpdateKinds updates the node's kind list and re-registers in the KV
// bucket so other nodes see the change on the next watch event.
func (p *Provider) UpdateKinds(kinds []string) error {
	p.membersMu.Lock()
	p.self.Kinds = kinds
	p.membersMu.Unlock()

	return p.registerSelf()
}

// createMemberBucket creates or binds the member KV bucket.
// The bucket is configured with a TTL so that keys expire if not refreshed,
// and LimitMarkerTTL so that watchers receive delete notifications on expiry.
func (p *Provider) createMemberBucket() error {
	bucketName := p.config.memberBucketName(p.clusterName)

	kv, err := p.js.CreateOrUpdateKeyValue(p.ctx, jetstream.KeyValueConfig{
		Bucket:          bucketName,
		Replicas:        p.config.Replicas,
		TTL:             p.config.MemberTTL,
		LimitMarkerTTL:  p.config.MemberTTL, // emit delete markers for TTL-expired keys
	})
	if err != nil {
		return fmt.Errorf("natskv: create member bucket %q: %w", bucketName, err)
	}

	p.memberBucket = kv
	return nil
}

// createLeaderBucket creates a separate KV bucket for leader election with
// its own TTL (LeaderTTL), so the leader key can have a different TTL than
// member keys.
func (p *Provider) createLeaderBucket() error {
	bucketName := p.config.memberBucketName(p.clusterName) + "_leader"

	kv, err := p.js.CreateOrUpdateKeyValue(p.ctx, jetstream.KeyValueConfig{
		Bucket:         bucketName,
		Replicas:       p.config.Replicas,
		TTL:            p.config.LeaderTTL,
		LimitMarkerTTL: p.config.LeaderTTL,
	})
	if err != nil {
		return fmt.Errorf("natskv: create leader bucket %q: %w", bucketName, err)
	}

	p.leaderBucket = kv
	return nil
}

// memberKey returns the full KV key for a member.
func (p *Provider) memberKey(memberID string) string {
	return p.config.KeyPrefix + ".members." + memberID
}

// leaderKey returns the full KV key for the leader.
func (p *Provider) leaderKey() string {
	return p.config.KeyPrefix + ".leader"
}

// registerSelf writes the member key to the KV bucket. The bucket-level TTL
// handles expiration; startRefresh re-puts the key periodically to keep it alive.
func (p *Provider) registerSelf() error {
	p.membersMu.RLock()
	data, err := p.self.Serialize()
	p.membersMu.RUnlock()
	if err != nil {
		return fmt.Errorf("natskv: serialize self: %w", err)
	}

	key := p.memberKey(p.self.ID)
	_, err = p.memberBucket.Put(p.ctx, key, data)
	if err != nil {
		return fmt.Errorf("natskv: register self: %w", err)
	}

	return nil
}

// loadInitialMembers reads all existing member keys from the KV bucket.
// It uses IncludeHistory to get all current values. The watcher sends nil
// as a sentinel value to indicate the end of initial values.
func (p *Provider) loadInitialMembers() error {
	prefix := p.config.KeyPrefix + ".members."

	watcher, err := p.memberBucket.Watch(p.ctx, prefix+">", jetstream.IncludeHistory())
	if err != nil {
		return fmt.Errorf("natskv: watch initial members: %w", err)
	}
	defer watcher.Stop()

	for entry := range watcher.Updates() {
		if entry == nil {
			// nil signals end of initial values
			break
		}
		if entry.Operation() == jetstream.KeyValueDelete || entry.Operation() == jetstream.KeyValuePurge {
			continue
		}
		p.handleMemberPut(entry)
	}

	return nil
}

// handleMemberPut processes a Put event for a member key.
func (p *Provider) handleMemberPut(entry jetstream.KeyValueEntry) {
	node, err := NewNodeFromBytes(entry.Value())
	if err != nil {
		if p.cluster != nil {
			p.cluster.Logger().Error("Invalid member data",
				slog.String("key", entry.Key()), slog.Any("error", err))
		}
		return
	}

	// Don't track self via the watcher
	if p.self != nil && node.Equal(p.self) {
		return
	}

	p.membersMu.Lock()
	p.members[node.ID] = node
	p.membersMu.Unlock()
}

// handleMemberDelete processes a Delete event for a member key.
func (p *Provider) handleMemberDelete(entry jetstream.KeyValueEntry) {
	memberID := extractMemberID(entry.Key(), p.config.KeyPrefix+".members.")
	if memberID == "" {
		return
	}

	p.membersMu.Lock()
	delete(p.members, memberID)
	p.membersMu.Unlock()
}

// publishClusterTopologyEvent converts internal state to cluster.Member list
// and notifies the cluster's MemberList.
func (p *Provider) publishClusterTopologyEvent() {
	start := time.Now()

	p.membersMu.RLock()
	members := make([]*cluster.Member, 0, len(p.members)+1)
	for _, m := range p.members {
		if m.IsAlive() {
			members = append(members, m.MemberStatus())
		}
	}
	// Include self if registered as a member (not client).
	// Must be inside membersMu scope because MemberStatus() reads p.self.Kinds
	// which is written by UpdateKinds() under membersMu.Lock().
	if p.self != nil && p.isMember {
		members = append(members, p.self.MemberStatus())
	}
	p.membersMu.RUnlock()

	if p.cluster != nil {
		p.cluster.Logger().Debug("Update cluster topology",
			slog.String("provider", "natskv"),
			slog.Int("members", len(members)))
		p.cluster.MemberList.UpdateClusterTopology(members)

		if p.metricsEnabled {
			p.providerMetrics.TopologyUpdateDuration.Record(
				context.Background(),
				time.Since(start).Seconds(),
				metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
			)
		}
	}
}

// startWatching starts the KV watcher goroutine with retry logic and panic recovery.
func (p *Provider) startWatching() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				if p.cluster != nil {
					p.cluster.Logger().Error("Recovered from panic in watcher",
						slog.String("provider", "natskv"),
						slog.Any("error", r))
				}
				p.clusterError = fmt.Errorf("watcher panic: %v", r)
			}
		}()

		for !p.shutdown.Load() {
			if err := p.keepWatching(); err != nil {
				if p.shutdown.Load() {
					return
				}
				if p.cluster != nil {
					p.cluster.Logger().Error("Watcher failed, retrying",
						slog.String("provider", "natskv"),
						slog.Any("error", err))
				}
				p.clusterError = err
				if p.metricsEnabled {
					p.providerMetrics.WatchReconnectCount.Add(
						context.Background(), 1,
						metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
					)
				}

				select {
				case <-time.After(p.config.RetryInterval):
				case <-p.ctx.Done():
					return
				}
			}
		}
	}()
}

// keepWatching runs the KV watcher until an error or shutdown.
// It uses UpdatesOnly so that only new changes are delivered.
func (p *Provider) keepWatching() error {
	prefix := p.config.KeyPrefix + ".members."
	watcher, err := p.memberBucket.Watch(p.ctx, prefix+">", jetstream.UpdatesOnly())
	if err != nil {
		return fmt.Errorf("natskv: start watcher: %w", err)
	}
	defer watcher.Stop()

	for entry := range watcher.Updates() {
		if entry == nil {
			continue
		}

		switch entry.Operation() {
		case jetstream.KeyValuePut:
			p.handleMemberPut(entry)
		case jetstream.KeyValueDelete, jetstream.KeyValuePurge:
			p.handleMemberDelete(entry)
		default:
			if p.cluster != nil {
				p.cluster.Logger().Warn("Unknown KV operation",
					slog.String("key", entry.Key()),
					slog.String("operation", entry.Operation().String()))
			}
		}

		p.publishClusterTopologyEvent()
	}

	return nil
}

// startLeaderWatching starts a KV watcher goroutine that monitors the leader key.
// When the leader key is deleted or expires (TTL), followers re-attempt election.
func (p *Provider) startLeaderWatching() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger().Error("Recovered from panic in leader watcher",
					slog.String("provider", "natskv"),
					slog.Any("error", r))
			}
		}()

		for !p.shutdown.Load() {
			if err := p.keepWatchingLeader(); err != nil {
				if p.shutdown.Load() {
					return
				}
				p.logger().Error("Leader watcher failed, retrying",
					slog.String("provider", "natskv"),
					slog.Any("error", err))

				select {
				case <-time.After(p.config.RetryInterval):
				case <-p.ctx.Done():
					return
				}
			}
		}
	}()
}

// keepWatchingLeader watches the leader key for delete/expiry events and triggers
// re-election when the leader key disappears.
func (p *Provider) keepWatchingLeader() error {
	key := p.leaderKey()
	watcher, err := p.leaderBucket.Watch(p.ctx, key, jetstream.UpdatesOnly())
	if err != nil {
		return fmt.Errorf("natskv: start leader watcher: %w", err)
	}
	defer watcher.Stop()

	for entry := range watcher.Updates() {
		if entry == nil {
			continue
		}

		switch entry.Operation() {
		case jetstream.KeyValueDelete, jetstream.KeyValuePurge:
			// Leader key was deleted or expired. If we are not the current
			// leader, attempt to claim leadership.
			if !p.isLeader.Load() {
				p.attemptLeaderElection()
			}
		case jetstream.KeyValuePut:
			// Leader key was refreshed or claimed by another member. Normal operation.
		}
	}

	return nil
}

// startRefresh starts the goroutine that periodically re-puts the member key
// and leader key (if leader) to keep them alive within the bucket TTL.
func (p *Provider) startRefresh() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				if p.cluster != nil {
					p.cluster.Logger().Error("Recovered from panic in refresh",
						slog.String("provider", "natskv"),
						slog.Any("error", r))
				}
			}
		}()

		ticker := time.NewTicker(p.config.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p.shutdown.Load() {
					return
				}
				if err := p.refreshMemberKey(); err != nil {
					if p.cluster != nil {
						p.cluster.Logger().Warn("Failed to refresh member key",
							slog.String("provider", "natskv"),
							slog.Any("error", err))
					}
					if p.metricsEnabled {
						p.providerMetrics.KeyRefreshFailureCount.Add(
							context.Background(), 1,
							metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
						)
					}
				}
				if err := p.refreshLeaderKey(); err != nil {
					if p.cluster != nil {
						p.cluster.Logger().Warn("Failed to refresh leader key",
							slog.String("provider", "natskv"),
							slog.Any("error", err))
					}
					if p.metricsEnabled {
						p.providerMetrics.KeyRefreshFailureCount.Add(
							context.Background(), 1,
							metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
						)
					}
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// refreshMemberKey re-puts the member key to reset the bucket-level TTL.
func (p *Provider) refreshMemberKey() error {
	p.membersMu.RLock()
	data, err := p.self.Serialize()
	p.membersMu.RUnlock()
	if err != nil {
		return err
	}
	key := p.memberKey(p.self.ID)
	_, err = p.memberBucket.Put(p.ctx, key, data)
	return err
}

// refreshLeaderKey re-puts the leader key if this node is the leader.
func (p *Provider) refreshLeaderKey() error {
	if !p.isLeader.Load() {
		return nil
	}

	data := []byte(fmt.Sprintf(`{"memberID":"%s","electedAt":"%s"}`,
		p.self.ID, time.Now().UTC().Format(time.RFC3339)))
	key := p.leaderKey()
	_, err := p.leaderBucket.Put(p.ctx, key, data)
	if err != nil {
		// Lost leader key -- re-attempt election
		p.isLeader.Store(false)
		p.setRole(Follower)
		p.attemptLeaderElection()
	}
	return err
}

// attemptLeaderElection tries to become leader using atomic Create.
// Create returns ErrKeyExists if the leader key already exists, meaning
// another member holds leadership.
func (p *Provider) attemptLeaderElection() {
	key := p.leaderKey()
	data := []byte(fmt.Sprintf(`{"memberID":"%s","electedAt":"%s"}`,
		p.self.ID, time.Now().UTC().Format(time.RFC3339)))

	_, err := p.leaderBucket.Create(p.ctx, key, data)
	if err != nil {
		// Another member is already leader or NATS error
		return
	}

	// Successfully created -- we are leader
	p.isLeader.Store(true)
	p.setRole(Leader)
	if p.metricsEnabled {
		p.providerMetrics.LeaderElectionCount.Add(
			context.Background(), 1,
			metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
		)
	}
}

// setRole updates the role and notifies listeners.
func (p *Provider) setRole(role RoleType) {
	p.roleMu.Lock()
	if role == p.role {
		p.roleMu.Unlock()
		return
	}

	p.logger().Info("Role changed",
		slog.String("provider", "natskv"),
		slog.String("from", p.role.String()),
		slog.String("to", role.String()))

	p.role = role
	p.roleMu.Unlock()

	// Non-blocking send to role changed channel
	select {
	case p.roleChangedChan <- role:
	default:
	}

	// Notify all registered singleton schedulers
	for _, scheduler := range p.schedulers {
		safeRun(p.logger(), func() {
			scheduler.OnRoleChanged(role)
		})
	}

	// Notify all registered leader functions
	isLeader := role == Leader
	for _, fn := range p.leaderFuncs {
		capturedFn := fn
		safeRun(p.logger(), func() {
			capturedFn(isLeader)
		})
	}
}

// startRoleChangedNotifyLoop starts the goroutine that notifies the
// external role changed listener.
func (p *Provider) startRoleChangedNotifyLoop() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case role := <-p.roleChangedChan:
				if lis := p.roleChangedListener; lis != nil {
					safeRun(p.logger(), func() { lis.OnRoleChanged(role) })
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// logger returns the cluster logger if available, otherwise the default logger.
func (p *Provider) logger() *slog.Logger {
	if p.cluster != nil {
		return p.cluster.Logger()
	}
	return slog.Default()
}

// safeRun executes fn and recovers from panics, logging the stack trace.
func safeRun(logger *slog.Logger, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 64<<10)
			buf = buf[:runtime.Stack(buf, false)]
			logger.Warn("OnRoleChanged panic recovered",
				slog.Any("error", fmt.Errorf("%v\n%s", r, buf)))
		}
	}()
	fn()
}

// extractMemberID extracts the member ID from a KV key by stripping the prefix.
func extractMemberID(key, prefix string) string {
	if len(key) <= len(prefix) {
		return ""
	}
	return key[len(prefix):]
}

// splitHostPort parses an address string into host and port components.
func splitHostPort(addr string) (host string, port int, err error) {
	if h, p, e := net.SplitHostPort(addr); e != nil {
		if addr != "nonhost" {
			err = e
		}
		host = "nonhost"
		port = -1
	} else {
		host = h
		port, err = strconv.Atoi(p)
	}
	return
}
