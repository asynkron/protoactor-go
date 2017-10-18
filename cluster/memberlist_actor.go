package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
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
	members              map[string]*MemberStatus
	memberStrategyByKind map[string]MemberStrategy
}

func (a *memberlistActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		a.members = make(map[string]*MemberStatus)
		a.memberStrategyByKind = make(map[string]MemberStrategy)
	case *MembersByKindRequest:
		res := make([]string, 0)
		if memberStrategy, ok := a.memberStrategyByKind[msg.kind]; ok {
			members := memberStrategy.GetAllMembers()
			for _, m := range members {
				if !msg.onlyAlive || m.Alive {
					res = append(res, m.Address())
				}
			}
		}
		ctx.Respond(&MembersResponse{members: res})
	case *PartitionMemberRequest:
		var res string
		if memberStrategy, ok := a.memberStrategyByKind[msg.kind]; ok {
			res = memberStrategy.GetPartition(msg.name)
		}
		ctx.Respond(&MemberResponse{res})
	case *ActivatorMemberRequest:
		var res string
		if memberStrategy, ok := a.memberStrategyByKind[msg.kind]; ok {
			res = memberStrategy.GetActivator()
		}
		ctx.Respond(&MemberResponse{res})
	case ClusterTopologyEvent:

		//build a lookup for the new statuses
		tmp := make(map[string]*MemberStatus)
		for _, new := range msg {
			tmp[new.Address()] = new
		}

		//first remove old ones
		for key, old := range a.members {
			new := tmp[key]
			if new == nil {
				a.updateAndNotify(new, old)
			}
		}

		//find all the entries that exist in the new set
		for key, new := range tmp {
			old := a.members[key]
			a.members[key] = new
			a.updateAndNotify(new, old)
		}
	}
}

func (a *memberlistActor) updateAndNotify(new *MemberStatus, old *MemberStatus) {

	if new == nil && old == nil {
		//ignore, not possible
		return
	}
	if new == nil {
		//update MemberStrategy
		for _, k := range old.Kinds {
			if s, ok := a.memberStrategyByKind[k]; ok {
				s.RemoveMember(old)
				if len(s.GetAllMembers()) == 0 {
					delete(a.memberStrategyByKind, k)
				}
			}
		}

		//notify left
		meta := MemberMeta{
			Host:  old.Host,
			Port:  old.Port,
			Kinds: old.Kinds,
		}
		left := &MemberLeftEvent{MemberMeta: meta}
		eventstream.Publish(left)
		delete(a.members, old.Address()) //remove this member as it has left

		rt := &remote.EndpointTerminatedEvent{
			Address: old.Address(),
		}
		eventstream.Publish(rt)

		return
	}
	if old == nil {
		//update MemberStrategy
		for _, k := range new.Kinds {
			if _, ok := a.memberStrategyByKind[k]; !ok {
				a.memberStrategyByKind[k] = cfg.MemberStrategyBuilder(k)
			}
			a.memberStrategyByKind[k].AddMember(new)
		}

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

	//update MemberStrategy
	if new.Alive != old.Alive || new.MemberID != old.MemberID || new.StatusValue != nil && !new.StatusValue.IsSame(old.StatusValue) {
		for _, k := range new.Kinds {
			if _, ok := a.memberStrategyByKind[k]; !ok {
				a.memberStrategyByKind[k] = cfg.MemberStrategyBuilder(k)
			}
			a.memberStrategyByKind[k].AddMember(new)
		}
	}

	if new.MemberID != old.MemberID {
		//notify member rejoined
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
		//notify member unavailable
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
		//notify member reachable
		meta := MemberMeta{
			Host:  new.Host,
			Port:  new.Port,
			Kinds: new.Kinds,
		}
		available := &MemberAvailableEvent{MemberMeta: meta}
		eventstream.Publish(available)
	}
}
