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

// absenceClock tracks the first time each activation key was observed to have
// an absent owner. It is used to implement the ActivationAbsentGrace window:
// on the first Get() that finds an absent owner the timestamp is recorded;
// cleanup is deferred until a subsequent Get() finds the grace elapsed.
//
// All methods are safe for concurrent use.
type absenceClock struct {
	mu  sync.Mutex
	obs map[string]time.Time // key -> first-absent observation time
}

// firstAbsent returns the first-observed-absent time for key. If no prior
// observation exists it records now and returns now.
// Safe to call on the zero-value absenceClock (obs is lazy-initialized).
func (a *absenceClock) firstAbsent(key string, now time.Time) time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.obs == nil {
		a.obs = make(map[string]time.Time)
	}
	if t, ok := a.obs[key]; ok {
		return t
	}
	a.obs[key] = now
	return now
}

// clear removes key from the absence map. It is called when the owner is
// confirmed present, when the identity record is absent entirely, or after
// cleanup fires. Safe to call on the zero-value absenceClock.
func (a *absenceClock) clear(key string) {
	a.mu.Lock()
	delete(a.obs, key)
	a.mu.Unlock()
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
	poisonSub     *nats.Subscription // member-side subscription for peer remove-and-poison requests

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

	// absence tracks when each identity key was first observed to have an
	// absent owner. See absenceClock for semantics.
	absence absenceClock

	// janitorStop is closed to signal the background janitor goroutine to stop.
	// It is non-nil only on non-client (member) nodes.
	janitorStop     chan struct{}
	janitorStopOnce sync.Once

	// writeGuard tracks consecutive identity-write failures and drives the
	// fail-stop watchdog. See writeFailureGuard.
	writeGuard writeFailureGuard

	// markerTTLEnabled records whether the identities bucket was actually
	// created with marker TTLs. It is set once, in Setup, and read only by
	// tombstoneOpts. It is load-bearing, not informational: a PurgeTTL stamps
	// a Nats-TTL header, and the server rejects that header on a stream with
	// AllowMsgTTL: false, so attaching it to a bucket that fell back would
	// make every identity delete fail.
	markerTTLEnabled bool
}

// SetsOwnPidCache returns true, signalling to DefaultContext that this
// IdentityLookup manages its own PidCache.Set calls and that DefaultContext
// must not perform a redundant Set on the result of Get().
func (il *IdentityLookup) SetsOwnPidCache() bool { return true }

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

// poisonReq asks a member to remove-and-poison a specific grain PID via its
// local placement actor. Used by the remote store-failure cleanup path, which
// cannot send the (non-proto) RemoveAndPoisonRequest across the wire directly.
//
// Kind/Identity name the identity record so the receiving member can re-read it
// and validate the request before honoring it. Revision is the caller's held
// lock revision at the moment its persist failed: the record must still be at
// that revision (i.e. still the loser's unresolved lock-only record) for the
// poison to be honored. This prevents a replayed/spoofed poisonReq from an
// earlier legitimately-failed activation from killing a later, legitimate
// re-activation of the same identity — a successor's persist (or a new lock)
// bumps the record's revision away from the carried value, so the replay is
// rejected. See handleRemoveAndPoisonRequest.
type poisonReq struct {
	Kind       string `json:"k"`
	Identity   string `json:"i"`
	PidID      string `json:"pid"`
	PidAddress string `json:"adr"`
	Revision   uint64 `json:"rev"`
}

// poisonResp acknowledges a poisonReq. Ok is true once the grain was removed
// from tracking and poisoned (or was already gone).
type poisonResp struct {
	Ok    bool   `json:"ok"`
	Error string `json:"err,omitempty"`
}

// poisonSubject returns the NATS subject used for member-to-member
// remove-and-poison requests.
func poisonSubject(clusterName string) string {
	return "_protoactor." + clusterName + ".poison"
}

