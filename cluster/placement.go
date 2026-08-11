package cluster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/awevoke/protoactor-go/actor"
)

// RecordCheck classifies the state of a stored activation record relative to
// a locally-tracked grain, as reported by PlacementConfig.CheckActivationRecord.
type RecordCheck int

const (
	// RecordOwn: a completed activation record exists and its PID matches the
	// locally-tracked PID. The activation is authoritative — disarm self-check.
	RecordOwn RecordCheck = iota
	// RecordForeign: a completed activation record exists but points at a
	// different PID (another member won the identity). The local actor is a
	// duplicate and must be poisoned.
	RecordForeign
	// RecordLockOnly: the record exists but is lock-only (no PID persisted yet).
	// A resolution is still in flight — defer the decision (never poison).
	RecordLockOnly
	// RecordAbsent: no record exists for the identity at all.
	RecordAbsent
)

func (r RecordCheck) String() string {
	switch r {
	case RecordOwn:
		return "own"
	case RecordForeign:
		return "foreign"
	case RecordLockOnly:
		return "lock-only"
	case RecordAbsent:
		return "absent"
	default:
		return "unknown"
	}
}

// PlacementConfig configures the shared placement actor's behavior.
// Identity lookups provide their own persistence callbacks. If both
// PersistActivation and RemoveActivation are nil, the placement actor
// operates in memory-only mode (disthash behavior).
type PlacementConfig struct {
	// PersistActivation stores the activation after spawn. Called inside
	// ReenterAfter — the placement actor does not respond until this
	// completes (or fails after retries). Nil means no persistence (disthash).
	// A LockNotHeld sentinel error signals the lock was stolen — no retry.
	// The requestID parameter is the ActivationRequest.RequestId, which
	// identity lookups set to the spawn lock ID for CAS verification.
	PersistActivation func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID, requestID string) error

	// RemoveActivation cleans storage when an actor terminates. Called
	// from the Terminated handler. Errors are logged but do not block
	// cleanup of the local tracking map. Nil means no cleanup needed.
	RemoveActivation func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error

	// RebalanceOnTopology, when set, is called on ClusterTopology events.
	// It receives the new topology and the current local actors map
	// (keyed by ClusterIdentity.AsKey()). It returns the set of identity
	// keys that should be rebalanced (poisoned and re-activated elsewhere).
	// Nil means no rebalancing (storage-backed providers).
	RebalanceOnTopology func(topology *ClusterTopology, actors map[string]*GrainMeta) []string

	// PersistenceRetries is how many times to retry PersistActivation
	// before giving up and poisoning the actor. Default: 3.
	PersistenceRetries int

	// PersistenceRetryDelay is the base delay between persistence retries.
	// Default: 50ms.
	PersistenceRetryDelay time.Duration

	// ShutdownTimeout is the maximum time to wait for all local grains to
	// stop during the Stopping handler. If exceeded, remaining grains are
	// force-stopped. Default: 30s.
	ShutdownTimeout time.Duration

	// CheckActivationRecord, when non-nil, enables the post-spawn self-check.
	// After spawning a grain in response to a locally-initiated activation
	// (empty ActivationRequest.RequestId means remote-initiated and is
	// exempt), the placement actor arms a self-check timer. When it fires, it
	// calls CheckActivationRecord to compare the persisted identity record
	// against the locally-tracked PID and acts on the result (disarm, poison,
	// or defer). Nil (every provider except natskv) means no self-check timer
	// is ever armed and the placement actor behaves bit-for-bit as before.
	CheckActivationRecord func(ctx context.Context, ci *ClusterIdentity, selfPID *actor.PID) RecordCheck

	// CleanupOwnRecord, when non-nil, is called after the placement actor
	// self-poisons a duplicate grain. It CAS-deletes any persisted record that
	// still points at the poisoned PID, leaving records that point at a
	// different (successor) PID untouched. Only meaningful together with
	// CheckActivationRecord.
	CleanupOwnRecord func(ctx context.Context, ci *ClusterIdentity, selfPID *actor.PID)

	// SelfCheckDelay is how long after a spawn the self-check timer fires and
	// how long each re-arm defers. It should be at least twice the caller's
	// persistence bound so a legitimately-in-flight persist has time to land.
	// Default: 20s. Only used when CheckActivationRecord is non-nil.
	SelfCheckDelay time.Duration

	// SelfCheckHardReapAge bounds how long a self-check may keep re-arming on a
	// RecordAbsent observation before poisoning the orphaned grain. It mirrors
	// the identity store's HardReapAge so a grain whose record never persists
	// is eventually reclaimed. Default: 60s. Only used when CheckActivationRecord
	// is non-nil.
	SelfCheckHardReapAge time.Duration
}

