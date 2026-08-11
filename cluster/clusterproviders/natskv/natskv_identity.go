package natskv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Compile-time check that IdentityLookup implements cluster.IdentityLookup.
var _ cluster.IdentityLookup = (*IdentityLookup)(nil)

// inflight tracks an in-progress Get() call so that concurrent callers
// for the same identity can coalesce on a single result.
type inflight struct {
	done chan struct{}
	pid  *actor.PID
}

// IdentityLookup implements cluster.IdentityLookup directly using NATS JetStream KV
// for lock acquisition, activation storage, and member tracking.
type IdentityLookup struct {
	provider      *Provider
	cluster       *cluster.Cluster
	memberID      string
	isClient      bool
	defunct       atomic.Bool
	identities    jetstream.KeyValue
	memberTracker jetstream.KeyValue
	config        *config
	semaphore     chan struct{}
	setupErr      error
	activationSub *nats.Subscription // member-side subscription for client activation requests

	// Placement actor and proxy PIDs (non-client only). Accessed atomically
	// because Shutdown() nils these while concurrent Get()/RequestFuture paths
	// may be reading them.
	placementPID atomic.Pointer[actor.PID]
	proxyPID     atomic.Pointer[actor.PID]

	// Strategy manager for member selection. Accessed atomically because the
	// topology event handler and Shutdown() may swap it concurrently with
	// resolveIdentity() readers.
	strategyMgr atomic.Pointer[cluster.StrategyManager]

	// Inflight coalescing map.
	inflightMu sync.Mutex
	inflights  map[string]*inflight

	// lockRevisions maps lockID -> NATS KV revision from tryAcquireLock.
	// Used to bridge the revision into the PersistActivation callback.
	lockRevisions sync.Map

	// now is the clock function used by all time-dependent paths in
	// IdentityLookup. Defaults to time.Now; overridable in tests.
	now func() time.Time
}

// activationRecord is the JSON-encoded value stored in the identities KV bucket.
type activationRecord struct {
	LockID     string `json:"lid,omitempty"`
	PidID      string `json:"pid,omitempty"`
	PidAddress string `json:"adr,omitempty"`
	MemberID   string `json:"mid,omitempty"`
}

// memberRecord is the JSON-encoded value stored in the member tracking KV bucket.
type memberRecord struct {
	Keys []string `json:"keys"`
}

// activationReq is sent by cluster clients to members via NATS request/reply
// to trigger remote grain activation.
type activationReq struct {
	Kind     string `json:"k"`
	Identity string `json:"i"`
}

// activationResp is the response from a member after handling an activation request.
type activationResp struct {
	PidID      string `json:"pid,omitempty"`
	PidAddress string `json:"adr,omitempty"`
	Error      string `json:"err,omitempty"`
}

// activateSubject returns the NATS subject used for client-to-member activation requests.
func activateSubject(clusterName string) string {
	return "_protoactor." + clusterName + ".activate"
}

// newIdentityLookup creates a new IdentityLookup associated with the given provider.
func newIdentityLookup(p *Provider) *IdentityLookup {
	return &IdentityLookup{
		provider:  p,
		config:    p.config,
		semaphore: make(chan struct{}, p.config.MaxConcurrency),
		now:       time.Now,
	}
}

