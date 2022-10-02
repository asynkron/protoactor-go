package partition

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/eventstream"
	"github.com/asynkron/protoactor-go/log"
)

const (
	ActorNameIdentity  = "partition"
	ActorNamePlacement = "partition-activator"
)

type Manager struct {
	cluster        *clustering.Cluster
	topologySub    *eventstream.Subscription
	identityActor  *actor.PID
	placementActor *actor.PID
	rdv            *clustering.Rendezvous
}

func newPartitionManager(c *clustering.Cluster) *Manager {
	return &Manager{
		cluster: c,
	}
}

func (pm *Manager) Start() {
	plog.Info("Started partition manager")
	system := pm.cluster.ActorSystem

	identityProps := actor.PropsFromProducer(func() actor.Actor { return newIdentityActor(pm.cluster, pm) })
	pm.identityActor, _ = system.Root.SpawnNamed(identityProps, ActorNameIdentity)
	plog.Info("Started partition identity actor")

	activatorProps := actor.PropsFromProducer(func() actor.Actor { return newPlacementActor(pm.cluster, pm) })
	pm.placementActor, _ = system.Root.SpawnNamed(activatorProps, ActorNamePlacement)
	plog.Info("Started partition placement actor")

	pm.topologySub = system.EventStream.
		Subscribe(func(ev interface{}) {
			// fmt.Printf("PM got event.... %v", ev)
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

func (pm *Manager) PidOfIdentityActor(addr string) *actor.PID {
	return actor.NewPID(addr, ActorNameIdentity)
}

func (pm *Manager) PidOfActivatorActor(addr string) *actor.PID {
	return actor.NewPID(addr, ActorNamePlacement)
}

func (pm *Manager) onClusterTopology(tplg *clustering.ClusterTopology) {
	plog.Info("onClusterTopology", log.Uint64("eventId", tplg.TopologyHash))

	for _, m := range tplg.Members {
		plog.Info("Got member " + m.Id)

		for _, k := range m.Kinds {
			plog.Info("" + m.Id + " - " + k)
		}
	}

	pm.rdv = clustering.NewRendezvous()
	pm.rdv.UpdateMembers(tplg.Members)
	pm.cluster.ActorSystem.Root.Send(pm.identityActor, tplg)
}

func (pm *Manager) Get(identity *clustering.ClusterIdentity) *actor.PID {
	ownerAddress := pm.rdv.GetByClusterIdentity(identity)

	if ownerAddress == "" {
		return nil
	}

	identityOwnerPid := pm.PidOfIdentityActor(ownerAddress)
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
