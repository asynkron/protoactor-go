package disthash

import (
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/eventstream"
)

const (
	PartitionActivatorActorName = "partition-activator"
)

type Manager struct {
	cluster        *clustering.Cluster
	topologySub    *eventstream.Subscription
	placementActor *actor.PID
	rdv            *clustering.Rendezvous
}

func newPartitionManager(c *clustering.Cluster) *Manager {
	return &Manager{
		cluster: c,
		rdv:     clustering.NewRendezvous(),
	}
}

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

func (pm *Manager) Stop() {
	system := pm.cluster.ActorSystem
	system.EventStream.Unsubscribe(pm.topologySub)

	err := system.Root.PoisonFuture(pm.placementActor).Wait()
	if err != nil {
		pm.cluster.Logger().Error("Failed to shutdown partition placement actor", slog.Any("error", err))
	}

	pm.cluster.Logger().Info("Stopped PartitionManager")
}

func (pm *Manager) PidOfActivatorActor(addr string) *actor.PID {
	return actor.NewPID(addr, PartitionActivatorActorName)
}

func (pm *Manager) onClusterTopology(tplg *clustering.ClusterTopology) {
	pm.cluster.Logger().Info("onClusterTopology", slog.Uint64("topology-hash", tplg.TopologyHash))

	for _, m := range tplg.Members {
		pm.cluster.Logger().Info("Got member ", slog.String("MemberId", m.Id))
		for _, k := range m.Kinds {
			pm.cluster.Logger().Info("" + m.Id + " - " + k)
		}
	}

	pm.rdv = clustering.NewRendezvous()
	pm.rdv.UpdateMembers(tplg.Members)
	pm.cluster.ActorSystem.Root.Send(pm.placementActor, tplg)
}

func (pm *Manager) Get(identity *clustering.ClusterIdentity) *actor.PID {
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
	if !ok {
		return nil
	}
	return typed.Pid
}
