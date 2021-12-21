package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"sync"

	"github.com/AsynkronIT/protoactor-go/cluster/chash"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

// MemberList is responsible to keep track of the current cluster topology
// it does so by listening to changes from the ClusterProvider.
// the default ClusterProvider is consul.ConsulProvider which uses the Consul HTTP API to scan for changes
type MemberList struct {
	cluster              *Cluster
	mutex                sync.RWMutex
	members              *MemberSet
	memberStrategyByKind map[string]MemberStrategy
	bannedMembers        *MemberSet

	chashByKind map[string]chash.ConsistentHash
}

func NewMemberList(cluster *Cluster) *MemberList {
	memberList := &MemberList{
		cluster:              cluster,
		members:              emptyMemberSet,
		memberStrategyByKind: make(map[string]MemberStrategy),
		bannedMembers:        emptyMemberSet,
	}
	return memberList
}

func (ml *MemberList) GetActivatorMember(kind string, requestSourceAddress string) string {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	var res string
	if memberStrategy, ok := ml.memberStrategyByKind[kind]; ok {
		res = memberStrategy.GetActivator(requestSourceAddress)
	}
	return res
}

func (ml *MemberList) Length() int {
	return ml.members.Len()
}

func (ml *MemberList) Members() *MemberSet {
	return ml.members
}

func (ml *MemberList) UpdateClusterTopology(members []*Member) {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()

	// TLDR:
	// this method basically filters out any member status in the banned list
	// then makes a delta between new and old members
	// notifying the cluster accordingly which members left or joined

	topology, done, active, joined, left := ml.getTopologyChanges(members)
	if done {
		return
	}

	// include any new banned members into the known set of banned members
	ml.bannedMembers = ml.bannedMembers.Union(left)
	ml.members = active

	// notify that these members left
	for _, m := range left.Members() {
		ml.memberLeave(m)
		ml.TerminateMember(m)
	}

	// notify that these members joined
	for _, m := range joined.Members() {
		ml.memberJoin(m)
	}

	ml.cluster.ActorSystem.EventStream.Publish(topology)

	plog.Info("Updated ClusterTopology",
		log.Uint64("topologyHash", ml.members.TopologyHash()),
		log.Int("membersCount", len(members)),
		log.Int("joined", len(topology.Joined)),
		log.Int("left", len(topology.Left)),
	)
}

func (ml *MemberList) memberJoin(joiningMember *Member) {
	for _, kind := range joiningMember.Kinds {
		if ml.memberStrategyByKind[kind] == nil {
			ml.memberStrategyByKind[kind] = ml.getMemberStrategyByKind(kind)
		}

		ml.memberStrategyByKind[kind].AddMember(joiningMember)
	}
}

func (ml *MemberList) memberLeave(leavingMember *Member) {
	for _, kind := range leavingMember.Kinds {
		if ml.memberStrategyByKind[kind] != nil {
			continue
		}

		ml.memberStrategyByKind[kind].RemoveMember(leavingMember)
	}
}

func (ml *MemberList) getTopologyChanges(members []*Member) (topology *ClusterTopology, unchanged bool, active *MemberSet, joined *MemberSet, left *MemberSet) {
	memberSet := NewMemberSet(members)

	// get active members
	// (this bit means that we will never allow a member that failed a health check to join back in)
	active = memberSet.Except(ml.bannedMembers)

	// nothing changed? exit
	if active.Equals(ml.members) {
		return nil, true, nil, nil, nil
	}

	left = ml.members.Except(active)
	joined = active.Except(ml.members)

	topology = &ClusterTopology{
		TopologyHash: active.TopologyHash(),
		Members:      active.Members(),
		Left:         left.Members(),
		Joined:       joined.Members(),
	}
	return topology, false, active, joined, left
}

func (ml *MemberList) TerminateMember(m *Member) {
	// tell the world that this endpoint should is no longer relevant
	endpointTerminated := &remote.EndpointTerminatedEvent{
		Address: m.Address(),
	}

	ml.cluster.ActorSystem.EventStream.Publish(endpointTerminated)
}

func (ml *MemberList) BroadcastEvent(message interface{}, includeSelf bool) {
	for _, m := range ml.members.members {
		if !includeSelf && m.Id == ml.cluster.ActorSystem.Id {
			continue
		}

		pid := actor.NewPID(m.Address(), "eventstream")
		ml.cluster.ActorSystem.Root.Send(pid, message)
	}
}

func (ml *MemberList) getMemberStrategyByKind(kind string) MemberStrategy {
	clusterKind := ml.cluster.GetClusterKind(kind)

	var strategy MemberStrategy

	strategy = clusterKind.Strategy
	if strategy != nil {
		return strategy
	}

	strategy = ml.cluster.Config.MemberStrategyBuilder(ml.cluster, kind)
	if strategy != nil {
		return strategy
	}

	return newDefaultMemberStrategy(ml.cluster, kind)
}