// Setup initializes the identity lookup with the cluster context, creates the
// required KV buckets, and subscribes to topology events for member cleanup.
func (il *IdentityLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	il.cluster = c
	// Use the full node name (clusterName_systemID) as memberID to match
	// the Member.Id format returned by Node.MemberStatus(). This ensures
	// the memberTracker KV entries are keyed consistently with the member
	// IDs used by ByMember() lookups and removeMemberID() cleanup.
	il.memberID = fmt.Sprintf("%s_%s", c.Config.Name, c.ActorSystem.ID)
	il.isClient = isClient
	il.inflights = make(map[string]*inflight)

	ctx := context.Background()
	js := il.provider.js
	clusterName := c.Config.Name

	// Create the identities KV bucket (no TTL -- activations persist).
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:   il.config.identityBucketName(clusterName),
		Replicas: il.config.Replicas,
	})
	if err != nil {
		il.setupErr = fmt.Errorf("natskv identity setup failed: create identities bucket: %w", err)
		il.identityLogger().Error("natskv identity: failed to create identities bucket",
			slog.Any("error", err))
		return
	}
	il.identities = identities

	// Create the member tracking KV bucket.
	tracker, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:   "protoactor_" + clusterName + "_identities_tracking",
		Replicas: il.config.Replicas,
	})
	if err != nil {
		il.setupErr = fmt.Errorf("natskv identity setup failed: create tracking bucket: %w", err)
		il.identityLogger().Error("natskv identity: failed to create tracking bucket",
			slog.Any("error", err))
		return
	}
	il.memberTracker = tracker

	// Subscribe to ClusterTopology events to clean up when members leave
	// and to update the strategy manager.
	knownMembers := make(map[string]*cluster.Member)
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			strMgr := il.strategyMgr.Load()
			for _, member := range topology.Left {
				il.removeMemberID(context.Background(), member.Id)
				if strMgr != nil {
					strMgr.RemoveMember(member)
				}
				delete(knownMembers, member.Id)
			}
			for _, member := range topology.Joined {
				if strMgr != nil {
					strMgr.AddMember(member)
				}
				knownMembers[member.Id] = member
			}
			// Handle kind changes for staying members: when a member
			// updates its kinds (e.g., after RegisterKinds), it appears
			// in topology.Members but not in Joined. Remove the stale
			// member and re-add with updated kinds so the strategy
			// manager can find activators for the new kinds.
			if strMgr != nil {
				for _, member := range topology.Members {
					if old, ok := knownMembers[member.Id]; ok && !cluster.KindsEqual(old.Kinds, member.Kinds) {
						strMgr.RemoveMember(old)
						strMgr.AddMember(member)
						knownMembers[member.Id] = member
					}
				}
			}
		}
	})

	// Members subscribe to activation requests from cluster clients.
	// Clients cannot spawn actors, so they publish NATS requests that
	// members handle by performing the full Get() (spawn) protocol.
	if !isClient && il.provider.nc != nil {
		subject := activateSubject(c.Config.Name)
		queueGroup := c.Config.Name + "_activators"
		sub, err := il.provider.nc.QueueSubscribe(subject, queueGroup, il.handleActivationRequest)
		if err != nil {
			il.identityLogger().Error("natskv identity: failed to subscribe to activation requests",
				slog.Any("error", err))
		} else {
			il.activationSub = sub
		}
	}

	// Non-client members: spawn placement actor and proxy.
	if !isClient {
		il.setupPlacementActor(c)
	}
}

// setupPlacementActor spawns the placement actor, proxy actor, and creates
// the strategy manager for non-client members.
func (il *IdentityLookup) setupPlacementActor(c *cluster.Cluster) {
	// Create the PersistActivation callback that bridges lockID -> NATS KV revision.
	// Empty requestID means a remote-initiated request where the requesting
	// node holds the lock and will persist — skip persistence here.
	persistActivation := func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID, requestID string) error {
		if requestID == "" {
			// Remote-initiated activation: the requesting node holds the
			// lock and persists after receiving our response.
			return nil
		}

		// Consume the revision stored by activateLocal().
		revVal, ok := il.lockRevisions.LoadAndDelete(requestID)
		if !ok {
			return cluster.ErrLockNotHeld
		}
		rev := revVal.(uint64)

		// Build the activation record and CAS update.
		key := kvKey(ci)
		updated := activationRecord{
			PidID:      pid.Id,
			PidAddress: pid.Address,
			MemberID:   il.memberID,
		}
		data, err := json.Marshal(&updated)
		if err != nil {
			return fmt.Errorf("natskv persist: marshal failed: %w", err)
		}

		_, err = il.identities.Update(ctx, key, data, rev)
		if err != nil {
			return cluster.ErrLockNotHeld
		}

		// Track this identity key under the member.
		il.addKeyToMember(ctx, il.memberID, key)
		return nil
	}

	// Create the RemoveActivation callback.
	removeActivation := func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID) error {
		key := kvKey(ci)

		// Read the entry to validate the PID before deleting.
		entry, err := il.identities.Get(ctx, key)
		if err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				return nil
			}
			return err
		}

		var rec activationRecord
		if err := json.Unmarshal(entry.Value(), &rec); err != nil {
			return err
		}

		// Only delete if the stored PID matches.
		if rec.PidID != pid.Id || rec.PidAddress != pid.Address {
			return nil
		}

		if rec.MemberID != "" {
			il.removeKeyFromMember(ctx, rec.MemberID, key)
		}

		// CAS delete.
		if err := il.identities.Delete(ctx, key, jetstream.LastRevision(entry.Revision())); err != nil {
			if !errors.Is(err, jetstream.ErrKeyNotFound) && !errors.Is(err, jetstream.ErrKeyExists) {
				return err
			}
		}
		return nil
	}

	config := cluster.PlacementConfig{
		PersistActivation: persistActivation,
		RemoveActivation:  removeActivation,
	}

	// Spawn the placement actor.
	placementProps := cluster.NewPlacementActorProps(c, config)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$placement-activator")
	if err != nil {
		il.identityLogger().Error("natskv identity: failed to spawn placement actor",
			slog.Any("error", err))
	}
	il.placementPID.Store(placementPID)

	// Spawn the proxy actor.
	proxyProps := cluster.NewActivatorProxyProps(placementPID, il)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$proxy-activator")
	if err != nil {
		il.identityLogger().Error("natskv identity: failed to spawn proxy actor",
			slog.Any("error", err))
	}
	il.proxyPID.Store(proxyPID)

	// Create the strategy manager.
	il.strategyMgr.Store(cluster.NewStrategyManager(c))
}

