package cluster

import (
	"sync"

	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var membershipSub *eventstream.Subscription

var ml = &memberlist{
	mutex:                &sync.RWMutex{},
	members:              make(map[string]*MemberStatus),
	memberStrategyByKind: make(map[string]MemberStrategy),
}

func subscribeMemberlistToEventStream() {
	membershipSub = eventstream.
		Subscribe(updateClusterTopology).
		WithPredicate(func(m interface{}) bool {
			_, ok := m.(ClusterTopologyEvent)
			return ok
		})
}

func unsubMemberlistToEventStream() {
	eventstream.Unsubscribe(membershipSub)
}

func getMembers(kind string) []string {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	res := make([]string, 0)
	if memberStrategy, ok := ml.memberStrategyByKind[kind]; ok {
		members := memberStrategy.GetAllMembers()
		for _, m := range members {
			if m.Alive {
				res = append(res, m.Address())
			}
		}
	}
	return res
}

func getPartitionMember(name, kind string) string {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	var res string
	if memberStrategy, ok := ml.memberStrategyByKind[kind]; ok {
		res = memberStrategy.GetPartition(name)
	}
	return res
}

func getActivatorMember(kind string) string {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	var res string
	if memberStrategy, ok := ml.memberStrategyByKind[kind]; ok {
		res = memberStrategy.GetActivator()
	}
	return res
}

func updateClusterTopology(m interface{}) {

	ml.mutex.Lock()
	defer ml.mutex.Unlock()

	msg, _ := m.(ClusterTopologyEvent)

	//build a lookup for the new statuses
	tmp := make(map[string]*MemberStatus)
	for _, new := range msg {
		tmp[new.Address()] = new
	}

	//first remove old ones
	for key, old := range ml.members {
		new := tmp[key]
		if new == nil {
			ml.updateAndNotify(new, old)
		}
	}

	//find all the entries that exist in the new set
	for key, new := range tmp {
		old := ml.members[key]
		ml.members[key] = new
		ml.updateAndNotify(new, old)
	}
}

// memberlist is responsible to keep track of the current cluster topology
// it does so by listening to changes from the ClusterProvider.
// the default ClusterProvider is consul.ConsulProvider which uses the Consul HTTP API to scan for changes
type memberlist struct {
	mutex                *sync.RWMutex
	members              map[string]*MemberStatus
	memberStrategyByKind map[string]MemberStrategy
}

func (a *memberlist) updateAndNotify(new *MemberStatus, old *MemberStatus) {

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
