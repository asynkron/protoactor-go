package natskv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	natskvmetrics "github.com/awevoke/protoactor-go/cluster/clusterproviders/natskv/metrics"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/metric"
)

// Compile-time interface checks.
var _ cluster.ClusterProvider = (*Provider)(nil)
var _ cluster.SingletonSchedulerRegistrar = (*Provider)(nil)

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
	// publishMu serializes publishClusterTopologyEvent so that a snapshot
	// computed by one publisher (watcher or reconcile goroutine) is always
	// applied to MemberList before the next publisher computes and applies its
	// own -- preventing an older topology from clobbering a newer one.
	publishMu sync.Mutex
	shutdown  atomic.Bool
	// clusterError is written by the watcher and reconcile goroutines and read
	// by GetHealthStatus; clusterErrMu guards it.
	clusterError error
	clusterErrMu sync.Mutex
	// reconcileMisses holds the member IDs that were absent from the live KV
	// key set during the previous reconcile pass. A member is only pruned once
	// it is absent in two consecutive passes (see reconcileMembers). Accessed
	// only from the single reconcile goroutine (serial), so it needs no lock.
	reconcileMisses map[string]struct{}
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc

	// Leader election
	role                cluster.RoleType
	roleMu              sync.Mutex
	roleChangedChan     chan cluster.RoleType
	roleChangedListener cluster.RoleChangedListener
	schedulers          []cluster.RoleChangedListener
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
		reconcileMisses:     make(map[string]struct{}),
		role:                cluster.RoleFollower,
		roleChangedChan:     make(chan cluster.RoleType, 1),
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
	p.clusterErrMu.Lock()
	defer p.clusterErrMu.Unlock()
	return p.clusterError
}

// setClusterError records the most recent background-goroutine error surfaced
// by GetHealthStatus. Safe to call from any goroutine.
func (p *Provider) setClusterError(err error) {
	p.clusterErrMu.Lock()
	p.clusterError = err
	p.clusterErrMu.Unlock()
}

// RegisterSingletonScheduler registers a RoleChangedListener to be notified on
// leadership role changes. Safe to call before or after StartMember.
// If the node is already leader, the listener is immediately notified.
func (p *Provider) RegisterSingletonScheduler(listener cluster.RoleChangedListener) {
	if p.shutdown.Load() {
		return
	}
	p.roleMu.Lock()
	defer p.roleMu.Unlock()
	p.schedulers = append(p.schedulers, listener)
	if p.role == cluster.RoleLeader {
		cluster.SafeRunRoleChange(p.logger(), func() {
			listener.OnRoleChanged(cluster.RoleLeader)
		})
	}
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
	p.startReconcile()
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
	p.startReconcile()

	return nil
}

