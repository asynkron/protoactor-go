package storage

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// inflight tracks an in-progress Get() call so that concurrent callers
// for the same identity can coalesce on a single result.
type inflight struct {
	done chan struct{}
	pid  *actor.PID
}

// IdentityStorageLookup adapts a cluster.StorageLookup into a
// cluster.IdentityLookup. It uses the storage backend to manage identity
// activations and spawn locks, and routes activation requests through the
// shared placement actor for proper lifecycle management.
type IdentityStorageLookup struct {
	storage  cluster.StorageLookup
	cluster  *cluster.Cluster
	memberID string
	isClient bool

	// placementPID and proxyPID are the local placement and proxy actors
	// spawned during Setup(). They are nil for client-only nodes. Accessed
	// atomically so Shutdown() can swap them to nil while Get() readers may
	// still be in flight.
	placementPID atomic.Pointer[actor.PID]
	proxyPID     atomic.Pointer[actor.PID]

	// strategyManager selects which member should host a given identity.
	// Accessed atomically because the topology event handler and Shutdown()
	// may swap it concurrently with resolveIdentity() readers.
	strategyManager atomic.Pointer[cluster.StrategyManager]

	// defunct is set during Shutdown() and checked on entry to Get()/Peek()
	// so post-shutdown reconciler ticks short-circuit cleanly.
	defunct atomic.Bool

	// inflightMu protects the inflights map for concurrent Get() coalescing.
	inflightMu sync.Mutex
	inflights  map[string]*inflight

	// pendingLocksMu protects the pendingLocks map that bridges
	// requestID -> *SpawnLock for the PersistActivation callback.
	pendingLocksMu sync.Mutex
	pendingLocks   map[string]*cluster.SpawnLock
}

// Compile-time check that IdentityStorageLookup implements cluster.IdentityLookup.
var _ cluster.IdentityLookup = (*IdentityStorageLookup)(nil)

// New creates a new IdentityStorageLookup that delegates storage operations
// to the provided StorageLookup backend.
func New(storage cluster.StorageLookup) *IdentityStorageLookup {
	return &IdentityStorageLookup{
		storage:      storage,
		inflights:    make(map[string]*inflight),
		pendingLocks: make(map[string]*cluster.SpawnLock),
	}
}

// logger returns the cluster logger if available, otherwise the default logger.
func (l *IdentityStorageLookup) logger() *slog.Logger {
	if l.cluster != nil {
		return l.cluster.Logger()
	}
	return slog.Default()
}

// Get resolves a cluster identity to an actor PID.
//
// The resolution protocol:
//  1. Check inflight coalescing — if another goroutine is already resolving
//     the same identity, wait for its result.
//  2. Check for an existing activation in the storage backend. If the owning
//     member has left the cluster (stale), clean up and proceed to spawn.
//  3. Select a target member via the strategy manager. If this node is
//     selected, acquire a storage lock, route through the local placement
//     actor, and persist the activation. If another node is selected, send
//     the activation request to that node's proxy actor.
//
// Returns nil if the identity could not be resolved (caller should retry).
func (l *IdentityStorageLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	if l.defunct.Load() {
		l.logger().Warn("IdentityStorageLookup: cannot Get, defunct, shutdown was called")
		return nil
	}

	// Check PID cache first (avoids storage round-trip for cached PIDs).
	if pid, ok := l.cluster.PidCache.Get(ci.Identity, ci.Kind); ok {
		return pid
	}

	key := ci.AsKey()

	// Step 0: Inflight coalescing — if another goroutine is resolving this
	// identity, wait for its result instead of issuing a duplicate request.
	l.inflightMu.Lock()
	if inf, ok := l.inflights[key]; ok {
		l.inflightMu.Unlock()
		<-inf.done
		return inf.pid
	}
	inf := &inflight{done: make(chan struct{})}
	l.inflights[key] = inf
	l.inflightMu.Unlock()

	// When we're done, publish the result and remove from inflights.
	defer func() {
		close(inf.done)
		l.inflightMu.Lock()
		delete(l.inflights, key)
		l.inflightMu.Unlock()
	}()

	pid := l.resolveIdentity(ci)
	inf.pid = pid
	return pid
}