// newIdentityLookup creates a new IdentityLookup associated with the given provider.
func newIdentityLookup(p *Provider) *IdentityLookup {
	return &IdentityLookup{
		provider:  p,
		config:    p.config,
		semaphore: make(chan struct{}, p.config.MaxConcurrency),
		now:       time.Now,
		absence:   absenceClock{obs: make(map[string]time.Time)},
	}
}

// createBucketWithMarkerTTL creates cfg's bucket with LimitMarkerTTL set, and
// retries once without it when the server does not support marker TTLs.
//
// LimitMarkerTTL requires JetStream API level >= 1; nats.go probes AccountInfo
// and returns ErrLimitMarkerTTLNotSupported below that. A cluster provider that
// fails to start is far worse than tombstones accumulating, so an unsupported
// server degrades to the previous behaviour with a WARN naming the reason.
// Every OTHER error is fatal and propagates: a fallback that hid a dead NATS
// connection would hide a broken identity write path.
//
// The returned bool is load-bearing, not informational: casDelete's PurgeTTL
// stamps a Nats-TTL header, and nats-server rejects that header on a stream
// with AllowMsgTTL: false. Attaching it to a bucket that fell back would make
// every identity delete fail. See tombstoneOpts.
//
// Round-trip cost: the TTL-carrying attempt adds one AccountInfo call per
// bucket per process start (nats.go issues one from prepareKeyValueConfig
// regardless, and a second one only when LimitMarkerTTL is set).
func createBucketWithMarkerTTL(
	ctx context.Context,
	js jetstream.JetStream,
	cfg jetstream.KeyValueConfig,
	ttl time.Duration,
	logger *slog.Logger,
) (kv jetstream.KeyValue, markerTTLEnabled bool, err error) {
	if ttl > 0 {
		withTTL := cfg
		withTTL.LimitMarkerTTL = ttl

		kv, err = js.CreateOrUpdateKeyValue(ctx, withTTL)
		if err == nil {
			return kv, true, nil
		}

		if !errors.Is(err, jetstream.ErrLimitMarkerTTLNotSupported) {
			return nil, false, err
		}

		logger.Warn("natskv identity: server does not support KV marker TTLs; delete markers will accumulate",
			slog.String("bucket", cfg.Bucket),
			slog.Duration("requestedTTL", ttl))
	}

	kv, err = js.CreateOrUpdateKeyValue(ctx, cfg)
	if err != nil {
		return nil, false, err
	}

	return kv, false, nil
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

	// Create the identities KV bucket. Records themselves have no TTL --
	// activations persist -- but the delete MARKERS they leave behind expire
	// server-side after TombstoneTTL. Without that, the bucket's retained
	// message count grows with every identity ever activated instead of with
	// the live set, and every janitor ListKeys pays transport for the markers
	// because nats.go filters them client-side.
	//
	// The enable is ONE-WAY on an existing bucket: nats-server's stream update
	// check refuses any config that clears AllowMsgTTL ("message TTL status
	// can not be disabled"), so a bucket cannot be moved back to
	// no-marker-TTL in place. That has a sharp operational edge -- reverting
	// to a build that creates these buckets WITHOUT LimitMarkerTTL makes this
	// very call fail, i.e. fails Setup, against an already-upgraded bucket.
	// Rollback is therefore bucket rotation via WithIdentityBucket, which is
	// already the documented incident lever, not a code revert on its own.
	//
	// Re-pushing the config is otherwise safe here because both buckets are
	// created with ONLY Bucket and Replicas set, so LimitMarkerTTL is the
	// single field that changes. Enabling it does not make the server start
	// planting markers of its own: it emits subject-delete markers only when
	// limits remove the last message on a subject (MaxAge, per-message TTL),
	// and these buckets set neither -- the only TTL-bearing message they ever
	// carry is the purge marker casDelete writes.
	identities, identityMarkerTTL, err := createBucketWithMarkerTTL(ctx, js, jetstream.KeyValueConfig{
		Bucket:   il.config.identityBucketName(clusterName),
		Replicas: il.config.Replicas,
	}, il.config.TombstoneTTL, il.identityLogger())
	if err != nil {
		il.setupErr = fmt.Errorf("natskv identity setup failed: create identities bucket: %w", err)
		il.identityLogger().Error("natskv identity: failed to create identities bucket",
			slog.Any("error", err))
		return
	}
	il.identities = identities

	// Create the member tracking KV bucket, on the same terms.
	tracker, trackerMarkerTTL, err := createBucketWithMarkerTTL(ctx, js, jetstream.KeyValueConfig{
		Bucket:   "protoactor_" + clusterName + "_identities_tracking",
		Replicas: il.config.Replicas,
	}, il.config.TombstoneTTL, il.identityLogger())
	if err != nil {
		il.setupErr = fmt.Errorf("natskv identity setup failed: create tracking bucket: %w", err)
		il.identityLogger().Error("natskv identity: failed to create tracking bucket",
			slog.Any("error", err))
		return
	}
	il.memberTracker = tracker

	// The identities bucket's answer is the one that governs, since that is
	// the bucket casDelete purges. The two cannot disagree -- same server,
	// same call -- so a disagreement means an assumption broke; take the
	// conjunction (the safe direction: no PurgeTTL) and say so.
	il.markerTTLEnabled = identityMarkerTTL && trackerMarkerTTL
	if identityMarkerTTL != trackerMarkerTTL {
		il.identityLogger().Warn("natskv identity: KV marker TTL support differs between buckets",
			slog.Bool("identities", identityMarkerTTL),
			slog.Bool("tracking", trackerMarkerTTL))
	}

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

		// Subscribe (per-member, no queue group) to remove-and-poison requests
		// so a peer that failed to persist a remote activation can ask us to
		// tear down the grain we spawned. Not a queue group: the request is
		// addressed to the specific member that hosts the grain, using a
		// member-scoped subject.
		poisonSub, err := il.provider.nc.Subscribe(
			poisonSubject(c.Config.Name)+"."+il.memberID, il.handleRemoveAndPoisonRequest)
		if err != nil {
			il.identityLogger().Error("natskv identity: failed to subscribe to poison requests",
				slog.Any("error", err))
		} else {
			il.poisonSub = poisonSub
		}
	}

	// Non-client members: spawn placement actor and proxy.
	if !isClient {
		il.setupPlacementActor(c)
		il.janitorStop = make(chan struct{})
		go il.runJanitor()
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
		il.recordIdentityWriteOutcome("persistActivation/update", false, err)
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
		err = il.identities.Delete(ctx, key, jetstream.LastRevision(entry.Revision()))
		il.recordIdentityWriteOutcome("removeActivation/delete", true, err)
		if err != nil {
			if !errors.Is(err, jetstream.ErrKeyNotFound) && !errors.Is(err, jetstream.ErrKeyExists) {
				return err
			}
		}
		return nil
	}

	config := cluster.PlacementConfig{
		PersistActivation:     persistActivation,
		RemoveActivation:      removeActivation,
		CheckActivationRecord: il.checkActivationRecord,
		CleanupOwnRecord:      il.cleanupOwnRecord,
		SelfCheckHardReapAge:  il.config.HardReapAge,
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
// It runs a bounded retry loop (up to 3 passes) starting at the
// existing-activation check (step 3). Each pass can:
//   - Return immediately with a valid PID.
//   - Reap an abandoned lock and continue to the next pass.
//   - Wait for an in-progress activation and either return the PID or
//     continue (if the lock was deleted and resolution should retry).
//   - Acquire the lock and activate locally or remotely.
func (il *IdentityLookup) resolveIdentity(ci *cluster.ClusterIdentity) *actor.PID {
	il.acquire()
	defer il.release()

	ctx := context.Background()

resolution:
	for pass := 0; pass < 3; pass++ {
		// Step 3: Check for existing activation with stale validation.
		key := kvKey(ci)
		rec, existingRev := il.getExistingActivationWithRev(ctx, ci)
		if rec == nil {
			// No activation record exists. If there was an absence entry for
			// this key (from a prior Get that found an absent-owner record),
			// clear it: the record is now gone entirely.
			il.absence.clear(key)
		} else if cluster.ValidateActivationMember(il.cluster.MemberList, rec.MemberID) {
			// Owner is present — clear any stale absence entry and return the PID.
			il.absence.clear(key)
			pid := pidFromRecord(rec)
			il.cluster.PidCache.Set(ci.Identity, ci.Kind, pid)
			return pid
		} else {
			// Owner is absent. Apply the ActivationAbsentGrace window.
			now := il.now()
			firstSeen := il.absence.firstAbsent(key, now)
			if now.Sub(firstSeen) < il.config.ActivationAbsentGrace {
				// Still within grace — return the stale PID without caching it.
				// The caller gets a usable PID; the grace window protects us
				// from a premature cleanup during a rolling restart.
				return pidFromRecord(rec)
			}
			// Grace has elapsed — perform cleanup and fall through to
			// lock/spawn resolution (the loop continues).
			il.identityLogger().Info("natskv identity: cleaning stale activation after grace elapsed",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity),
				slog.String("staleMember", rec.MemberID))
			il.casDelete(ctx, key, existingRev, "resolveIdentity/stale-after-grace")
			if rec.MemberID != "" {
				il.removeKeyFromMember(ctx, rec.MemberID, key)
			}
			il.absence.clear(key)
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
			// Another node holds the lock. Check whether it is an abandoned
			// lock we can reap; if so, continue to the next pass.
			if il.maybeReapLock(ctx, ci) {
				continue resolution
			}
			// Lock is held by a live owner — wait for activation to complete.
			arec := il.waitForActivation(ctx, ci)
			if arec != nil {
				return pidFromRecord(arec)
			}
			// waitForActivation returned nil: the lock was deleted (KeyValueDelete).
			// Re-enter the loop so we can try to acquire the lock ourselves.
			continue resolution
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

	il.identityLogger().Warn("natskv identity: resolution loop exhausted",
		slog.String("kind", ci.Kind),
		slog.String("identity", ci.Identity))
	return nil
}

// maybeReapLock checks whether the current identity record is an abandoned
// lock that can be forcibly reaped, and deletes it if so.
//
// Reap conditions (both require PidID == ""):
//   - Owner-absence branch: record has MemberID, the owner is absent from the
//     member list, and entryAge > LockOwnerAbsentGrace.
//   - Hard branch: entryAge > HardReapAge, regardless of owner state.
//     This is the only branch that applies to legacy records with empty MemberID.
//
// Returns true if the lock was reaped (deleted), false otherwise.
// A CAS miss on delete is a terminal no-op (another node raced us); the
// function still returns true so the caller re-enters the resolution loop.
func (il *IdentityLookup) maybeReapLock(ctx context.Context, ci *cluster.ClusterIdentity) bool {
	key := kvKey(ci)

	entry, err := il.identities.Get(ctx, key)
	if err != nil {
		// Key absent or unreadable: nothing to reap.
		return false
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return false
	}

	// Only lock-only records are eligible for reaping (PidID must be empty).
	if rec.PidID != "" {
		return false
	}

	age := entryAge(entry, il.now())

	// Hard branch: age alone qualifies — fires for both named and legacy records.
	if age > il.config.HardReapAge {
		il.identityLogger().Info("natskv identity: reaping stale lock (hard age threshold)",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.String("memberID", rec.MemberID),
			slog.Duration("age", age),
			slog.Duration("hardReapAge", il.config.HardReapAge))
		il.casDelete(ctx, key, entry.Revision(), "maybeReapLock/hard")
		return true
	}

	// Owner-absence branch: only applies when MemberID is set.
	if rec.MemberID != "" {
		ownerAbsent := il.cluster == nil || !cluster.ValidateActivationMember(il.cluster.MemberList, rec.MemberID)
		if ownerAbsent && age > il.config.LockOwnerAbsentGrace {
			il.identityLogger().Info("natskv identity: reaping stale lock (owner absent)",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity),
				slog.String("memberID", rec.MemberID),
				slog.Duration("age", age),
				slog.Duration("grace", il.config.LockOwnerAbsentGrace))
			il.casDelete(ctx, key, entry.Revision(), "maybeReapLock/owner-absent")
			return true
		}
	}

	return false
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

	// Reuse detection without a proto change: on a successful spawn the
	// placement actor's PersistActivation callback consumed our lock revision
	// and upgraded the record to lock-only -> PID. But on the REUSE path (the
	// placement actor returned an already-tracked actor without spawning),
	// PersistActivation never ran, so the record is still lock-only and our
	// revision is still present. Detect that and upgrade the record ourselves
	// using the held revision, so no lock-only record survives a successful
	// activateLocal. On store failure, fall into the standard failure cleanup.
	if _, held := il.lockRevisions.Load(lockID); held {
		if rec := il.readRawRecord(ctx, ci); rec != nil && rec.PidID == "" {
			il.lockRevisions.Delete(lockID)
			storeErr := il.storeActivation(ctx, ci, lockID, revision, il.memberID,
				activationResp.Pid.Address, activationResp.Pid.Id)
			if storeErr != nil {
				il.identityLogger().Error("natskv identity: failed to upgrade reused activation record",
					slog.String("kind", ci.Kind),
					slog.String("identity", ci.Identity),
					slog.Any("error", storeErr))
				il.casDelete(ctx, kvKey(ci), revision, "activateLocal/reuse-store-failed")
				return nil
			}
		}
	}

	// Populate the local PID cache.
	il.cluster.PidCache.Set(ci.Identity, ci.Kind, activationResp.Pid)

	return activationResp.Pid
}

// readRawRecord reads the current identity record for ci without the
// completed-activation filter that getExistingActivationWithRev applies, so
// callers can distinguish a lock-only record (PidID == "") from a completed
// one. Returns nil if the key is absent or unreadable.
func (il *IdentityLookup) readRawRecord(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	entry, err := il.identities.Get(ctx, kvKey(ci))
	if err != nil {
		return nil
	}
	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return nil
	}
	return &rec
}