// Get resolves a cluster identity to an actor PID.
//
// The resolution protocol:
//  1. Check PID cache.
//  2. Inflight coalescing — if another goroutine is resolving this identity, wait.
//  3. Check for an existing activation with stale member validation.
//  4. If client, request remote activation via NATS.
//  5. Acquire lock, select target via strategy, route to placement actor.
func (il *IdentityLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	if il.setupErr != nil {
		il.identityLogger().Error("natskv identity: cannot Get, setup failed", slog.Any("error", il.setupErr))
		return nil
	}

	if il.defunct.Load() {
		il.identityLogger().Warn("natskv identity: cannot Get, defunct, shutdown was called")
		return nil
	}

	if err := cluster.ValidateIdentity(ci.Identity); err != nil {
		il.identityLogger().Warn("natskv identity: rejecting invalid identity",
			slog.String("kind", ci.Kind), slog.Any("error", err))
		return nil
	}

	// Step 1: Check PID cache.
	if pid, ok := il.cluster.PidCache.Get(ci.Identity, ci.Kind); ok {
		return pid
	}

	key := ci.AsKey()

	// Step 2: Inflight coalescing.
	il.inflightMu.Lock()
	if inf, ok := il.inflights[key]; ok {
		il.inflightMu.Unlock()
		<-inf.done
		return inf.pid
	}
	inf := &inflight{done: make(chan struct{})}
	il.inflights[key] = inf
	il.inflightMu.Unlock()

	defer func() {
		close(inf.done)
		il.inflightMu.Lock()
		delete(il.inflights, key)
		il.inflightMu.Unlock()
	}()

	pid := il.resolveIdentity(ci)
	inf.pid = pid
	return pid
}

