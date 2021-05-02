package cluster

import (
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

//func (ml *MemberList) getPartitionMemberV2(clusterIdentity *ClusterIdentity) string {
//	ml.mutex.RLock()
//	defer ml.mutex.RUnlock()
//	if ms, ok := ml.memberStrategyByKind[clusterIdentity.Kind]; ok {
//		return ms.GetPartition(clusterIdentity.Identity)
//	}
//	return ""
//}

func (ml *MemberList) GetActivatorMember(kind string) string {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	var res string
	if memberStrategy, ok := ml.memberStrategyByKind[kind]; ok {
		res = memberStrategy.GetActivator()
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

	memberSet := NewMemberSet(members)

	//get active members
	//(this bit means that we will never allow a member that failed a health check to join back in)
	newMembers := memberSet.Except(ml.bannedMembers)

	//nothing changed? exit
	if newMembers.Equals(ml.members) {
		return
	}

	left := ml.members.Except(newMembers)
	joined := newMembers.Except(ml.members)
	ml.bannedMembers = ml.bannedMembers.Union(left)
	ml.members = newMembers

	//for any member that left, send a endpoint terminate event
	for _, m := range left.Members() {
		ml.TerminateMember(m)
	}

	newTopology := &ClusterTopology{
		TopologyHash: newMembers.TopologyHash(),
		Members:      newMembers.Members(),
		Left:         left.Members(),
		Joined:       joined.Members(),
	}

	//recalculate member strategies
	ml.refreshMemberStrategies(newTopology)

	ml.cluster.ActorSystem.EventStream.Publish(newTopology)

	plog.Info("Updated ClusterTopology",
		log.Uint64("topologyHash", ml.members.TopologyHash()),
		log.Int("membersByMemberId", len(members)),
		log.Int("joined", len(newTopology.Joined)),
		log.Int("left", len(newTopology.Left)),
	)
}

func (ml *MemberList) TerminateMember(m *Member) {
	//tell the world that this endpoint should is no longer relevant
	endpointTerminated := &remote.EndpointTerminatedEvent{
		Address: m.Address(),
	}

	ml.cluster.ActorSystem.EventStream.Publish(endpointTerminated)
}

func (ml *MemberList) refreshMemberStrategies(tplg *ClusterTopology) {
	groups := GroupMembersByKind(tplg.Members)
	strategies := map[string]MemberStrategy{}
	chashes := map[string]chash.ConsistentHash{}
	for kind, membersByMemberId := range groups {
		strategies[kind] = newDefaultMemberStrategyV2(kind, membersByMemberId)
		chashes[kind] = NewRendezvousV2(membersByMemberId)
	}
	ml.memberStrategyByKind = strategies
	ml.chashByKind = chashes
}
