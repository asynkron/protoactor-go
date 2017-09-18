package cluster

import (
	"fmt"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var (
	memberlistPID *actor.PID
	membershipSub *eventstream.Subscription
)

func spawnMembershipActor() {
	memberlistPID, _ = actor.SpawnNamed(actor.FromProducer(newMembershipActor()), "#membership")
}

func stopMembershipActor() {
	memberlistPID.GracefulStop()
}

func newMembershipActor() actor.Producer {
	return func() actor.Actor {
		return &memberlistActor{}
	}
}

func subscribeMembershipActorToEventStream() {
	membershipSub = eventstream.
		Subscribe(memberlistPID.Tell).
		WithPredicate(func(m interface{}) bool {
			_, ok := m.(ClusterTopologyEvent)
			return ok
		})
}

func unsubMembershipActorToEventStream() {
	eventstream.Unsubscribe(membershipSub)
}

// membershipActor is responsible to keep track of the current cluster topology
// it does so by listening to changes from the ClusterProvider.
// the default ClusterProvider is consul.ConsulProvider which uses the Consul HTTP API to scan for changes
type memberlistActor struct {
	members map[string]*MemberStatus
}

func (a *memberlistActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.members = make(map[string]*MemberStatus)
	case *MemberByKindRequest:
		var res []string

		//TODO: optimize this
		for key, v := range a.members {
			if !msg.onlyAlive || (msg.onlyAlive && v.Alive) {
				for _, k := range v.Kinds {
					if k == msg.kind {
						res = append(res, key)
					}
				}
			}
		}
		ctx.Respond(&MemberByKindResponse{
			members: res,
		})
	case ClusterTopologyEvent:

		//build a lookup for the new statuses
		tmp := make(map[string]*MemberStatus)
		for _, new := range msg {
			//key is address:port
			key := fmt.Sprintf("%v:%v", new.Host, new.Port)
			tmp[key] = new
		}

		//find the entires that only exist in the old set but not in the new
		rmvds := make([]string, 0)
		for key, old := range a.members {
			status := tmp[key]
			if status == nil || status != nil && !status.Alive {
				a.notify(key, nil, old, &rmvds)				
			}
		}
		for _, rmvd := range rmvds {
			delete(a.members, rmvd)
		}

		//find all the entries that exist in the new set
		rmvds = make([]string, 0)
		for key, new := range tmp {
			if a.members[key] == nil && new.Alive {
				a.members[key] = new
				a.notify(key, new, nil, &rmvds)
			}
		}		
		for _, rmvd := range rmvds {
			delete(a.members, rmvd)
		}
	}
}

func (a *memberlistActor) notify(key string, new *MemberStatus, old *MemberStatus, removed *[]string) {

	if new == nil && old == nil {
		//ignore, not possible
		return
	}
	if new == nil {
		//notify left
		meta := MemberMeta{
			Host:  old.Host,
			Port:  old.Port,
			Kinds: old.Kinds,
		}
		left := &MemberLeftEvent{MemberMeta: meta}
		eventstream.Publish(left)
		//delete(a.members, key) //remove this member as it has left
		*removed = append(*removed, key)

		rt := &remote.EndpointTerminatedEvent{
			Address: fmt.Sprintf("%v:%v", old.Host, old.Port),
		}
		eventstream.Publish(rt)

		return
	}
	if old == nil {
		//notify joined
		meta := MemberMeta{
			Host:  new.Host,
			Port:  new.Port,
			Kinds: new.Kinds,
		}
		joined := &MemberJoinedEvent{MemberMeta: meta}
		eventstream.Publish(joined)
		return
	}
	if new.MemberID != old.MemberID {
		meta := MemberMeta{
			Host:  new.Host,
			Port:  new.Port,
			Kinds: new.Kinds,
		}
		joined := &MemberRejoinedEvent{MemberMeta: meta}
		eventstream.Publish(joined)
		return
	}
	if old.Alive && !new.Alive {
		//notify node unavailable
		meta := MemberMeta{
			Host:  new.Host,
			Port:  new.Port,
			Kinds: new.Kinds,
		}
		unavailable := &MemberUnavailableEvent{MemberMeta: meta}
		eventstream.Publish(unavailable)
		return
	}
	if !old.Alive && new.Alive {
		//notify node reachable
		meta := MemberMeta{
			Host:  new.Host,
			Port:  new.Port,
			Kinds: new.Kinds,
		}
		available := &MemberAvailableEvent{MemberMeta: meta}
		eventstream.Publish(available)
	}
}