// checkActivationRecord is the PlacementConfig.CheckActivationRecord callback.
// It classifies the persisted identity record for a grain the local placement
// actor spawned but did not persist (remote-initiated activation) with one KV
// Get: absent/unreadable -> RecordAbsent; lock-only (no PID) -> RecordLockOnly;
// PID matches selfPID -> RecordOwn; PID differs -> RecordForeign.
func (il *IdentityLookup) checkActivationRecord(ctx context.Context, ci *cluster.ClusterIdentity, selfPID *actor.PID) cluster.RecordCheck {
	key := kvKey(ci)
	entry, err := il.identities.Get(ctx, key)
	if err != nil {
		return cluster.RecordAbsent
	}
	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return cluster.RecordAbsent
	}
	if rec.PidID == "" {
		return cluster.RecordLockOnly
	}
	if rec.PidID == selfPID.Id && rec.PidAddress == selfPID.Address {
		return cluster.RecordOwn
	}
	return cluster.RecordForeign
}

// cleanupOwnRecord is the PlacementConfig.CleanupOwnRecord callback. After the
// placement actor self-poisons a duplicate grain, it CAS-deletes a completed
// record only if that record still points at selfPID. A successor's record
// (different PID) is left untouched.
func (il *IdentityLookup) cleanupOwnRecord(ctx context.Context, ci *cluster.ClusterIdentity, selfPID *actor.PID) {
	key := kvKey(ci)
	entry, err := il.identities.Get(ctx, key)
	if err != nil {
		return
	}
	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return
	}
	// Only delete a record that points at the poisoned PID.
	if rec.PidID != selfPID.Id || rec.PidAddress != selfPID.Address {
		return
	}
	if rec.MemberID != "" {
		il.removeKeyFromMember(ctx, rec.MemberID, key)
	}
	il.casDelete(ctx, key, entry.Revision(), "cleanupOwnRecord")
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
		// The remote placement actor spawned the grain but our persist failed,
		// so the identity record is still lock-only. Ask the remote to remove
		// the grain from its tracking and poison it BEFORE we delete the lock,
		// so no live instance survives on the losing side. On timeout we
		// proceed — the remote's self-check is the backstop.
		il.requestRemoteRemoveAndPoison(member, ci, activationResp.Pid, revision)
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
	err = il.identities.Delete(ctx, key, jetstream.LastRevision(entry.Revision()))
	il.recordIdentityWriteOutcome("RemovePid/delete", true, err)
}