func (c *PlacementConfig) defaults() {
	if c.PersistenceRetries <= 0 {
		c.PersistenceRetries = 3
	}
	if c.PersistenceRetryDelay <= 0 {
		c.PersistenceRetryDelay = 50 * time.Millisecond
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 30 * time.Second
	}
	if c.SelfCheckDelay <= 0 {
		c.SelfCheckDelay = 20 * time.Second
	}
	if c.SelfCheckHardReapAge <= 0 {
		c.SelfCheckHardReapAge = 60 * time.Second
	}
}

// GrainMeta tracks the PID and identity of a locally-spawned grain.
type GrainMeta struct {
	ID          *ClusterIdentity
	PID         *actor.PID
	ActivatedAt time.Time

	// selfCheckArmed indicates an outstanding self-check timer is pending for
	// this grain (set only when CheckActivationRecord is configured and the
	// activation was locally-initiated).
	selfCheckArmed bool
	// firstAbsent records the first time the self-check observed RecordAbsent
	// for this grain, used to bound re-arming by SelfCheckHardReapAge. Zero
	// means no absent observation yet.
	firstAbsent time.Time
}

// selfCheck is the message the placement actor sends to itself (after a delay)
// to trigger a post-spawn activation-record self-check for a tracked grain.
type selfCheck struct {
	key string
	pid *actor.PID
}

// RemoveAndPoisonRequest asks the placement actor to remove a tracked grain
// from its local map (so a straggler persist cannot re-record it), poison it,
// and reply with RemoveAndPoisonAck once the poison completes. Sent by an
// identity lookup's store-failure path to close the duplicate-instance window
// on remote placement.
type RemoveAndPoisonRequest struct {
	PID *actor.PID
}

// RemoveAndPoisonAck acknowledges a RemoveAndPoisonRequest.
type RemoveAndPoisonAck struct{}

// ListGrainsRequest asks the placement actor to return all active grains.
type ListGrainsRequest struct{}

// ListGrainsResponse contains all active grains on this placement actor.
type ListGrainsResponse struct {
	Grains []*GrainInfo
}

// placementActor is the shared default placement actor. It handles
// ActivationRequests, tracks local grains, and supports optional
// persistence callbacks and topology rebalancing.
type placementActor struct {
	cluster  *Cluster
	config   PlacementConfig
	actors   map[string]*GrainMeta // key: ClusterIdentity.AsKey()
	spawning map[string]bool       // in-flight spawns (no mutex — single-threaded actor)
	stopping bool                  // set in Stopping handler

	// persisting tracks in-flight PersistActivation goroutines by grain PID
	// (via pidKey). Each carries an atomic abort flag the persist goroutine
	// checks before every KV attempt so a poison decided on the mailbox
	// (removeAndPoison / persist-failure) can prevent a straggler write from
	// recording an actor that is being torn down. Mutated only on the mailbox
	// goroutine; the abort flag itself is atomic for cross-goroutine reads.
	persisting map[string]*int32
}

