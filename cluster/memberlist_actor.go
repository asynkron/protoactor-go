package cluster

import (
	"fmt"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster/members"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var (
	memberlistPID *actor.PID
	membershipSub *eventstream.Subscription
)

func spawnMembershipActor() {
	memberlistPID, _ = actor.SpawnNamed(actor.FromProducer(newMembershipActor()), "memberlist")
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
	members        map[string]*MemberStatus
	membersByKinds map[string]*members.MemberNodeSet
}

func (a *memberlistActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.members = make(map[string]*MemberStatus)
		a.membersByKinds = make(map[string]*members.MemberNodeSet)
	case *MembersByKindRequest:
		var res []string
		if members, ok := a.membersByKinds[msg.kind]; ok {
			res = members.GetAllMemberAddresses(msg.onlyAlive)
		}
		ctx.Respond(&MembersResponse{members: res})
	case *MemberByRoundRobinRequest:
		var res string
		if members, ok := a.membersByKinds[msg.kind]; ok {
			res = members.GetByRoundRobin()
		}
		ctx.Respond(&MemberResponse{res})
	case *MemberByDHTRequest:
		var res string
		if members, ok := a.membersByKinds[msg.kind]; ok {
			res = members.GetByRdv(msg.name)
		}
		ctx.Respond(&MemberResponse{res})
	case ClusterTopologyEvent:

		//build a lookup for the new statuses
		tmp := make(map[string]*MemberStatus)
		for _, new := range msg {
			//key is address:port
			key := a.getKey(new)
			tmp[key] = new
		}

		//first remove old ones
		for key, old := range a.members {
			new := tmp[key]
			if new == nil {
				a.notify(key, new, old)
				a.updateMembersByKind(key, new, old)
			}
		}

		//find all the entries that exist in the new set
		for key, new := range tmp {
			old := a.members[key]
			a.members[key] = new
			a.notify(key, new, old)
			a.updateMembersByKind(key, new, old)
		}
	}
}

func (a *memberlistActor) getKey(m *MemberStatus) string {
	return fmt.Sprintf("%v:%v", m.Host, m.Port)
}

func (a *memberlistActor) notify(key string, new *MemberStatus, old *MemberStatus) {

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
		delete(a.members, key) //remove this member as it has left

		rt := &remote.EndpointTerminatedEvent{
			Address: a.getKey(old),
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

func (a *memberlistActor) updateMembersByKind(key string, new *MemberStatus, old *MemberStatus) {
	if old != nil {
		for _, k := range old.Kinds {
			if s, ok := a.membersByKinds[k]; ok {
				s.Remove(key)
				if s.Length() == 0 {
					delete(a.membersByKinds, k)
				}
			}
		}
	}
	if new != nil {
		for _, k := range new.Kinds {
			if _, ok := a.membersByKinds[k]; !ok {
				a.membersByKinds[k] = members.NewMemberNodeSet()
			}
			a.membersByKinds[k].Add(key, new.Alive, new.Weight)
		}
	}
}
