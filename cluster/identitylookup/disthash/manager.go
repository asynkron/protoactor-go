package disthash

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/eventstream"
	"github.com/asynkron/protoactor-go/log"
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
	plog.Info("Started partition manager")
	system := pm.cluster.ActorSystem

	activatorProps := actor.PropsFromProducer(func() actor.Actor { return newPlacementActor(pm.cluster, pm) })
	pm.placementActor, _ = system.Root.SpawnNamed(activatorProps, PartitionActivatorActorName)
	plog.Info("Started partition placement actor")

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
		plog.Error("Failed to shutdown partition placement actor", log.Error(err))
	}

	plog.Info("Stopped PartitionManager")
}

func (pm *Manager) PidOfActivatorActor(addr string) *actor.PID {
	return actor.NewPID(addr, PartitionActivatorActorName)
}

func (pm *Manager) onClusterTopology(tplg *clustering.ClusterTopology) {
	plog.Info("onClusterTopology", log.Uint64("topology-hash", tplg.TopologyHash))

	for _, m := range tplg.Members {
		plog.Info("Got member ", log.String("MemberId", m.Id))
		for _, k := range m.Kinds {
			plog.Info("" + m.Id + " - " + k)
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
		RequestId:       "aaaa",
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