// NewPlacementActorProps returns actor Props for spawning the shared
// placement actor. The placement actor should be spawned as a named
// actor (e.g., "$placement-activator") on each non-client member.
func NewPlacementActorProps(cluster *Cluster, config PlacementConfig) *actor.Props {
	config.defaults()
	return actor.PropsFromProducer(func() actor.Actor {
		return &placementActor{
			cluster:    cluster,
			config:     config,
			actors:     make(map[string]*GrainMeta),
			spawning:   make(map[string]bool),
			persisting: make(map[string]*int32),
		}
	})
}

func (p *placementActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		ctx.Logger().Info("Placement actor started")
	case *actor.Stopping:
		ctx.Logger().Info("Placement actor stopping")
		p.onStopping(ctx)
	case *actor.Stopped:
		ctx.Logger().Info("Placement actor stopped")
	case *actor.Terminated:
		p.onTerminated(ctx, msg)
	case *ActivationRequest:
		p.onActivationRequest(ctx, msg)
	case *selfCheck:
		p.onSelfCheck(ctx, msg)
	case *RemoveAndPoisonRequest:
		p.onRemoveAndPoison(ctx, msg)
	case *ClusterTopology:
		p.onClusterTopology(ctx, msg)
	case *ListGrainsRequest:
		p.onListGrains(ctx)
	case *PeekRequest:
		p.onPeekRequest(ctx, msg)
	default:
		ctx.Logger().Error("Placement actor received unknown message",
			slog.Any("message", msg), slog.Any("sender", ctx.Sender()))
	}
}

// onStopping poisons all locally tracked grains with DeactivationReasonShutdown,
// waiting up to ShutdownTimeout for graceful termination.
func (p *placementActor) onStopping(ctx actor.Context) {
	p.stopping = true

	if len(p.actors) == 0 {
		return
	}

	futures := make(map[string]actor.Future, len(p.actors))
	for key, meta := range p.actors {
		p.cluster.SetDeactivationReason(meta.PID, DeactivationReasonShutdown)
		futures[key] = ctx.PoisonFuture(meta.PID)
	}

	// Wait for all poisons to complete, up to ShutdownTimeout.
	// Drain futures via a goroutine so we can race against the deadline.
	// Capture logger and root before goroutine — ctx methods must not be
	// called from outside the actor's mailbox goroutine.
	log := ctx.Logger()
	root := p.cluster.ActorSystem.Root
	done := make(chan struct{})
	go func() {
		for key, future := range futures {
			err := future.Wait()
			if err != nil {
				log.Error("Failed to poison actor during shutdown",
					slog.String("identity", key), slog.Any("error", err))
			}
		}
		close(done)
	}()

	select {
	case <-done:
		// All grains stopped gracefully.
	case <-time.After(p.config.ShutdownTimeout):
		log.Warn("Shutdown timeout reached, force-stopping remaining grains")
		for _, meta := range p.actors {
			root.Stop(meta.PID)
		}
	}
}

// onTerminated handles a watched grain stopping. It cleans up the local
// map and calls RemoveActivation if configured.
func (p *placementActor) onTerminated(ctx actor.Context, msg *actor.Terminated) {
	found, key := p.pidToKey(msg.Who)
	if !found {
		ctx.Logger().Warn("Terminated actor not found in placement map",
			slog.Any("pid", msg.Who))
		return
	}

	meta := p.actors[key]

	// Update kind counter.
	if clusterKind := p.cluster.GetClusterKind(meta.ID.Kind); clusterKind != nil {
		clusterKind.Dec()
	}

	// Clean up local map first.
	delete(p.actors, key)

	// Update metrics gauge after cleanup.
	if p.cluster.MetricsEnabled() {
		p.cluster.Metrics().VirtualActorsCount.Set(p.cluster.VirtualActorCount())
	}

	// Call RemoveActivation callback if set. Errors are logged, not fatal.
	if p.config.RemoveActivation != nil {
		rmCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.config.RemoveActivation(rmCtx, meta.ID, meta.PID); err != nil {
			ctx.Logger().Error("RemoveActivation callback failed",
				slog.String("identity", key),
				slog.Any("pid", meta.PID),
				slog.Any("error", err))
		}
	}
}

