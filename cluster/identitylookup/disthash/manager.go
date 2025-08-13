package disthash

import (
	"github.com/asynkron/protoactor-go/actor"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/eventstream"
	"log/slog"
	"sync"
	"time"
)

const (
	// PartitionActivatorActorName is the name used for the partition activator actor.
	PartitionActivatorActorName = "partition-activator"
)

// Manager coordinates partition ownership and routing for virtual actors.
type Manager struct {
	cluster        *clustering.Cluster
	topologySub    *eventstream.Subscription
	placementActor *actor.PID
	rdvMutex       sync.RWMutex
	rdv            *clustering.Rendezvous
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

	activatorProps := actor.PropsFromProducer(func() actor.Actor { return newPlacementActor(pm.cluster, pm) })
	pm.placementActor, _ = system.Root.SpawnNamed(activatorProps, PartitionActivatorActorName)
	pm.cluster.Logger().Info("Started partition placement actor")

	pm.topologySub = system.EventStream.
		Subscribe(func(ev interface{}) {
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

// PidOfActivatorActor returns the PID of the partition activator on the given node.
func (pm *Manager) PidOfActivatorActor(addr string) *actor.PID {
	return actor.NewPID(addr, PartitionActivatorActorName)
}

func (pm *Manager) onClusterTopology(tplg *clustering.ClusterTopology) {
	pm.rdvMutex.Lock()
	defer pm.rdvMutex.Unlock()

	pm.cluster.Logger().Info("onClusterTopology", slog.Uint64("topology-hash", tplg.TopologyHash))

	for _, m := range tplg.Members {
		pm.cluster.Logger().Info("Got member", slog.Any("member", m))
	}

	pm.rdv = clustering.NewRendezvous()
	pm.rdv.UpdateMembers(tplg.Members)
	pm.cluster.ActorSystem.Root.Send(pm.placementActor, tplg)
}

// Get resolves the PID responsible for the given cluster identity.
// Returns nil if the cluster kind is unknown or activation failed.
func (pm *Manager) Get(identity *clustering.ClusterIdentity) *actor.PID {
	pm.rdvMutex.RLock()
	defer pm.rdvMutex.RUnlock()

	ownerAddress := pm.rdv.GetByClusterIdentity(identity)

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