// Shutdown performs cleanup when the cluster is shutting down.
// It stops the placement actor first (graceful grain shutdown), then
// stops the proxy, closes the strategy manager, and removes member records.
func (il *IdentityLookup) Shutdown() {
	il.defunct.Store(true)
	// Stop the janitor goroutine (member nodes only).
	// sync.Once guards against a double-close panic if Shutdown is called more
	// than once (e.g. by a finalizer and an explicit call).
	if il.janitorStop != nil {
		il.janitorStopOnce.Do(func() { close(il.janitorStop) })
	}
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
	if il.poisonSub != nil {
		_ = il.poisonSub.Unsubscribe()
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

// casDelete removes the given key only if its current revision matches rev.
// A CAS miss (ErrKeyExists from jetstream) is a terminal no-op: the key was
// modified after our read, so we must not blind-delete. Never loops or falls
// back to unconditional delete.
//
// Purge rather than Delete: Purge is Delete plus KV-Operation: PURGE,
// Nats-Rollup: sub and a per-message TTL, so the subject collapses to a single
// marker that the server removes on its own instead of retaining it forever.
// The rollup discards nothing a reader could still want -- these buckets run at
// the KV default History of 1, so the subject already holds at most one
// message. Both LastRevision and PurgeTTL are KVDeleteOpt, so the CAS guard is
// unaffected, and the server evaluates the expected-revision header before it
// stores anything, so a rejected purge never rolls the subject up.
func (il *IdentityLookup) casDelete(ctx context.Context, key string, rev uint64, site string) {
	err := il.identities.Purge(ctx, key, il.tombstoneOpts(rev)...)
	// Control flow is unchanged: casDelete is always a terminal no-op and
	// never retries or blind-deletes. Only the logging/accounting differs --
	// the recorder classifies the error (success / benign CAS conflict / real
	// failure), resetting or advancing the fail-stop streak accordingly. A CAS
	// miss (wrong-last-sequence) is benign and Debug-logged; a genuine write
	// failure is now surfaced at Warn instead of being swallowed at Debug.
	il.recordIdentityWriteOutcome("casDelete/"+site, true, err)
}

// tombstoneOpts builds the delete options for a purge marker. PurgeTTL is
// attached ONLY when the bucket actually carries marker TTLs: Purge stamps a
// Nats-TTL header, and nats-server rejects that header on a stream with
// AllowMsgTTL: false (JSMessageTTLDisabledErr, "per-message TTL is disabled").
// On a server that fell back, an unconditional PurgeTTL would make every
// identity delete fail -- passivated identities would never be removed and the
// write-failure watchdog would fail-stop the process. That is a correctness
// outage in place of a latency optimisation, so the flag gates the option.
//
// LastRevision is always attached: the CAS guard is not negotiable, and both
// options are KVDeleteOpt, so the rollup does not defeat it.
func (il *IdentityLookup) tombstoneOpts(rev uint64) []jetstream.KVDeleteOpt {
	opts := []jetstream.KVDeleteOpt{jetstream.LastRevision(rev)}

	if il.markerTTLEnabled && il.config.TombstoneTTL > 0 {
		opts = append(opts, jetstream.PurgeTTL(il.config.TombstoneTTL))
	}

	return opts
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
		// ErrKeyExists (code 10071) means another node holds the lock: a benign
		// CAS conflict that resets the failure streak. Anything else is a real
		// write failure, surfaced at Warn and counted toward fail-stop.
		il.recordIdentityWriteOutcome("tryAcquireLock/create", false, err)
		return "", 0, false
	}
	il.recordIdentityWriteOutcome("tryAcquireLock/create", false, nil)

	return lockID, revision, true
}

// storeActivation stores a completed activation using CAS (revision-based Update),
// then tracks the key in the member's tracking record.
//
// The operation is bounded to 10s: an unresponsive KV must not block the
// caller (activateRemote / the reuse-upgrade path) indefinitely.
func (il *IdentityLookup) storeActivation(ctx context.Context, ci *cluster.ClusterIdentity, lockID string, revision uint64, memberID, pidAddress, pidID string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

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
	// Record the outcome: a wrong-last-sequence conflict (another writer won
	// the CAS) is benign and resets the streak; a genuine failure counts.
	il.recordIdentityWriteOutcome("storeActivation/update", false, err)
	if err != nil {
		return err
	}

	// Track this identity key under the member.
	il.addKeyToMember(ctx, memberID, key)
	return nil
}

// waitForActivation watches the NATS KV key for the given cluster identity
// until an activation appears (PID is set), or the WaiterWindow timeout expires.
// WaiterWindow is intentionally shorter than LockTTL: it bounds how long a
// caller waits for a lock holder to complete, so that abandoned locks are
// detected and reaped promptly on the next resolution pass.
// Note: requestRemoteActivation uses RemoteActivationTimeout for its own
// timeout, which is independent of LockTTL and WaiterWindow.
func (il *IdentityLookup) waitForActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	key := kvKey(ci)
	watchCtx, cancel := context.WithTimeout(ctx, il.config.WaiterWindow)
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

	// Loop exited: either the watch context timed out (WaiterWindow elapsed)
	// or the watcher was closed due to a NATS error. Classify for metrics.
	il.recordWaitTimeoutOutcome(ctx, key)
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
	err = il.memberTracker.Delete(ctx, memberID)
	il.recordIdentityWriteOutcome("removeMemberID/deleteMember", true, err)
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
			il.recordIdentityWriteOutcome("addKeyToMember/create", false, err)
			if err == nil {
				return
			}
			if errors.Is(err, jetstream.ErrKeyExists) {
				// Another goroutine created it first -- retry with update.
				continue
			}
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
		il.recordIdentityWriteOutcome("addKeyToMember/update", false, err)
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
		il.recordIdentityWriteOutcome("removeKeyFromMember/update", false, err)
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

// requestRemoteRemoveAndPoison asks the member hosting a remotely-spawned grain
// to remove it from tracking and poison it. It is best-effort: on any error or
// timeout it returns and the caller proceeds (the remote placement actor's
// self-check is the backstop). Bounded to 5s.
func (il *IdentityLookup) requestRemoteRemoveAndPoison(member *cluster.Member, ci *cluster.ClusterIdentity, pid *actor.PID, revision uint64) {
	nc := il.provider.nc
	if nc == nil {
		return
	}

	data, err := json.Marshal(poisonReq{
		Kind:       ci.Kind,
		Identity:   ci.Identity,
		PidID:      pid.Id,
		PidAddress: pid.Address,
		Revision:   revision,
	})
	if err != nil {
		return
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subject := poisonSubject(il.cluster.Config.Name) + "." + member.Id
	if _, err := nc.RequestWithContext(reqCtx, subject, data); err != nil {
		il.identityLogger().Warn("natskv identity: remote remove-and-poison request failed (self-check is backstop)",
			slog.String("targetMember", member.Id),
			slog.String("pid", pid.String()),
			slog.Any("error", err))
	}
}

// handleRemoveAndPoisonRequest is the NATS subscription handler for peer
// remove-and-poison requests. It validates that the target grain is genuinely
// an orphan of the caller's failed activation before forwarding a
// RemoveAndPoisonRequest to the local placement actor.
//
// Decision rule (target validation against replay/spoof): re-read the identity
// record and honor the poison only when the target is provably the loser's
// unresolved activation:
//
//   - record ABSENT: the loser's lock was already released and no successor has
//     written a record, so the tracked grain is a genuine orphan -> honor.
//   - record LOCK-ONLY (PidID == "") AND its current revision == the caller's
//     carried revision: this is still the loser's own unresolved lock (its
//     persist failed before it could upgrade the record) -> honor.
//   - anything else -> REJECT. A non-empty PidID means some activation (a
//     successor, or the loser itself) has completed and is authoritative; a
//     revision mismatch on a lock-only record means the lock was re-acquired by
//     a successor. A replayed poisonReq from an earlier failed activation
//     carries a stale revision, so it cannot match the current record and is
//     rejected, protecting the legitimate successor.
//
// This mirrors the CAS-with-revision discipline used throughout the identity
// store: the caller's held lock revision is the proof that the record it wants
// poisoned is the same record it lost on.
func (il *IdentityLookup) handleRemoveAndPoisonRequest(msg *nats.Msg) {
	var req poisonReq
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		resp, _ := json.Marshal(poisonResp{Error: "bad request"})
		_ = msg.Respond(resp)
		return
	}

	placementPID := il.placementPID.Load()
	if placementPID == nil {
		resp, _ := json.Marshal(poisonResp{Error: "no placement actor"})
		_ = msg.Respond(resp)
		return
	}

	if !il.poisonTargetIsOrphan(&req) {
		il.identityLogger().Warn("natskv identity: rejecting remove-and-poison request; target is not an orphan of the caller's failed activation (replay/spoof guard)",
			slog.String("kind", req.Kind),
			slog.String("identity", req.Identity),
			slog.String("pid", req.PidAddress+"/"+req.PidID),
			slog.Uint64("callerRevision", req.Revision))
		resp, _ := json.Marshal(poisonResp{Error: "rejected: target not orphaned"})
		_ = msg.Respond(resp)
		return
	}

	pid := actor.NewPID(req.PidAddress, req.PidID)
	future := il.cluster.ActorSystem.Root.RequestFuture(placementPID,
		&cluster.RemoveAndPoisonRequest{PID: pid}, 5*time.Second)
	if _, err := future.Result(); err != nil {
		resp, _ := json.Marshal(poisonResp{Error: err.Error()})
		_ = msg.Respond(resp)
		return
	}

	resp, _ := json.Marshal(poisonResp{Ok: true})
	_ = msg.Respond(resp)
}

// poisonTargetIsOrphan re-reads the identity record named by the request and
// reports whether the poison should be honored. See handleRemoveAndPoisonRequest
// for the decision rule. A missing kind/identity (older wire format) is treated
// as not-orphan, so an unvalidatable request is rejected rather than trusted.
func (il *IdentityLookup) poisonTargetIsOrphan(req *poisonReq) bool {
	if req.Kind == "" && req.Identity == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := kvKey(&cluster.ClusterIdentity{Kind: req.Kind, Identity: req.Identity})
	entry, err := il.identities.Get(ctx, key)
	if err != nil {
		// Record absent: the loser's lock was released and no successor has
		// written a record yet. The tracked grain is a genuine orphan.
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return true
		}
		// Any other read error: fail closed (do not poison on uncertainty).
		return false
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return false
	}

	// A completed record (PidID set) is authoritative — a successor (or the
	// loser itself) has resolved the identity. Never poison against it.
	if rec.PidID != "" {
		return false
	}

	// Lock-only record: honor only if it is still the loser's own lock, proven
	// by the revision matching the one the caller held when its persist failed.
	return entry.Revision() == req.Revision
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

	// Bound the remote round-trip with RemoteActivationTimeout, not LockTTL.
	// Member-side spawn (placement RPC) can legitimately take ~10s+, so the
	// old LockTTL (5s) bound truncated valid activations. LockTTL is untouched
	// everywhere else.
	reqCtx, cancel := context.WithTimeout(ctx, il.config.RemoteActivationTimeout)
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