// onActivationRequest is the core spawn flow.
func (p *placementActor) onActivationRequest(ctx actor.Context, msg *ActivationRequest) {
	key := msg.ClusterIdentity.AsKey()

	// 1. If stopping, respond Failed.
	if p.stopping {
		ctx.Respond(&ActivationResponse{Failed: true})
		return
	}

	// 2. If identity already tracked locally, respond with existing PID.
	if meta, found := p.actors[key]; found {
		ctx.Respond(&ActivationResponse{Pid: meta.PID})
		return
	}

	// 3. If identity is in-flight (spawning), respond Failed (duplicate).
	if p.spawning[key] {
		ctx.Logger().Warn("Duplicate activation request ignored; spawn already in progress",
			slog.String("identity", msg.ClusterIdentity.Identity),
			slog.String("kind", msg.ClusterIdentity.Kind))
		ctx.Respond(&ActivationResponse{Failed: true})
		return
	}

	// 4. Get ClusterKind from cluster; respond Failed if unknown.
	clusterKind := p.cluster.GetClusterKind(msg.ClusterIdentity.Kind)
	if clusterKind == nil {
		ctx.Logger().Error("Unknown cluster kind",
			slog.String("kind", msg.ClusterIdentity.Kind))
		ctx.Respond(&ActivationResponse{Failed: true})
		return
	}

	// 5. CanSpawnIdentity check (async via ReenterAfter).
	if clusterKind.CanSpawnIdentity != nil {
		p.spawning[key] = true
		p.spawnWithVerification(ctx, msg, clusterKind)
		return
	}

	// 6. No CanSpawnIdentity — spawn directly.
	p.spawnActor(ctx, msg, clusterKind)
}

// spawnWithVerification runs CanSpawnIdentity in a goroutine and uses
// ReenterAfter to continue on the actor's mailbox.
func (p *placementActor) spawnWithVerification(ctx actor.Context, msg *ActivationRequest, kind *ActivatedKind) {
	key := msg.ClusterIdentity.AsKey()
	future := actor.NewFuture(ctx.ActorSystem(), 10*time.Second)

	go func() {
		ok, err := kind.CanSpawnIdentity(context.Background(), msg.ClusterIdentity.Identity)
		if err != nil {
			p.cluster.ActorSystem.Root.Send(future.PID(), &verificationResult{ok: false, err: err})
			return
		}
		p.cluster.ActorSystem.Root.Send(future.PID(), &verificationResult{ok: ok, err: nil})
	}()

	ctx.ReenterAfter(future, func(res any, err error) {
		defer func() {
			delete(p.spawning, key)
		}()

		if err != nil {
			ctx.Logger().Error("CanSpawnIdentity future error",
				slog.String("identity", msg.ClusterIdentity.Identity),
				slog.Any("error", err))
			ctx.Respond(&ActivationResponse{Failed: true})
			return
		}

		vr, ok := res.(*verificationResult)
		if !ok {
			ctx.Respond(&ActivationResponse{Failed: true})
			return
		}

		if vr.err != nil {
			ctx.Logger().Error("CanSpawnIdentity returned error",
				slog.String("identity", msg.ClusterIdentity.Identity),
				slog.Any("error", vr.err))
			ctx.Respond(&ActivationResponse{Failed: true})
			return
		}

		if !vr.ok {
			ctx.Respond(&ActivationResponse{InvalidIdentity: true})
			return
		}

		// Verification passed — proceed with spawn.
		p.spawnActor(ctx, msg, kind)
	})
}

// verificationResult carries the CanSpawnIdentity outcome through a future.
type verificationResult struct {
	ok  bool
	err error
}

// persistenceResult carries the PersistActivation outcome through a future.
type persistenceResult struct {
	err error
}