// resolveIdentity performs the actual identity resolution logic.
func (il *IdentityLookup) resolveIdentity(ci *cluster.ClusterIdentity) *actor.PID {
	il.acquire()
	defer il.release()

	ctx := context.Background()

	// Step 3: Check for existing activation with stale validation.
	rec, existingRev := il.getExistingActivationWithRev(ctx, ci)
	if rec != nil {
		if cluster.ValidateActivationMember(il.cluster.MemberList, rec.MemberID) {
			pid := pidFromRecord(rec)
			il.cluster.PidCache.Set(ci.Identity, ci.Kind, pid)
			return pid
		}
		// Stale activation from a dead member — clean it up.
		il.identityLogger().Info("natskv identity: cleaning stale activation",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.String("staleMember", rec.MemberID))
		il.casDelete(ctx, kvKey(ci), existingRev, "resolveIdentity/stale")
		if rec.MemberID != "" {
			il.removeKeyFromMember(ctx, rec.MemberID, kvKey(ci))
		}
	}

	// Step 4: If client, request remote activation from a member via NATS.
	if il.isClient {
		arec := il.requestRemoteActivation(ctx, ci)
		if arec != nil {
			return pidFromRecord(arec)
		}
		return nil
	}

	// Step 5: Try to acquire the spawn lock.
	lockID, revision, ok := il.tryAcquireLock(ctx, ci)
	if !ok {
		// Another node is spawning. Wait for it.
		arec := il.waitForActivation(ctx, ci)
		if arec != nil {
			return pidFromRecord(arec)
		}
		return nil
	}

	// Step 6: Select target member via strategy manager.
	senderAddress := il.cluster.ActorSystem.Address()
	strMgr := il.strategyMgr.Load()
	if strMgr == nil {
		il.identityLogger().Warn("natskv identity: strategy manager is nil (cluster shutting down?)",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		return nil
	}
	targetMember := strMgr.GetActivator(ci, senderAddress)
	if targetMember == nil {
		il.identityLogger().Warn("natskv identity: no available member for activation",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		il.casDelete(ctx, kvKey(ci), revision, "resolveIdentity/no-target")
		return nil
	}

	// Step 7: Route activation request.
	if targetMember.Id == il.memberID {
		return il.activateLocal(ctx, ci, lockID, revision)
	}

	return il.activateRemote(ctx, ci, lockID, revision, targetMember)
}

// activateLocal sends an ActivationRequest to the local placement actor.
// The lock revision is stored in lockRevisions so PersistActivation can use it.
func (il *IdentityLookup) activateLocal(ctx context.Context, ci *cluster.ClusterIdentity, lockID string, revision uint64) *actor.PID {
	// Store the revision so PersistActivation callback can find it.
	il.lockRevisions.Store(lockID, revision)

	defer func() {
		il.lockRevisions.Delete(lockID)
	}()

	req := &cluster.ActivationRequest{
		ClusterIdentity: ci,
		RequestId:       lockID,
	}

	placementPID := il.placementPID.Load()
	if placementPID == nil {
		il.identityLogger().Warn("natskv identity: placement actor is nil (cluster shutting down?)",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		il.casDelete(ctx, kvKey(ci), revision, "activateLocal/nil-placement")
		return nil
	}

	resp, err := il.cluster.ActorSystem.Root.RequestFuture(placementPID, req, 10*time.Second).Result()
	if err != nil {
		il.identityLogger().Error("natskv identity: placement actor request failed",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		il.casDelete(ctx, kvKey(ci), revision, "activateLocal/request-failed")
		return nil
	}

	activationResp, ok := resp.(*cluster.ActivationResponse)
	if !ok || activationResp.Failed || activationResp.Pid == nil {
		il.identityLogger().Warn("natskv identity: placement actor returned failure",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		il.casDelete(ctx, kvKey(ci), revision, "activateLocal/response-failure")
		return nil
	}

	// Populate the local PID cache.
	il.cluster.PidCache.Set(ci.Identity, ci.Kind, activationResp.Pid)

	return activationResp.Pid
}

// activateRemote sends an ActivationRequest to a remote node's proxy actor.
// The remote placement actor spawns the actor but does NOT persist (empty
// requestID causes PersistActivation to no-op). This node holds the lock
// and persists the activation after getting the PID back.
func (il *IdentityLookup) activateRemote(ctx context.Context, ci *cluster.ClusterIdentity, lockID string, revision uint64, member *cluster.Member) *actor.PID {
	proxyPID := actor.NewPID(member.Address(), "$proxy-activator")

	// Empty RequestId tells the remote PersistActivation to skip persistence.
	req := &cluster.ActivationRequest{
		ClusterIdentity: ci,
	}

	resp, err := il.cluster.ActorSystem.Root.RequestFuture(proxyPID, req, 10*time.Second).Result()
	if err != nil {
		il.identityLogger().Error("natskv identity: remote activation request failed",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.String("targetMember", member.Id),
			slog.Any("error", err))
		il.casDelete(ctx, kvKey(ci), revision, "activateRemote/request-failed")
		return nil
	}

	activationResp, ok := resp.(*cluster.ActivationResponse)
	if !ok || activationResp.Failed || activationResp.Pid == nil {
		il.identityLogger().Warn("natskv identity: remote activation returned failure",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.String("targetMember", member.Id))
		il.casDelete(ctx, kvKey(ci), revision, "activateRemote/response-failure")
		return nil
	}

	// Persist the activation locally — we hold the lock.
	storeErr := il.storeActivation(ctx, ci, lockID, revision, member.Id, activationResp.Pid.Address, activationResp.Pid.Id)
	if storeErr != nil {
		il.identityLogger().Error("natskv identity: failed to store remote activation",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", storeErr))
		il.casDelete(ctx, kvKey(ci), revision, "activateRemote/store-failed")
		return nil
	}

	// Populate the local PID cache.
	il.cluster.PidCache.Set(ci.Identity, ci.Kind, activationResp.Pid)

	return activationResp.Pid
}

func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	if il.setupErr != nil {
		il.identityLogger().Error("natskv identity: cannot RemovePid, setup failed", slog.Any("error", il.setupErr))
		return
	}

	// If the actor is running locally, do NOT delete its identity record.
	// This prevents the orphan bug: a timeout triggers RemovePid, but the
	// actor is alive (just slow). Deleting the KV record would leave the
	// process alive but unresolvable.
	if il.cluster != nil && pid.Address == il.cluster.ActorSystem.Address() {
		_, exists := il.cluster.ActorSystem.ProcessRegistry.GetLocal(pid.Id)
		if exists {
			il.identityLogger().Debug("natskv identity: RemovePid skipped, actor is alive locally",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity),
				slog.String("pid", pid.String()))
			return
		}
	}

	ctx := context.Background()
	key := kvKey(ci)

	// Read the entry to validate the PID before deleting.
	entry, err := il.identities.Get(ctx, key)
	if err != nil {
		return // Not found, nothing to remove.
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return
	}

	// Only delete if the stored PID matches the one being removed.
	if rec.PidID != pid.Id || rec.PidAddress != pid.Address {
		return
	}

	if rec.MemberID != "" {
		il.removeKeyFromMember(ctx, rec.MemberID, key)
	}

	// Use CAS delete (LastRevision) so the delete only succeeds if the
	// key hasn't been modified since we read it.
	if err := il.identities.Delete(ctx, key, jetstream.LastRevision(entry.Revision())); err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) && !errors.Is(err, jetstream.ErrKeyExists) {
			il.identityLogger().Error("natskv identity: RemovePid delete failed",
				slog.String("key", key), slog.Any("error", err))
		}
	}
}

// Shutdown performs cleanup when the cluster is shutting down.
// It stops the placement actor first (graceful grain shutdown), then
// stops the proxy, closes the strategy manager, and removes member records.
func (il *IdentityLookup) Shutdown() {
	il.defunct.Store(true)
	// Stop placement actor first — this triggers graceful shutdown of all
	// locally tracked grains (poisons them with DeactivationReasonShutdown).
	if placementPID := il.placementPID.Swap(nil); placementPID != nil {
		if err := il.cluster.ActorSystem.Root.PoisonFuture(placementPID).Wait(); err != nil {
			il.identityLogger().Error("natskv identity: failed to stop placement actor",
				slog.Any("error", err))
		}
	}

	// Stop proxy activator.
	if proxyPID := il.proxyPID.Swap(nil); proxyPID != nil {
		if err := il.cluster.ActorSystem.Root.PoisonFuture(proxyPID).Wait(); err != nil {
			il.identityLogger().Error("natskv identity: failed to stop proxy activator",
				slog.Any("error", err))
		}
	}

	// Close strategy manager.
	if strMgr := il.strategyMgr.Swap(nil); strMgr != nil {
		strMgr.Close()
	}

	if il.activationSub != nil {
		_ = il.activationSub.Unsubscribe()
	}
	if il.setupErr != nil {
		return
	}
	if il.memberID != "" {
		il.removeMemberID(context.Background(), il.memberID)
	}
}

// kvKey converts a ClusterIdentity to a NATS KV-safe key.
//
// The key is the "kind/identity" string from ClusterIdentity.AsKey() with one
// substitution: ':' is rewritten to '_' because colon is not a valid NATS KV
// key character (NATS KV validates against `^[-/_=\.a-zA-Z0-9]+$`). The slash
// between kind and identity is preserved — kinds are forbidden from
// containing '/' (see cluster.ValidateKindName), so the first '/' is always
// the kind/identity boundary and any subsequent '/' is part of the identity.
func kvKey(ci *cluster.ClusterIdentity) string {
	return strings.ReplaceAll(ci.AsKey(), ":", "_")
}

// entryAge returns how long ago a KV entry was created, clamped to zero.
// A negative result (server clock ahead of local clock) is treated as zero
// to avoid incorrect reap decisions caused by clock skew.
func entryAge(e jetstream.KeyValueEntry, now time.Time) time.Duration {
	age := now.Sub(e.Created())
	if age < 0 {
		return 0
	}
	return age
}

// acquire acquires a slot from the concurrency semaphore.
func (il *IdentityLookup) acquire() {
	il.semaphore <- struct{}{}
}

// release releases a slot back to the concurrency semaphore.
func (il *IdentityLookup) release() {
	<-il.semaphore
}

// getExistingActivation looks up the current activation for a cluster identity.
// Returns nil if no activation exists or if the key only contains a lock (no PID).
func (il *IdentityLookup) getExistingActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	rec, _ := il.getExistingActivationWithRev(ctx, ci)
	return rec
}

// getExistingActivationWithRev looks up the current activation for a cluster
// identity and returns both the record and the NATS KV revision of the entry.
// Returns (nil, 0) if no activation exists or if the key only contains a lock.
// Callers that need to CAS-delete the record must use this revision.
func (il *IdentityLookup) getExistingActivationWithRev(ctx context.Context, ci *cluster.ClusterIdentity) (*activationRecord, uint64) {
	key := kvKey(ci)

	entry, err := il.identities.Get(ctx, key)
	if err != nil {
		return nil, 0
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return nil, 0
	}

	// Only return if it's a completed activation (has PID info).
	if rec.PidID == "" || rec.PidAddress == "" {
		return nil, 0
	}

	return &rec, entry.Revision()
}

// casDelete deletes the given key only if its current revision matches rev.
// A CAS miss (ErrKeyExists from jetstream) is a terminal no-op: the key was
// modified after our read, so we must not blind-delete. Any error other than
// ErrKeyNotFound is logged at debug level. Never loops or falls back to
// unconditional delete.
func (il *IdentityLookup) casDelete(ctx context.Context, key string, rev uint64, site string) {
	err := il.identities.Delete(ctx, key, jetstream.LastRevision(rev))
	if err == nil || errors.Is(err, jetstream.ErrKeyNotFound) {
		return
	}
	// CAS miss or transient error: terminal no-op.
	il.identityLogger().Debug("natskv identity: casDelete miss (terminal no-op)",
		slog.String("site", site),
		slog.String("key", key),
		slog.Uint64("rev", rev),
		slog.Any("error", err))
}

// tryAcquireLock attempts to acquire an exclusive spawn lock for the given
// cluster identity using NATS KV Create (atomic, fails if key exists).
// Returns the lock ID, the revision for CAS, and whether the lock was acquired.
// The MemberID field of the written record identifies this node as the lock
// owner so that restart-resilience logic can detect and reap abandoned locks.
func (il *IdentityLookup) tryAcquireLock(ctx context.Context, ci *cluster.ClusterIdentity) (lockID string, revision uint64, ok bool) {
	lockID = uuid.New().String()
	key := kvKey(ci)

	rec := activationRecord{
		LockID:   lockID,
		MemberID: il.memberID,
	}
	data, err := json.Marshal(&rec)
	if err != nil {
		il.identityLogger().Error("natskv identity: tryAcquireLock marshal failed", slog.Any("error", err))
		return "", 0, false
	}

	revision, err = il.identities.Create(ctx, key, data)
	if err != nil {
		if !errors.Is(err, jetstream.ErrKeyExists) {
			il.identityLogger().Error("natskv identity: tryAcquireLock failed",
				slog.String("key", key), slog.Any("error", err))
		}
		return "", 0, false
	}

	return lockID, revision, true
}

// storeActivation stores a completed activation using CAS (revision-based Update),
// then tracks the key in the member's tracking record.
func (il *IdentityLookup) storeActivation(ctx context.Context, ci *cluster.ClusterIdentity, lockID string, revision uint64, memberID, pidAddress, pidID string) error {
	key := kvKey(ci)

	updated := activationRecord{
		LockID:     "", // Clear the lock.
		PidID:      pidID,
		PidAddress: pidAddress,
		MemberID:   memberID,
	}
	data, err := json.Marshal(&updated)
	if err != nil {
		return err
	}

	_, err = il.identities.Update(ctx, key, data, revision)
	if err != nil {
		return err
	}

	// Track this identity key under the member.
	il.addKeyToMember(ctx, memberID, key)
	return nil
}

// waitForActivation watches the NATS KV key for the given cluster identity
// until an activation appears (PID is set), or the lock TTL timeout expires.
func (il *IdentityLookup) waitForActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	key := kvKey(ci)
	watchCtx, cancel := context.WithTimeout(ctx, il.config.LockTTL)
	defer cancel()

	watcher, err := il.identities.Watch(watchCtx, key)
	if err != nil {
		il.identityLogger().Error("natskv identity: waitForActivation watch failed",
			slog.String("key", key), slog.Any("error", err))
		return nil
	}
	defer watcher.Stop()

	for entry := range watcher.Updates() {
		if entry == nil {
			// Initial values done signal -- continue waiting for real updates.
			continue
		}

		if entry.Operation() == jetstream.KeyValueDelete || entry.Operation() == jetstream.KeyValuePurge {
			// Key was deleted -- lock was removed, let caller retry.
			return nil
		}

		var rec activationRecord
		if err := json.Unmarshal(entry.Value(), &rec); err != nil {
			continue
		}

		// Activation is complete when PID is set.
		if rec.PidID != "" && rec.PidAddress != "" {
			return &rec
		}
	}

	return nil
}

