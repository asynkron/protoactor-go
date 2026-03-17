package cluster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// PlacementConfig configures the shared placement actor's behavior.
// Identity lookups provide their own persistence callbacks. If both
// PersistActivation and RemoveActivation are nil, the placement actor
// operates in memory-only mode (disthash behavior).
type PlacementConfig struct {
	// PersistActivation stores the activation after spawn. Called inside
	// ReenterAfter — the placement actor does not respond until this
	// completes (or fails after retries). Nil means no persistence (disthash).
	// A LockNotHeld sentinel error signals the lock was stolen — no retry.
	PersistActivation func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error

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
}

// GrainMeta tracks the PID and identity of a locally-spawned grain.
type GrainMeta struct {
	ID  *ClusterIdentity
	PID *actor.PID
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
}

// NewPlacementActorProps returns actor Props for spawning the shared
// placement actor. The placement actor should be spawned as a named
// actor (e.g., "$placement-activator") on each non-client member.
func NewPlacementActorProps(cluster *Cluster, config PlacementConfig) *actor.Props {
	config.defaults()
	return actor.PropsFromProducer(func() actor.Actor {
		return &placementActor{
			cluster:  cluster,
			config:   config,
			actors:   make(map[string]*GrainMeta),
			spawning: make(map[string]bool),
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
	case *ClusterTopology:
		p.onClusterTopology(ctx, msg)
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

	// 7. If PersistActivation is set, call it via ReenterAfter with retry.
	if p.config.PersistActivation != nil {
		p.persistAndRespond(ctx, msg, pid, clusterKind)
		return
	}

	// 8. No persistence — add to map immediately, respond with PID.
	delete(p.spawning, key)
	p.actors[key] = &GrainMeta{ID: msg.ClusterIdentity, PID: pid}
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

			persistCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			lastErr = p.config.PersistActivation(persistCtx, ci, pid)
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
		p.actors[key] = &GrainMeta{ID: ci, PID: pid}
		ctx.Respond(&ActivationResponse{Pid: pid})
	})
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

// pidToKey finds the identity key for a PID in the actors map.
func (p *placementActor) pidToKey(pid *actor.PID) (bool, string) {
	for k, v := range p.actors {
		if v.PID.Equal(pid) {
			return true, k
		}
	}
	return false, ""
}