// spawnActor performs the actual spawn, watch, and optional persistence.
func (p *placementActor) spawnActor(ctx actor.Context, msg *ActivationRequest, clusterKind *ActivatedKind) {
	key := msg.ClusterIdentity.AsKey()

	// Mark as spawning (may already be set from spawnWithVerification).
	p.spawning[key] = true

	props := WithClusterIdentity(clusterKind.Props, msg.ClusterIdentity)

	// Spawn as a top-level actor (not a child of the placement actor) for
	// backward compatibility with existing PID formats in identity stores.
	// Use Root.SpawnNamed + explicit Watch instead of ctx.SpawnNamed (which
	// would make grains children of the placement actor, changing PID paths
	// and supervision semantics).
	pid, spawnErr := p.cluster.ActorSystem.Root.SpawnNamed(props, msg.ClusterIdentity.Kind+"/"+msg.ClusterIdentity.Identity)
	if spawnErr != nil {
		delete(p.spawning, key)
		ctx.Logger().Error("Failed to spawn actor",
			slog.String("identity", msg.ClusterIdentity.Identity),
			slog.String("kind", msg.ClusterIdentity.Kind),
			slog.Any("error", spawnErr))
		ctx.Respond(&ActivationResponse{Failed: true})
		return
	}

	// Watch the spawned actor for Terminated messages.
	ctx.Watch(pid)

	clusterKind.Inc()

	if p.cluster.MetricsEnabled() {
		p.cluster.Metrics().VirtualActorsCount.Set(p.cluster.VirtualActorCount())
	}

	// 7. If PersistActivation is set, call it via ReenterAfter with retry.
	if p.config.PersistActivation != nil {
		p.persistAndRespond(ctx, msg, pid, clusterKind)
		return
	}

	// 8. No persistence — add to map immediately, respond with PID.
	delete(p.spawning, key)
	p.actors[key] = &GrainMeta{ID: msg.ClusterIdentity, PID: pid, ActivatedAt: time.Now()}
	ctx.Respond(&ActivationResponse{Pid: pid})
}