// removeMemberID removes all activations belonging to the given member.
// It reads the member tracking record, deletes each identity key, then
// deletes the member record itself.
func (il *IdentityLookup) removeMemberID(ctx context.Context, memberID string) {
	if il.memberTracker == nil {
		return
	}

	entry, err := il.memberTracker.Get(ctx, memberID)
	if err != nil {
		return
	}

	var mrec memberRecord
	if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
		il.identityLogger().Error("natskv identity: removeMemberID unmarshal failed",
			slog.String("memberID", memberID), slog.Any("error", err))
		return
	}

	// Delete each identity key belonging to this member.
	// Read each key first to obtain its current revision, then CAS-delete.
	// If the key is absent between the list read and this read, skip it
	// (another node already cleaned it up). If the record's MemberID has
	// changed, the key was re-activated by a different member -- skip it
	// (the tracking list is stale but the new owner is alive). A CAS miss
	// from a concurrent writer is a terminal no-op.
	for _, key := range mrec.Keys {
		idEntry, err := il.identities.Get(ctx, key)
		if err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				continue // already gone -- nothing to do
			}
			il.identityLogger().Error("natskv identity: removeMemberID get identity failed",
				slog.String("key", key), slog.Any("error", err))
			continue
		}
		// Validate that the stored record still belongs to this member.
		// If another member has re-activated the grain, leave it alone.
		var idRec activationRecord
		if jsonErr := json.Unmarshal(idEntry.Value(), &idRec); jsonErr != nil {
			il.identityLogger().Error("natskv identity: removeMemberID unmarshal identity failed",
				slog.String("key", key), slog.Any("error", jsonErr))
			continue
		}
		if idRec.MemberID != memberID {
			il.identityLogger().Debug("natskv identity: removeMemberID skipping key taken by new member",
				slog.String("key", key),
				slog.String("currentOwner", idRec.MemberID),
				slog.String("removedMember", memberID))
			continue
		}
		il.casDelete(ctx, key, idEntry.Revision(), "removeMemberID")
	}

	// Delete the member tracking record itself.
	if err := il.memberTracker.Delete(ctx, memberID); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
		il.identityLogger().Error("natskv identity: removeMemberID delete member failed",
			slog.String("memberID", memberID), slog.Any("error", err))
	}
}