// Shutdown deregisters the node and stops background tasks. If the node was
// the leader, it first transitions itself to follower so registered
// SingletonSchedulers stop their actors and any RoleChangedListeners observe
// the demotion before the KV state is torn down.
func (p *Provider) Shutdown(_ bool) error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil
	}

	// Snapshot leader state before stepping down so we still know to delete
	// the leader key below.
	wasLeader := p.isLeader.Load()
	p.isLeader.Store(false)
	// Synchronously notify listeners. SingletonScheduler.OnRoleChanged blocks
	// on PoisonFuture.Wait, so by the time setRole returns, singleton actors
	// have stopped (bounded by StopTimeout).
	p.setRole(cluster.RoleFollower)

	if p.cancel != nil {
		p.cancel()
	}

	// Delete own member key
	if p.self != nil && p.memberBucket != nil {
		key := p.memberKey(p.self.ID)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.memberBucket.Delete(ctx, key)

		if wasLeader && p.leaderBucket != nil {
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
		Bucket:         bucketName,
		Replicas:       p.config.Replicas,
		TTL:            p.config.MemberTTL,
		LimitMarkerTTL: p.config.MemberTTL, // emit delete markers for TTL-expired keys
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
	// Serialize the whole compute-and-apply so concurrent publishers (the
	// watcher and the reconcile goroutine) cannot interleave and apply an older
	// snapshot after a newer one. publishMu is always the outermost lock here;
	// no other code path acquires it, so it cannot invert with membersMu.
	p.publishMu.Lock()
	defer p.publishMu.Unlock()

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
				p.setClusterError(fmt.Errorf("watcher panic: %v", r))
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
				p.setClusterError(err)
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

// startReconcile starts a goroutine that periodically reconciles the in-memory
// member set against the live member keys in the KV bucket. This makes the
// provider self-healing: if a delete/expiry event for a member key is missed by
// the UpdatesOnly watcher (e.g. during watcher reconnects or crash churn), the
// stale member is pruned within one reconcile interval instead of persisting
// until a full restart. If ReconcileInterval <= 0, reconciliation is disabled.
func (p *Provider) startReconcile() {
	if p.config.ReconcileInterval <= 0 {
		return
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				if p.cluster != nil {
					p.cluster.Logger().Error("Recovered from panic in reconcile",
						slog.String("provider", "natskv"),
						slog.Any("error", r))
				}
				p.setClusterError(fmt.Errorf("reconcile panic: %v", r))
			}
		}()

		ticker := time.NewTicker(p.config.ReconcileInterval)
		defer ticker.Stop()

		for !p.shutdown.Load() {
			select {
			case <-ticker.C:
				if err := p.reconcileMembers(); err != nil {
					if p.shutdown.Load() {
						return
					}
					if p.cluster != nil {
						p.cluster.Logger().Error("Reconcile failed, retrying",
							slog.String("provider", "natskv"),
							slog.Any("error", err))
					}
					p.setClusterError(err)
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// reconcileMembers re-reads the set of live member keys from the KV bucket and
// reconciles the in-memory member set against it, in both directions:
//
//   - Prune: an in-memory member whose key is absent from the bucket in TWO
//     consecutive passes is removed. This corrects for delete/expiry events
//     that were never delivered to the watcher. The two-pass grace is what
//     makes pruning race-free: a member is present in p.members only because
//     its Put was observed, which means its key is durably in the bucket, so it
//     always reappears in the next snapshot. Only a genuine ghost -- absent
//     every pass -- is pruned. A member that merely joins concurrently with a
//     single snapshot (absent for one pass) is never falsely pruned, which
//     would otherwise permanently block a live node via the member block list.
//   - Upsert: a live key with no corresponding in-memory member is re-added.
//     This corrects for Put events the watcher missed (e.g. a dead or
//     reconnecting watcher), so the provider self-heals adds as well as deletes.
//
// The live-key snapshot uses ListKeysFiltered, which delivers last-per-subject
// with delete markers ignored, so it returns exactly the keys with a current
// value regardless of the bucket's history depth. If the reconcile changes the
// member set, the cluster topology is re-published.
func (p *Provider) reconcileMembers() error {
	prefix := p.config.KeyPrefix + ".members."

	lister, err := p.memberBucket.ListKeysFiltered(p.ctx, prefix+">")
	if err != nil {
		return fmt.Errorf("natskv: list members for reconcile: %w", err)
	}
	live := make(map[string]struct{})
	for key := range lister.Keys() {
		if id := extractMemberID(key, prefix); id != "" {
			live[id] = struct{}{}
		}
	}

	// Under the members lock, collect prune candidates (members absent from the
	// live snapshot for a second consecutive pass) and live IDs missing from
	// memory (for upsert). Candidates are not deleted here: each is first
	// confirmed absent with an authoritative point Get below, so that a
	// snapshot that was truncated (e.g. a watcher subscription closed
	// mid-listing surfaces no error) cannot cause a live member to be pruned.
	// The candidate's *Node is captured so the later delete can verify the
	// entry was not replaced in the meantime by a concurrent watcher Put.
	missingNow := make(map[string]struct{})
	pruneCandidates := make(map[string]*Node)
	var toUpsert []string

	p.membersMu.Lock()
	for id, node := range p.members {
		if p.self != nil && id == p.self.ID {
			continue // never prune self
		}
		if _, ok := live[id]; ok {
			continue
		}
		missingNow[id] = struct{}{}
		if _, missedBefore := p.reconcileMisses[id]; missedBefore {
			pruneCandidates[id] = node
		}
	}
	for id := range live {
		if p.self != nil && id == p.self.ID {
			continue
		}
		if _, ok := p.members[id]; !ok {
			toUpsert = append(toUpsert, id)
		}
	}
	p.membersMu.Unlock()

	// Carry the current missing set forward for the next pass's grace check.
	// (reconcileMembers is only called from the single, serial reconcile
	// goroutine, so this needs no additional synchronization.)
	p.reconcileMisses = missingNow

	// Confirm each prune candidate is really gone with a point Get before
	// deleting. Only a definitive ErrKeyNotFound authorizes a prune; a present
	// key (the snapshot was wrong) or a transient error leaves the member in
	// place, to be re-evaluated next pass. The delete re-checks under the lock
	// that the same *Node is still mapped, so a member re-added by the watcher
	// between the Get and the delete is not removed.
	pruned := 0
	for id, node := range pruneCandidates {
		_, gerr := p.memberBucket.Get(p.ctx, p.memberKey(id))
		if !errors.Is(gerr, jetstream.ErrKeyNotFound) {
			continue
		}
		p.membersMu.Lock()
		if cur, ok := p.members[id]; ok && cur == node {
			delete(p.members, id)
			pruned++
		}
		p.membersMu.Unlock()
	}

	// Upsert live members the watcher missed. The value is fetched outside the
	// members lock; adding a member whose key is present in KV is always
	// correct, because the key exists only if that member registered it.
	added := 0
	for _, id := range toUpsert {
		entry, gerr := p.memberBucket.Get(p.ctx, p.memberKey(id))
		if gerr != nil {
			continue // key vanished between snapshot and Get, or a transient error
		}
		node, nerr := NewNodeFromBytes(entry.Value())
		if nerr != nil {
			continue
		}
		if p.self != nil && node.Equal(p.self) {
			continue
		}
		p.membersMu.Lock()
		if _, exists := p.members[node.ID]; !exists {
			p.members[node.ID] = node
			added++
		}
		p.membersMu.Unlock()
	}

	if pruned > 0 || added > 0 {
		if p.cluster != nil {
			p.cluster.Logger().Info("Reconciled cluster members",
				slog.String("provider", "natskv"),
				slog.Int("pruned", pruned),
				slog.Int("added", added))
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
		p.setRole(cluster.RoleFollower)
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
	p.setRole(cluster.RoleLeader)
	if p.metricsEnabled {
		p.providerMetrics.LeaderElectionCount.Add(
			context.Background(), 1,
			metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...),
		)
	}
}

// setRole updates the role and notifies listeners.
func (p *Provider) setRole(role cluster.RoleType) {
	p.roleMu.Lock()
	defer p.roleMu.Unlock()

	if role == p.role {
		return
	}

	p.logger().Info("Role changed",
		slog.String("provider", "natskv"),
		slog.String("from", p.role.String()),
		slog.String("to", role.String()))

	p.role = role

	// Non-blocking send to role changed channel
	select {
	case p.roleChangedChan <- role:
	default:
	}

	// Notify all registered singleton schedulers
	for _, scheduler := range p.schedulers {
		cluster.SafeRunRoleChange(p.logger(), func() {
			scheduler.OnRoleChanged(role)
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
					cluster.SafeRunRoleChange(p.logger(), func() { lis.OnRoleChanged(role) })
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

// extractMemberID extracts the member ID from a KV key by stripping the prefix.
func extractMemberID(key, prefix string) string {
	if len(key) <= len(prefix) {
		return ""
	}
	return key[len(prefix):]
}

// IsLeader reports whether this node is the current cluster leader.
func (p *Provider) IsLeader() bool {
	return p.isLeader.Load()
}

// MemberKeyExists reports whether a member key currently exists in the members KV bucket.
// It performs a direct KV Get — not the in-memory map — so it detects member keys
// that were restored between janitor sweeps.
func (p *Provider) MemberKeyExists(ctx context.Context, memberID string) bool {
	if p.memberBucket == nil {
		return false
	}
	_, err := p.memberBucket.Get(ctx, p.memberKey(memberID))
	return err == nil
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