// persistAndRespond handles async persistence via ReenterAfter with retry logic.
func (p *placementActor) persistAndRespond(ctx actor.Context, msg *ActivationRequest, pid *actor.PID, clusterKind *ActivatedKind) {
	key := msg.ClusterIdentity.AsKey()
	ci := msg.ClusterIdentity
	retries := p.config.PersistenceRetries
	delay := p.config.PersistenceRetryDelay

	future := actor.NewFuture(ctx.ActorSystem(), 30*time.Second)

	root := p.cluster.ActorSystem.Root
	// Capture logger before goroutine — ctx methods must not be called
	// from outside the actor's mailbox goroutine.
	log := ctx.Logger()

	// Register an abort flag for this in-flight persist so a poison decided on
	// the mailbox goroutine (removeAndPoison) can prevent a straggler write
	// from recording an actor that is being torn down. Only meaningful when the
	// self-check is enabled; disthash/storage leave the flag unread. Registered
	// on the mailbox goroutine before the persist goroutine starts.
	var abort int32
	pk := pidKey(pid)
	p.persisting[pk] = &abort

	go func() {
		defer func() {
			if r := recover(); r != nil {
				root.Send(future.PID(), &persistenceResult{
					err: fmt.Errorf("PersistActivation panic: %v", r),
				})
			}
		}()

		var lastErr error
		for attempt := 0; attempt <= retries; attempt++ {
			if attempt > 0 {
				time.Sleep(delay)
			}

			// Skip the KV update if this persist has been aborted (the grain
			// was poisoned/untracked on the mailbox goroutine while we slept).
			if atomic.LoadInt32(&abort) != 0 {
				root.Send(future.PID(), &persistenceResult{err: ErrLockNotHeld})
				return
			}

			persistCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			lastErr = p.config.PersistActivation(persistCtx, ci, pid, msg.RequestId)
			cancel()

			if lastErr == nil {
				root.Send(future.PID(), &persistenceResult{err: nil})
				return
			}

			// LockNotHeld — no retry, fail immediately.
			if errors.Is(lastErr, ErrLockNotHeld) {
				root.Send(future.PID(), &persistenceResult{err: lastErr})
				return
			}

			// Other error — retry.
			log.Warn("PersistActivation failed, retrying",
				slog.String("identity", ci.Identity),
				slog.String("kind", ci.Kind),
				slog.Int("attempt", attempt+1),
				slog.Int("maxRetries", retries),
				slog.Any("error", lastErr))
		}

		// All retries exhausted.
		root.Send(future.PID(), &persistenceResult{err: lastErr})
	}()

	ctx.ReenterAfter(future, func(res any, err error) {
		defer func() {
			delete(p.spawning, key)
			delete(p.persisting, pk)
		}()

		// Helper to clean up on persistence failure: unwatch (to avoid
		// spurious "Terminated not found" warnings), poison, decrement.
		failAndPoison := func(reason string, e error) {
			ctx.Logger().Error(reason,
				slog.String("identity", ci.Identity),
				slog.Any("error", e))
			ctx.Unwatch(pid)
			ctx.Poison(pid)
			clusterKind.Dec()
			ctx.Respond(&ActivationResponse{Failed: true})
		}

		// Future-level error (timeout, etc.)
		if err != nil {
			failAndPoison("Persistence future error", err)
			return
		}

		pr, ok := res.(*persistenceResult)
		if !ok {
			failAndPoison("Unexpected persistence result type", fmt.Errorf("got %T", res))
			return
		}

		if pr.err != nil {
			failAndPoison("PersistActivation failed after retries", pr.err)
			return
		}

		// Persistence succeeded — add to map, respond with PID.
		meta := &GrainMeta{ID: ci, PID: pid, ActivatedAt: time.Now()}
		p.actors[key] = meta
		ctx.Respond(&ActivationResponse{Pid: pid})

		// Post-spawn self-check: only for remote-initiated activations
		// (empty RequestId), where this placement actor spawned the grain but
		// did NOT persist the record — the remote caller persists after
		// receiving the PID. If that caller dies or a foreign record wins, the
		// grain would otherwise be an orphan/duplicate. The self-check closes
		// that window. Local-initiated activations (non-empty RequestId)
		// persist inline via the callback before responding, so the record is
		// already authoritative and no self-check is needed.
		if p.config.CheckActivationRecord != nil && msg.RequestId == "" {
			p.armSelfCheck(ctx, key, meta)
		}
	})
}

// armSelfCheck schedules a self-check message to fire after SelfCheckDelay.
// It runs on the actor's mailbox goroutine (it only reads config and mutates
// meta), and dispatches the delayed send via a goroutine that messages back
// into this actor's mailbox — mirroring the future-based re-entry pattern used
// by spawnWithVerification/persistAndRespond, keeping all state mutations on
// the mailbox goroutine.
func (p *placementActor) armSelfCheck(ctx actor.Context, key string, meta *GrainMeta) {
	meta.selfCheckArmed = true
	self := ctx.Self()
	root := p.cluster.ActorSystem.Root
	delay := p.config.SelfCheckDelay
	pid := meta.PID
	go func() {
		time.Sleep(delay)
		root.Send(self, &selfCheck{key: key, pid: pid})
	}()
}

