package cluster

import (
	"fmt"

	"github.com/AsynkronIT/protoactor-go/actor"
)

var (
	memberlistPID *actor.PID
)

func spawnMembershipActor() {
	memberlistPID = actor.SpawnNamed(actor.FromProducer(newMembershipActor()), "#membership")
}

func newMembershipActor() actor.Producer {
	return func() actor.Actor {
		return &memberlistActor{}
	}
}

func subscribeMembershipActorToEventStream() {
	actor.EventStream.SubscribePID(func(m interface{}) bool {
		_, ok := m.(MemberStatusBatch)
		return ok
	}, memberlistPID)
}

//membershipActor is responsible to keep track of the current cluster topology
//it does so by listening to changes from the ClusterProvider.
//the default ClusterProvider is consul_cluster.ConsulProvider which uses the Consul HTTP API to scan for changes
//TODO: we need some way of creating a hashring per "kind", maybe we should have a child actor to the membership actor that handles nodes
//per kind.
type memberlistActor struct {
	members map[string]*MemberStatus
}

func (a *memberlistActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.members = make(map[string]*MemberStatus)
	case *MemberByKindRequest:
		var res []string
		for key, v := range a.members {
			if !msg.onlyAlive || (msg.onlyAlive && v.Alive) {
				res = append(res, key)
			}
		}
		ctx.Respond(&MemberByKindResponse{
			members: res,
		})
	case MemberStatusBatch:

		//build a lookup for the new statuses
		tmp := make(map[string]*MemberStatus)
		for _, new := range msg {
			//key is address:port
			key := fmt.Sprintf("%v:%v", new.Address, new.Port)
			tmp[key] = new
		}

		//find the entires that only exist in the old set but not in the new
		for key, old := range a.members {
			new := tmp[key]
			if new == nil {
				a.notify(new, old)
			}
		}

		//find all the entries that exist in the new set
		for key, new := range tmp {
			old := a.members[key]
			a.members[key] = new
			a.notify(new, old)
		}
	}
}

func (a *memberlistActor) notify(new *MemberStatus, old *MemberStatus) {
	address := MemberEvent{
		Address: new.Address,
		Port:    new.Port,
	}
	if new == nil && old == nil {
		//ignore, not possible
		return
	}
	if new == nil {
		//notify left
		left := &MemberLeftEvent{MemberEvent: address}
		actor.EventStream.Publish(left)
		return
	}
	if old == nil {
		//notify joined
		joined := &MemberJoinedEvent{MemberEvent: address}
		actor.EventStream.Publish(joined)
		return
	}
	if old.Alive && !new.Alive {
		//notify node unavailable
		unavailable := &MemberUnavailableEvent{MemberEvent: address}
		actor.EventStream.Publish(unavailable)
		return
	}
	if !old.Alive && new.Alive {
		//notify node reachable
		available := &MemberAvailableEvent{MemberEvent: address}
		actor.EventStream.Publish(available)
	}
}
