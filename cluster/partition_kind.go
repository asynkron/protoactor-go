package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster/chash"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

type PartitionKind struct {
	cluster    *Cluster
	Kind       string
	identity   *partitionIdentityActor
	activator  *partitionPlacementActor
	actorNames struct {
		Identity  string
		Placement string
	}
}

func newPartitionKind(c *Cluster, kind string) *PartitionKind {
	return &PartitionKind{
		cluster: c,
		Kind:    kind,
		actorNames: struct {
			Identity  string
			Placement string
		}{
			Identity:  ActorNameIdentity + "-" + kind,
			Placement: ActorNamePlacement + "-" + kind,
		},
	}
}

// Start ...
func (pm *PartitionKind) start(_chash chash.ConsistentHash) error {
	pm.identity = newPartitionIdentityActor(pm.cluster, pm, _chash)
	pm.activator = newPartitionPlacementActor(pm.cluster, pm, _chash)

	// spawn PartitionPlacementActor
	{
		props := actor.PropsFromProducer(func() actor.Actor {
			return pm.activator
		})
		pid, err := pm.cluster.ActorSystem.Root.SpawnNamed(props, pm.actorNames.Placement)
		if err != nil {
			return err
		}
		if err := pm.waiting(pid, 3*time.Second); err != nil {
			return err
		}
	}

	// spawn PartitionIdentityActor
	{
		props := actor.PropsFromProducer(func() actor.Actor {
			return pm.identity
		})
		pid, err := pm.cluster.ActorSystem.Root.SpawnNamed(props, pm.actorNames.Identity)
		if err != nil {
			return err
		}

		if err := pm.waiting(pid, 3*time.Second); err != nil {
			return err
		}
	}

	address := pm.identity.PID().GetAddress()
	plog.Info("Started Partition", log.String("kind", pm.Kind), log.String("address", address))
	return nil
}

// Stop ...
func (pm *PartitionKind) stop() {
	system := pm.cluster.ActorSystem
	if _, err := system.Root.PoisonFuture(pm.identity.PID()).Result(); err != nil {
		plog.Error("Stop actor failed", log.String("actor", ActorNameIdentity))
	}
	if _, err := system.Root.PoisonFuture(pm.activator.PID()).Result(); err != nil {
		plog.Error("Stop actor failed", log.String("actor", ActorNamePlacement))
	}
	plog.Info("Stopped partition", log.String("kind", pm.Kind))
}

// waiting actor ready OK.
func (pm *PartitionKind) waiting(pid *actor.PID, timeout time.Duration) error {
	ctx := pm.cluster.ActorSystem.Root
	if _, err := ctx.RequestFuture(pid, &remote.Ping{}, timeout).Result(); err != nil {
		return err
	}
	return nil
}

// PidOfIdentityActor ...
func (pm *PartitionKind) PidOfIdentityActor(addr string) *actor.PID {
	return &actor.PID{Address: addr, Id: pm.actorNames.Identity}
}

// PidOfPlacementActor ...
func (pm *PartitionKind) PidOfPlacementActor(addr string) *actor.PID {
	return &actor.PID{Address: addr, Id: pm.actorNames.Placement}
}