// resolveIdentity performs the actual identity resolution logic.
func (l *IdentityStorageLookup) resolveIdentity(ci *cluster.ClusterIdentity) *actor.PID {
	ctx, span := otel.Tracer("protoactor/identity").Start(context.Background(), "identity.lookup",
		trace.WithAttributes(
			attribute.String("kind", ci.Kind),
			attribute.String("identity", ci.Identity),
			attribute.String("provider", "storage"),
		),
	)
	defer span.End()

	// Step 1: Check for an existing activation.
	existing := l.storage.TryGetExistingActivation(ci)
	if existing != nil {
		// Validate that the owning member is still alive.
		if cluster.ValidateActivationMember(l.cluster.MemberList, existing.MemberID) {
			return l.pidFromStored(existing)
		}
		// Stale activation from a dead member — clean it up.
		l.logger().Info("IdentityStorageLookup: cleaning stale activation",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.String("staleMember", existing.MemberID))
		l.storage.RemoveActivation(&cluster.SpawnLock{ClusterIdentity: ci})
	}

	// Clients cannot spawn actors, so they must wait for a member to do it.
	if l.isClient {
		activation := l.storage.WaitForActivation(ci)
		if activation != nil {
			return l.pidFromStored(activation)
		}
		return nil
	}

	// Step 2: Acquire the spawn lock before selecting a target. The lock
	// is a distributed coordination mechanism — only one node can hold it.
	_, lockSpan := otel.Tracer("protoactor/identity").Start(ctx, "identity.lock_acquire",
		trace.WithAttributes(
			attribute.String("kind", ci.Kind),
			attribute.String("identity", ci.Identity),
		),
	)
	lock := l.storage.TryAcquireLock(ci)
	lockSpan.End()

	if lock == nil {
		// Another node is spawning this actor. Wait for it.
		activation := l.storage.WaitForActivation(ci)
		if activation != nil {
			return l.pidFromStored(activation)
		}
		return nil
	}

	// Step 3: Select target member via strategy manager.
	senderAddress := l.cluster.ActorSystem.Address()
	strMgr := l.strategyManager.Load()
	if strMgr == nil {
		l.logger().Warn("IdentityStorageLookup: strategy manager is nil (cluster shutting down?)",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		l.storage.RemoveLock(*lock)
		return nil
	}
	targetMember := strMgr.GetActivator(ci, senderAddress)
	if targetMember == nil {
		l.logger().Warn("IdentityStorageLookup: no available member for activation",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		l.storage.RemoveLock(*lock)
		return nil
	}

	// Step 4: Route activation request.
	if targetMember.Id == l.memberID {
		return l.activateLocal(ctx, ci, lock)
	}

	// Another node is the target — send request to its proxy. The remote
	// placement actor spawns but does NOT persist (empty requestID → no-op
	// in PersistActivation). This node persists after getting the PID back.
	return l.activateRemote(ctx, ci, lock, targetMember)
}

// activateLocal sends an ActivationRequest to the local placement actor.
// The lock is stored in pendingLocks so the PersistActivation callback can
// find it via requestID. The placement actor spawns, persists, and responds.
func (l *IdentityStorageLookup) activateLocal(ctx context.Context, ci *cluster.ClusterIdentity, lock *cluster.SpawnLock) *actor.PID {
	// Store the lock so PersistActivation callback can find it.
	l.pendingLocksMu.Lock()
	l.pendingLocks[lock.LockID] = lock
	l.pendingLocksMu.Unlock()

	defer func() {
		l.pendingLocksMu.Lock()
		delete(l.pendingLocks, lock.LockID)
		l.pendingLocksMu.Unlock()
	}()

	// Send ActivationRequest to the local placement actor.
	req := &cluster.ActivationRequest{
		ClusterIdentity: ci,
		RequestId:       lock.LockID,
	}

	placementPID := l.placementPID.Load()
	if placementPID == nil {
		l.logger().Warn("IdentityStorageLookup: placement actor is nil (cluster shutting down?)",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		l.storage.RemoveLock(*lock)
		return nil
	}

	resp, err := l.cluster.ActorSystem.Root.RequestFuture(placementPID, req, 10*time.Second).Result()
	if err != nil {
		l.logger().Error("IdentityStorageLookup: placement actor request failed",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		l.storage.RemoveLock(*lock)
		return nil
	}

	activationResp, ok := resp.(*cluster.ActivationResponse)
	if !ok || activationResp.Failed || activationResp.Pid == nil {
		l.logger().Warn("IdentityStorageLookup: placement actor returned failure",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		l.storage.RemoveLock(*lock)
		return nil
	}

	// Populate the local PID cache.
	l.cluster.PidCache.Set(ci.Identity, ci.Kind, activationResp.Pid)

	return activationResp.Pid
}

// activateRemote sends an ActivationRequest to a remote node's proxy actor.
// The remote placement actor spawns the actor but does NOT persist (empty
// requestID causes PersistActivation to no-op). This node holds the storage
// lock and persists the activation after getting the PID back.
func (l *IdentityStorageLookup) activateRemote(ctx context.Context, ci *cluster.ClusterIdentity, lock *cluster.SpawnLock, member *cluster.Member) *actor.PID {
	proxyPID := actor.NewPID(member.Address(), "$proxy-activator")

	// Empty RequestId tells the remote PersistActivation to skip persistence.
	req := &cluster.ActivationRequest{
		ClusterIdentity: ci,
	}

	resp, err := l.cluster.ActorSystem.Root.RequestFuture(proxyPID, req, 10*time.Second).Result()
	if err != nil {
		l.logger().Error("IdentityStorageLookup: remote activation request failed",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.String("targetMember", member.Id),
			slog.Any("error", err))
		l.storage.RemoveLock(*lock)
		return nil
	}

	activationResp, ok := resp.(*cluster.ActivationResponse)
	if !ok || activationResp.Failed || activationResp.Pid == nil {
		l.logger().Warn("IdentityStorageLookup: remote activation returned failure",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.String("targetMember", member.Id))
		l.storage.RemoveLock(*lock)
		return nil
	}

	// Persist the activation locally — we hold the storage lock.
	l.storage.StoreActivation(l.memberID, lock, activationResp.Pid)

	// Populate the local PID cache.
	l.cluster.PidCache.Set(ci.Identity, ci.Kind, activationResp.Pid)

	return activationResp.Pid
}

// RemovePid removes the activation for a cluster identity from the storage
// backend. This is called when an actor terminates.
func (l *IdentityStorageLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	l.storage.RemoveActivation(&cluster.SpawnLock{
		ClusterIdentity: ci,
	})
}

// Setup initializes the lookup with the cluster context. On non-client
// nodes it spawns the placement actor and proxy actor, creates the
// strategy manager, and subscribes to topology events.
func (l *IdentityStorageLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	l.cluster = c
	l.isClient = isClient
	l.memberID = c.ActorSystem.ID

	// Inject the cluster logger into the storage backend if it supports it.
	type loggerSetter interface {
		SetLogger(logger *slog.Logger)
	}
	if ls, ok := l.storage.(loggerSetter); ok {
		ls.SetLogger(c.Logger())
	}

	if !isClient {
		// Create the PersistActivation callback that bridges requestID -> SpawnLock.
		// Empty requestID means a remote-initiated request where the requesting
		// node holds the lock and will persist — skip persistence here.
		persistActivation := func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID, requestID string) error {
			if requestID == "" {
				// Remote-initiated activation: the requesting node holds the
				// storage lock and persists after receiving our response.
				return nil
			}
			l.pendingLocksMu.Lock()
			lock, ok := l.pendingLocks[requestID]
			l.pendingLocksMu.Unlock()
			if !ok {
				return cluster.ErrLockNotHeld
			}
			l.storage.StoreActivation(l.memberID, lock, pid)
			return nil
		}

		// Create the RemoveActivation callback.
		removeActivation := func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID) error {
			l.storage.RemoveActivation(&cluster.SpawnLock{ClusterIdentity: ci})
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
			l.logger().Error("IdentityStorageLookup: failed to spawn placement actor",
				slog.Any("error", err))
		}
		l.placementPID.Store(placementPID)

		// Spawn the proxy actor.
		proxyProps := cluster.NewActivatorProxyProps(placementPID, l)
		proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$proxy-activator")
		if err != nil {
			l.logger().Error("IdentityStorageLookup: failed to spawn proxy actor",
				slog.Any("error", err))
		}
		l.proxyPID.Store(proxyPID)

		// Create the strategy manager.
		l.strategyManager.Store(cluster.NewStrategyManager(c))
	}

	// Subscribe to topology events to clean up activations when members
	// leave, and to update the strategy manager.
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			strMgr := l.strategyManager.Load()
			for _, member := range topology.Left {
				l.storage.RemoveMemberId(member.Id)
				if strMgr != nil {
					strMgr.RemoveMember(member)
				}
			}
			for _, member := range topology.Joined {
				if strMgr != nil {
					strMgr.AddMember(member)
				}
			}
		}
	})
}

