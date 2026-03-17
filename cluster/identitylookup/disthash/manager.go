package disthash

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/eventstream"
)

const (
	// PlacementActorName is the well-known name for the shared placement actor.
	PlacementActorName = "partition-activator"
)

// Manager coordinates partition ownership and routing for virtual actors.
type Manager struct {
	cluster        *clustering.Cluster
	topologySub    *eventstream.Subscription
	placementActor *actor.PID
	rdvMutex       sync.RWMutex
	rdv            *clustering.Rendezvous
}

// PlacementActorPID returns the local placement actor PID.
func (pm *Manager) PlacementActorPID() *actor.PID {
	return pm.placementActor
}

func newPartitionManager(c *clustering.Cluster) *Manager {
	return &Manager{
		cluster: c,
		rdv:     clustering.NewRendezvous(),
	}
}

// Start initializes the manager and begins listening for topology changes.
func (pm *Manager) Start() {
	pm.cluster.Logger().Info("Started partition manager")
	system := pm.cluster.ActorSystem

	// RemoveActivation callback: broadcast ActivationTerminated to all
	// members so remote PID caches are invalidated.
	removeActivation := func(_ context.Context, ci *clustering.ClusterIdentity, pid *actor.PID) error {
		activationTerminated := &clustering.ActivationTerminated{
			Pid:             pid,
			ClusterIdentity: ci,
		}
		pm.cluster.MemberList.BroadcastEvent(activationTerminated, true)
		return nil
	}

	// RebalanceOnTopology callback: uses rendezvous hashing to identify
	// actors whose owner changed after a topology update.
	// Map keys are ClusterIdentity.AsKey() = "kind/identity", which
	// GetByIdentity parses by splitting on the first "/".
	rebalance := func(topology *clustering.ClusterTopology, actors map[string]*clustering.GrainMeta) []string {
		rdv := clustering.NewRendezvous()
		rdv.UpdateMembers(topology.Members)
		myAddress := pm.cluster.ActorSystem.Address()

		var keys []string
		for key := range actors {
			ownerAddress := rdv.GetByIdentity(key)
			if ownerAddress != myAddress {
				keys = append(keys, key)
			}
		}
		return keys
	}

	config := clustering.PlacementConfig{
		RemoveActivation:    removeActivation,
		RebalanceOnTopology: rebalance,
	}

	activatorProps := clustering.NewPlacementActorProps(pm.cluster, config)
	pm.placementActor, _ = system.Root.SpawnNamed(activatorProps, PlacementActorName)
	pm.cluster.Logger().Info("Started partition placement actor")

	pm.topologySub = system.EventStream.
		Subscribe(func(ev any) {
			if topology, ok := ev.(*clustering.ClusterTopology); ok {
				pm.onClusterTopology(topology)
			}
		})
}

// Stop terminates the placement actor and unsubscribes from topology events.
func (pm *Manager) Stop() {
	system := pm.cluster.ActorSystem
	system.EventStream.Unsubscribe(pm.topologySub)

	err := system.Root.PoisonFuture(pm.placementActor).Wait()
	if err != nil {
		pm.cluster.Logger().Error("Failed to shutdown partition placement actor", slog.Any("error", err))
	}

	pm.cluster.Logger().Info("Stopped PartitionManager")
}

// PidOfActivatorActor returns the PID of the placement actor on the given node.
func (pm *Manager) PidOfActivatorActor(addr string) *actor.PID {
	return actor.NewPID(addr, PlacementActorName)
}

func (pm *Manager) onClusterTopology(tplg *clustering.ClusterTopology) {
	pm.rdvMutex.Lock()
	defer pm.rdvMutex.Unlock()

	// gather member addresses to provide a concise summary log while
	// keeping detailed member data available at debug level
	memberAddrs := make([]string, len(tplg.Members))
	for i, m := range tplg.Members {
		addr := m.Host + ":" + strconv.Itoa(int(m.Port))
		pm.cluster.Logger().Debug("Topology member", slog.String("id", m.Id), slog.String("address", addr), slog.String("kinds", strings.Join(m.Kinds, ",")))
		memberAddrs[i] = addr
	}
	// log the overall topology change in a single info log
	pm.cluster.Logger().Info("onClusterTopology", slog.Uint64("topology-hash", tplg.TopologyHash), slog.Int("member-count", len(memberAddrs)), slog.String("members", strings.Join(memberAddrs, ",")))

	pm.rdv = clustering.NewRendezvous()
	pm.rdv.UpdateMembers(tplg.Members)
	pm.cluster.ActorSystem.Root.Send(pm.placementActor, tplg)
}

// Get resolves the PID responsible for the given cluster identity.
// Returns nil if the cluster kind is unknown or activation failed.
func (pm *Manager) Get(identity *clustering.ClusterIdentity) *actor.PID {
	// Snapshot the rendezvous under the read lock, then release before
	// making the blocking RPC call. Holding the RLock across the 5s
	// RequestFuture would block topology updates (which need a write lock).
	pm.rdvMutex.RLock()
	rdv := pm.rdv
	pm.rdvMutex.RUnlock()

	ownerAddress := rdv.GetByClusterIdentity(identity)

	if ownerAddress == "" {
		return nil
	}

	identityOwnerPid := pm.PidOfActivatorActor(ownerAddress)
	request := &clustering.ActivationRequest{
		ClusterIdentity: identity,
		RequestId:       "",
	}
	future := pm.cluster.ActorSystem.Root.RequestFuture(identityOwnerPid, request, 5*time.Second)
	res, err := future.Result()
	if err != nil {
		return nil
	}
	typed, ok := res.(*clustering.ActivationResponse)
	if !ok || typed.Failed {
		return nil
	}
	return typed.Pid
}