// onSelfCheck runs a post-spawn activation-record self-check for a tracked
// grain. It compares the persisted record (via CheckActivationRecord) against
// the tracked PID and acts on the result. It NEVER poisons on a lock-only
// observation (a resolution is still in flight); instead it re-arms.
func (p *placementActor) onSelfCheck(ctx actor.Context, msg *selfCheck) {
	if p.config.CheckActivationRecord == nil {
		return
	}

	meta, found := p.actors[msg.key]
	if !found {
		// Grain already gone (terminated / poisoned). Nothing to check.
		return
	}
	// Only act if this timer belongs to the currently-tracked PID. A stale
	// timer from a prior activation of the same identity is ignored.
	if !meta.PID.Equal(msg.pid) {
		return
	}
	if !meta.selfCheckArmed {
		return
	}

	checkCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	result := p.config.CheckActivationRecord(checkCtx, meta.ID, meta.PID)
	cancel()

	switch result {
	case RecordOwn:
		// The persisted record points at us — authoritative. Disarm.
		meta.selfCheckArmed = false
		meta.firstAbsent = time.Time{}

	case RecordForeign:
		// A different member won the identity. We are a duplicate — remove
		// from tracking first (so a straggler persist cannot re-record us),
		// poison, then clean any record still pointing at our PID.
		ctx.Logger().Warn("Placement self-check found foreign record; poisoning duplicate grain",
			slog.String("identity", meta.ID.Identity),
			slog.String("kind", meta.ID.Kind),
			slog.Any("pid", meta.PID))
		p.removeAndPoisonGrain(ctx, msg.key, meta)
		p.cleanupOwnRecordAsync(meta.ID, meta.PID)

	case RecordLockOnly:
		// A resolution is still in flight (possibly a successor reusing this
		// actor and about to persist). Defer — re-arm, never poison.
		p.armSelfCheck(ctx, msg.key, meta)

	case RecordAbsent:
		// No record at all. This may be a transient gap before the caller's
		// persist lands, or an orphan whose record will never appear. Bound
		// re-arming by SelfCheckHardReapAge measured from the first absent
		// observation.
		now := time.Now()
		if meta.firstAbsent.IsZero() {
			meta.firstAbsent = now
		}
		if now.Sub(meta.firstAbsent) > p.config.SelfCheckHardReapAge {
			ctx.Logger().Warn("Placement self-check found absent record beyond hard-reap age; poisoning orphan grain",
				slog.String("identity", meta.ID.Identity),
				slog.String("kind", meta.ID.Kind),
				slog.Any("pid", meta.PID),
				slog.Duration("absentFor", now.Sub(meta.firstAbsent)))
			p.removeAndPoisonGrain(ctx, msg.key, meta)
			p.cleanupOwnRecordAsync(meta.ID, meta.PID)
			return
		}
		p.armSelfCheck(ctx, msg.key, meta)
	}
}

// onRemoveAndPoison removes a tracked grain from the local map FIRST (so a
// straggler persist goroutine skips its KV update), poisons it, waits for the
// poison to complete, then acknowledges the sender.
func (p *placementActor) onRemoveAndPoison(ctx actor.Context, msg *RemoveAndPoisonRequest) {
	// Abort any in-flight persist for this PID so a straggler write cannot
	// record the actor we are tearing down.
	p.abortPersist(msg.PID)

	found, key := p.pidToKey(msg.PID)
	if !found {
		// Not tracked (already terminated, or the persist never completed).
		// The abort above still guards an in-flight persist. Ack so the caller
		// proceeds — the self-check is the backstop for any residual record.
		ctx.Respond(&RemoveAndPoisonAck{})
		return
	}
	meta := p.actors[key]

	// Remove from tracking FIRST so the persist goroutine's skip-if-untracked
	// guard fires and cannot re-record this actor.
	delete(p.actors, key)
	if clusterKind := p.cluster.GetClusterKind(meta.ID.Kind); clusterKind != nil {
		clusterKind.Dec()
	}
	if p.cluster.MetricsEnabled() {
		p.cluster.Metrics().VirtualActorsCount.Set(p.cluster.VirtualActorCount())
	}

	p.cluster.SetDeactivationReason(meta.PID, DeactivationReasonShutdown)
	// Unwatch so we do not process a Terminated for an already-removed grain.
	ctx.Unwatch(meta.PID)
	if err := ctx.PoisonFuture(meta.PID).Wait(); err != nil {
		ctx.Logger().Error("removeAndPoison: poison failed",
			slog.String("identity", meta.ID.Identity),
			slog.Any("pid", meta.PID),
			slog.Any("error", err))
	}
	ctx.Respond(&RemoveAndPoisonAck{})
}