// Shutdown performs graceful cleanup when the cluster is shutting down.
// It stops the placement actor first (which triggers graceful shutdown of all
// locally tracked grains), then stops the proxy activator, closes the strategy
// manager, and finally removes member records from storage.
func (l *IdentityStorageLookup) Shutdown() {
	l.defunct.Store(true)
	// Stop placement actor first — this triggers graceful shutdown of all
	// locally tracked grains (poisons them with DeactivationReasonShutdown).
	if placementPID := l.placementPID.Swap(nil); placementPID != nil {
		if err := l.cluster.ActorSystem.Root.PoisonFuture(placementPID).Wait(); err != nil {
			l.logger().Error("IdentityStorageLookup: failed to stop placement actor",
				slog.Any("error", err))
		}
	}

	// Stop proxy activator.
	if proxyPID := l.proxyPID.Swap(nil); proxyPID != nil {
		if err := l.cluster.ActorSystem.Root.PoisonFuture(proxyPID).Wait(); err != nil {
			l.logger().Error("IdentityStorageLookup: failed to stop proxy activator",
				slog.Any("error", err))
		}
	}

	// Close strategy manager.
	if strMgr := l.strategyManager.Swap(nil); strMgr != nil {
		strMgr.Close()
	}

	// Remove all activations belonging to this member from storage.
	if l.memberID != "" {
		l.storage.RemoveMemberId(l.memberID)
	}
}