// addKeyToMember adds an identity key to a member's tracking record.
// Uses a CAS-loop with retry on conflict.
func (il *IdentityLookup) addKeyToMember(ctx context.Context, memberID, key string) {
	for i := 0; i < 3; i++ {
		entry, err := il.memberTracker.Get(ctx, memberID)
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			// Create a new member record.
			mrec := memberRecord{Keys: []string{key}}
			data, err := json.Marshal(&mrec)
			if err != nil {
				il.identityLogger().Error("natskv identity: addKeyToMember marshal failed",
					slog.String("memberID", memberID), slog.Any("error", err))
				return
			}
			_, err = il.memberTracker.Create(ctx, memberID, data)
			if err == nil {
				return
			}
			if errors.Is(err, jetstream.ErrKeyExists) {
				// Another goroutine created it first -- retry with update.
				continue
			}
			il.identityLogger().Error("natskv identity: addKeyToMember create failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}
		if err != nil {
			il.identityLogger().Error("natskv identity: addKeyToMember get failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		var mrec memberRecord
		if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
			il.identityLogger().Error("natskv identity: addKeyToMember unmarshal failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		// Check if key already exists.
		for _, k := range mrec.Keys {
			if k == key {
				return
			}
		}

		mrec.Keys = append(mrec.Keys, key)
		data, err := json.Marshal(&mrec)
		if err != nil {
			il.identityLogger().Error("natskv identity: addKeyToMember marshal failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		_, err = il.memberTracker.Update(ctx, memberID, data, entry.Revision())
		if err == nil {
			return
		}
		// CAS conflict -- retry.
		time.Sleep(10 * time.Millisecond)
	}
}

// removeKeyFromMember removes an identity key from a member's tracking record.
// Uses a CAS-loop with retry on conflict.
func (il *IdentityLookup) removeKeyFromMember(ctx context.Context, memberID, key string) {
	for i := 0; i < 3; i++ {
		entry, err := il.memberTracker.Get(ctx, memberID)
		if err != nil {
			return
		}

		var mrec memberRecord
		if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
			il.identityLogger().Error("natskv identity: removeKeyFromMember unmarshal failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		// Filter out the key.
		filtered := make([]string, 0, len(mrec.Keys))
		for _, k := range mrec.Keys {
			if k != key {
				filtered = append(filtered, k)
			}
		}
		mrec.Keys = filtered

		data, err := json.Marshal(&mrec)
		if err != nil {
			il.identityLogger().Error("natskv identity: removeKeyFromMember marshal failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		_, err = il.memberTracker.Update(ctx, memberID, data, entry.Revision())
		if err == nil {
			return
		}
		// CAS conflict -- retry.
		time.Sleep(10 * time.Millisecond)
	}
}

// handleActivationRequest is the NATS subscription handler for client-initiated
// activation requests. It runs on member nodes and performs the full Get()
// (spawn) protocol, then responds with the resulting PID.
func (il *IdentityLookup) handleActivationRequest(msg *nats.Msg) {
	var req activationReq
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		resp, _ := json.Marshal(activationResp{Error: "bad request"})
		_ = msg.Respond(resp)
		return
	}

	ci := &cluster.ClusterIdentity{Kind: req.Kind, Identity: req.Identity}
	pid := il.Get(ci)
	if pid == nil {
		resp, _ := json.Marshal(activationResp{Error: "activation failed"})
		_ = msg.Respond(resp)
		return
	}

	resp, _ := json.Marshal(activationResp{PidID: pid.Id, PidAddress: pid.Address})
	_ = msg.Respond(resp)
}

// requestRemoteActivation sends a NATS request to a cluster member asking it
// to activate the given grain. This is used by cluster clients which cannot
// spawn actors themselves. Returns the activation record on success, nil on failure.
func (il *IdentityLookup) requestRemoteActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	nc := il.provider.nc
	if nc == nil {
		// No raw NATS connection available; fall back to passive wait.
		return il.waitForActivation(ctx, ci)
	}

	subject := activateSubject(il.cluster.Config.Name)
	data, err := json.Marshal(activationReq{Kind: ci.Kind, Identity: ci.Identity})
	if err != nil {
		return nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, il.config.LockTTL)
	defer cancel()

	resp, err := nc.RequestWithContext(reqCtx, subject, data)
	if err != nil {
		il.identityLogger().Error("natskv identity: remote activation request failed",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		return nil
	}

	var aresp activationResp
	if err := json.Unmarshal(resp.Data, &aresp); err != nil {
		return nil
	}

	if aresp.Error != "" || aresp.PidID == "" {
		return nil
	}

	// Populate the local PID cache so subsequent requests skip the remote call.
	pid := actor.NewPID(aresp.PidAddress, aresp.PidID)
	il.cluster.PidCache.Set(ci.Identity, ci.Kind, pid)

	return &activationRecord{
		PidID:      aresp.PidID,
		PidAddress: aresp.PidAddress,
	}
}

// identityLogger returns the cluster logger if available, otherwise the default logger.
func (il *IdentityLookup) identityLogger() *slog.Logger {
	if il.cluster != nil {
		return il.cluster.Logger()
	}
	return slog.Default()
}

// pidFromRecord converts an activationRecord into an actor.PID.
func pidFromRecord(rec *activationRecord) *actor.PID {
	return actor.NewPID(rec.PidAddress, rec.PidID)
}

// Compile-time check that IdentityLookup implements cluster.GrainEnumerator.
var _ cluster.GrainEnumerator = (*IdentityLookup)(nil)

// ListGrains returns all known grain activations across all members.
func (il *IdentityLookup) ListGrains() ([]*cluster.GrainInfo, error) {
	if il.setupErr != nil {
		return nil, il.setupErr
	}

	ctx := context.Background()

	memberKeys, err := il.memberTracker.Keys(ctx)
	if errors.Is(err, jetstream.ErrNoKeysFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("natskv identity: list member keys: %w", err)
	}

	var result []*cluster.GrainInfo
	for _, memberID := range memberKeys {
		grains, err := il.listMemberGrains(ctx, memberID)
		if err != nil {
			continue
		}
		result = append(result, grains...)
	}

	return result, nil
}

// ListGrainsByKind returns grain activations filtered by kind.
func (il *IdentityLookup) ListGrainsByKind(kind string) ([]*cluster.GrainInfo, error) {
	all, err := il.ListGrains()
	if err != nil {
		return nil, err
	}
	var result []*cluster.GrainInfo
	for _, g := range all {
		if g.Kind == kind {
			result = append(result, g)
		}
	}
	return result, nil
}

// ListGrainsByMember returns grain activations owned by a specific member.
func (il *IdentityLookup) ListGrainsByMember(memberID string) ([]*cluster.GrainInfo, error) {
	if il.setupErr != nil {
		return nil, il.setupErr
	}
	return il.listMemberGrains(context.Background(), memberID)
}

// listMemberGrains reads all activation records belonging to a specific member
// from the member tracker and identities buckets.
func (il *IdentityLookup) listMemberGrains(ctx context.Context, memberID string) ([]*cluster.GrainInfo, error) {
	entry, err := il.memberTracker.Get(ctx, memberID)
	if err != nil {
		return nil, err
	}

	var mrec memberRecord
	if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
		return nil, err
	}

	var result []*cluster.GrainInfo
	for _, key := range mrec.Keys {
		idEntry, err := il.identities.Get(ctx, key)
		if err != nil {
			continue
		}

		var rec activationRecord
		if err := json.Unmarshal(idEntry.Value(), &rec); err != nil {
			continue
		}

		if rec.PidID == "" {
			continue
		}

		// kvKey produces "kind/identity"; split on the first '/' so that
		// identities containing '/' round-trip correctly. Kind names are
		// validated to never contain '/' (see cluster.ValidateKindName).
		// Reverse the ':'->'_' substitution applied by kvKey is NOT possible
		// here without losing fidelity for identities that legitimately
		// contained '_' — kinds and identities with ':' will surface here as
		// the post-substitution form, which is consistent with how they were
		// stored. Callers wanting the original form should track it themselves.
		kind, identity := cluster.ParseStoredActivationInfoKey(key)
		result = append(result, &cluster.GrainInfo{
			Identity: identity,
			Kind:     kind,
			PID:      pidFromRecord(&rec),
			MemberID: memberID,
		})
	}

	return result, nil
}

// Peek checks if a grain activation exists without triggering activation.
func (il *IdentityLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	notFound := &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
		},
		Status: cluster.PeekStatusNotFound,
	}

	if il.setupErr != nil {
		return nil, fmt.Errorf("natskv identity: cannot Peek, setup failed: %w", il.setupErr)
	}
	if il.defunct.Load() {
		return nil, fmt.Errorf("natskv identity: cannot Peek, shutdown was called")
	}

	// Step 1: Check for existing activation in NATS KV.
	ctx := context.Background()
	existing := il.getExistingActivation(ctx, clusterIdentity)
	if existing == nil {
		return notFound, nil
	}

	// Step 2: Validate owning member is alive.
	if !cluster.ValidateActivationMember(il.cluster.MemberList, existing.MemberID) {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				MemberID: existing.MemberID,
			},
			Status: cluster.PeekStatusMemberDead,
		}, nil
	}

	// Step 3: Confirm process alive via placement actor.
	ownerAddress := existing.PidAddress
	proxyPID := actor.NewPID(ownerAddress, "$proxy-activator")
	future := il.cluster.ActorSystem.Root.RequestFuture(proxyPID, &cluster.PeekRequest{
		ClusterIdentity: clusterIdentity,
	}, 5*time.Second)

	res, err := future.Result()
	if err != nil {
		return nil, fmt.Errorf("peek request to %s failed: %w", ownerAddress, err)
	}

	peekResp, ok := res.(*cluster.PeekResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", res)
	}

	if peekResp.Found {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				PID:      peekResp.Pid,
				MemberID: existing.MemberID,
			},
			Status: cluster.PeekStatusAlive,
		}, nil
	}

	return &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
			MemberID: existing.MemberID,
		},
		Status: cluster.PeekStatusStale,
	}, nil
}