// removeAndPoisonGrain removes a grain from local tracking (kind counter and
// metrics updated), then poisons it. Removal happens BEFORE poisoning so a
// straggler persist goroutine's skip-if-untracked guard fires. Used by the
// self-check paths; unlike onRemoveAndPoison it does not wait or reply.
func (p *placementActor) removeAndPoisonGrain(ctx actor.Context, key string, meta *GrainMeta) {
	p.abortPersist(meta.PID)
	delete(p.actors, key)
	if clusterKind := p.cluster.GetClusterKind(meta.ID.Kind); clusterKind != nil {
		clusterKind.Dec()
	}
	if p.cluster.MetricsEnabled() {
		p.cluster.Metrics().VirtualActorsCount.Set(p.cluster.VirtualActorCount())
	}
	p.cluster.SetDeactivationReason(meta.PID, DeactivationReasonShutdown)
	ctx.Unwatch(meta.PID)
	ctx.Poison(meta.PID)
}

// abortPersist signals any in-flight persist goroutine for the given PID to
// skip its next KV update. Called on the mailbox goroutine.
func (p *placementActor) abortPersist(pid *actor.PID) {
	if flag, ok := p.persisting[pidKey(pid)]; ok {
		atomic.StoreInt32(flag, 1)
	}
}

// cleanupOwnRecordAsync invokes CleanupOwnRecord (if configured) off the
// mailbox goroutine so a slow KV round-trip does not block message processing.
func (p *placementActor) cleanupOwnRecordAsync(ci *ClusterIdentity, pid *actor.PID) {
	if p.config.CleanupOwnRecord == nil {
		return
	}
	go func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		p.config.CleanupOwnRecord(cleanupCtx, ci, pid)
	}()
}

// onClusterTopology handles topology changes by calling RebalanceOnTopology
// if configured.
func (p *placementActor) onClusterTopology(ctx actor.Context, msg *ClusterTopology) {
	if p.config.RebalanceOnTopology == nil {
		return
	}

	keysToRebalance := p.config.RebalanceOnTopology(msg, p.actors)
	for _, key := range keysToRebalance {
		meta, found := p.actors[key]
		if !found {
			continue
		}
		p.cluster.SetDeactivationReason(meta.PID, DeactivationReasonTopologyChange)
		ctx.Poison(meta.PID)
	}
}

// onListGrains returns all locally tracked grains.
func (p *placementActor) onListGrains(ctx actor.Context) {
	grains := make([]*GrainInfo, 0, len(p.actors))
	memberID := p.cluster.ActorSystem.ID
	for _, meta := range p.actors {
		grains = append(grains, &GrainInfo{
			Identity:    meta.ID.Identity,
			Kind:        meta.ID.Kind,
			PID:         meta.PID,
			MemberID:    memberID,
			ActivatedAt: meta.ActivatedAt,
		})
	}
	ctx.Respond(&ListGrainsResponse{Grains: grains})
}

// onPeekRequest checks whether a specific identity is active locally
// without spawning. This is a read-only, side-effect-free operation.
func (p *placementActor) onPeekRequest(ctx actor.Context, msg *PeekRequest) {
	if p.stopping {
		ctx.Respond(&PeekResponse{Found: false})
		return
	}

	key := msg.ClusterIdentity.AsKey()
	if meta, found := p.actors[key]; found {
		ctx.Respond(&PeekResponse{Found: true, Pid: meta.PID})
		return
	}

	ctx.Respond(&PeekResponse{Found: false})
}

// pidToKey finds the identity key for a PID in the actors map.
func (p *placementActor) pidToKey(pid *actor.PID) (bool, string) {
	for k, v := range p.actors {
		if v.PID.Equal(pid) {
			return true, k
		}
	}
	return false, ""
}