// Peek checks if a grain activation exists without triggering activation.
func (l *IdentityStorageLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	notFound := &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
		},
		Status: cluster.PeekStatusNotFound,
	}

	if l.defunct.Load() {
		return nil, fmt.Errorf("IdentityStorageLookup: cannot Peek, shutdown was called")
	}

	// Step 1: Check for existing activation in storage.
	existing := l.storage.TryGetExistingActivation(clusterIdentity)
	if existing == nil {
		return notFound, nil
	}

	// Step 2: Validate owning member is alive.
	if !cluster.ValidateActivationMember(l.cluster.MemberList, existing.MemberID) {
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
	pid := l.pidFromStored(existing)
	ownerAddress := pid.Address
	proxyPID := actor.NewPID(ownerAddress, "$proxy-activator")
	future := l.cluster.ActorSystem.Root.RequestFuture(proxyPID, &cluster.PeekRequest{
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

// Compile-time check that IdentityStorageLookup implements cluster.GrainEnumerator.
var _ cluster.GrainEnumerator = (*IdentityStorageLookup)(nil)

func (l *IdentityStorageLookup) ListGrains() ([]*cluster.GrainInfo, error) {
	enum, ok := l.storage.(cluster.StorageGrainEnumerator)
	if !ok {
		return nil, cluster.ErrEnumerationNotSupported
	}
	infos, err := enum.ListActivations()
	if err != nil {
		return nil, err
	}
	return convertStoredToGrainInfos(infos), nil
}

func (l *IdentityStorageLookup) ListGrainsByKind(kind string) ([]*cluster.GrainInfo, error) {
	all, err := l.ListGrains()
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

func (l *IdentityStorageLookup) ListGrainsByMember(memberID string) ([]*cluster.GrainInfo, error) {
	enum, ok := l.storage.(cluster.StorageGrainEnumerator)
	if !ok {
		return nil, cluster.ErrEnumerationNotSupported
	}
	infos, err := enum.ListActivationsByMember(memberID)
	if err != nil {
		return nil, err
	}
	return convertStoredToGrainInfos(infos), nil
}

func convertStoredToGrainInfos(infos []*cluster.StoredActivationInfo) []*cluster.GrainInfo {
	result := make([]*cluster.GrainInfo, len(infos))
	for i, info := range infos {
		result[i] = cluster.StoredActivationInfoToGrainInfo(info)
	}
	return result
}

// pidFromStored converts a StoredActivation into an actor.PID.
// The StoredActivation.Pid field is formatted as "address/id".
func (l *IdentityStorageLookup) pidFromStored(stored *cluster.StoredActivation) *actor.PID {
	// Parse the PID string. The format is "address/id" as produced by PID.String().
	// We need to split on the first "/" since the address may not contain a slash
	// but the id might contain additional path segments.
	pidStr := stored.Pid
	for i := 0; i < len(pidStr); i++ {
		if pidStr[i] == '/' {
			return actor.NewPID(pidStr[:i], pidStr[i+1:])
		}
	}

	// If no slash found, treat the entire string as the ID with empty address.
	// This shouldn't happen with well-formed data, but we handle it gracefully.
	l.logger().Warn("IdentityStorageLookup: malformed PID in stored activation",
		slog.String("pid", pidStr))
	return actor.NewPID("", pidStr)
}
