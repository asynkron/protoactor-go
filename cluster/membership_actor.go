package cluster

import (
	"fmt"

	"github.com/AsynkronIT/protoactor-go/actor"
)

var (
	membershipPID *actor.PID
)

func spawnMembershipActor() {
	membershipPID = actor.SpawnNamed(actor.FromProducer(newClusterActor()), "#membership")
}

func newMembershipActor() actor.Producer {
	return func() actor.Actor {
		return &membershipActor{}
	}
}

//membershipActor is responsible to keep track of the current cluster topology
//it does so by listening to changes from the ClusterProvider.
//the default ClusterProvider is consul_cluster.ConsulProvider which uses the Consul HTTP API to scan for changes
//TODO: we need some way of creating a hashring per "kind", maybe we should have a child actor to the membership actor that handles nodes
//per kind.
type membershipActor struct {
	members map[string]*MemberStatus
}

func (a *membershipActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.members = make(map[string]*MemberStatus)
	case []*MemberStatus:
		//TODO: keys that are present in the map but not in the message, are nodes that have left/been deregistered
		//we need to handle this too..
		for _, new := range msg {
			key := fmt.Sprintf("%v:%v", new.Alive, new.Port)
			old := a.members[key]
			a.members[key] = new
			if old == nil {
				//notify joined
			} else {
				if old.Alive && !new.Alive {
					//notify node unavailable
				}
				if !old.Alive && new.Alive {
					//notify node reachable
				}
			}
		}
	}
}
