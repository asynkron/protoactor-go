package cluster

import (
	"fmt"

	"github.com/AsynkronIT/protoactor-go/actor"
)

var (
	membershipPID *actor.PID
)

func spawnMembershipActor() {
	membershipPID = actor.SpawnNamed(actor.FromProducer(NewMembershipActor()), "#membership")
}

func NewMembershipActor() actor.Producer {
	return func() actor.Actor {
		return &membershipActor{}
	}
}

func init() {
	spawnMembershipActor()

	//subscribe the membership actor to the MemberStatusBatch event
	actor.EventStream.SubscribePID(func(m interface{}) bool {
		_, ok := m.(MemberStatusBatch)
		return ok
	}, membershipPID)
}

//membershipActor is responsible to keep track of the current cluster topology
//it does so by listening to changes from the ClusterProvider.
//the default ClusterProvider is consul_cluster.ConsulProvider which uses the Consul HTTP API to scan for changes
//TODO: we need some way of creating a hashring per "kind", maybe we should have a child actor to the membership actor that handles nodes
//per kind.
type membershipActor struct {
	members map[string]*MemberStatus
}

type MemberStatusEvent interface {
	MemberStatusEvent()
}

type MemberEvent struct {
	Address string
	Port    int
}

func (*MemberEvent) MemberStatusEvent() {}

type MemberJoinedEvent struct {
	MemberEvent
}

type MemberLeftEvent struct {
	MemberEvent
}

type MemberUnavailableEvent struct {
	MemberEvent
}

type MemberAvailableEvent struct {
	MemberEvent
}

func (a *membershipActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.members = make(map[string]*MemberStatus)
	case MemberStatusBatch:
		//TODO: keys that are present in the map but not in the message, are nodes that have left/been deregistered
		//we need to handle this too..
		for _, new := range msg {

			//key is address:port
			key := fmt.Sprintf("%v:%v", new.Address, new.Port)
			old := a.members[key]
			a.members[key] = new
			address := MemberEvent{
				Address: new.Address,
				Port:    new.Port,
			}
			if old == nil {
				//notify joined
				joined := &MemberJoinedEvent{MemberEvent: address}
				actor.EventStream.Publish(joined)
			} else {
				if old.Alive && !new.Alive {
					//notify node unavailable
					unavailable := &MemberUnavailableEvent{MemberEvent: address}
					actor.EventStream.Publish(unavailable)
				} else if !old.Alive && new.Alive {
					//notify node reachable
					available := &MemberAvailableEvent{MemberEvent: address}
					actor.EventStream.Publish(available)
				} else {
					//Ignore, no change...
				}
			}
		}
	}
}
