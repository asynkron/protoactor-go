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
	membersByMemberId    map[string]*Member
	memberStrategyByKind map[string]MemberStrategy
	bannedMemberIds      map[string]bool
	topologyHash         uint64
	chashByKind          map[string]chash.ConsistentHash
}

func NewMemberList(cluster *Cluster) *MemberList {
	memberList := &MemberList{
		cluster:              cluster,
		membersByMemberId:    make(map[string]*Member),
		memberStrategyByKind: make(map[string]MemberStrategy),
		bannedMemberIds:      make(map[string]bool),
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
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
	return len(ml.membersByMemberId)
}

func (ml *MemberList) UpdateClusterTopology(members []*Member) {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()

	//get active members
	//(this bit means that we will never allow a member that failed a health check to join back in)
	activeMembers := MembersExcept(members, ml.bannedMemberIds)

	//get the new topology hash
	newTopologyHash := TopologyHash(activeMembers)

	//nothing changed? exit
	if newTopologyHash == ml.topologyHash {
		return
	}

	//remember the new topology hash
	ml.topologyHash = newTopologyHash

	//membersByMemberId that left
	left := ml.getLeftMembers(activeMembers)

	//membersByMemberId that joined
	joined := ml.getJoinedMembers(activeMembers)

	//union membersByMemberId that left into bannedMemberIds set
	AddMembersToSet(ml.bannedMemberIds, left)

	//replace the member lookup with new data
	ml.membersByMemberId = MembersToMap(activeMembers)

	//for any member that left, send a endpoint terminate event
	for _, m := range left {
		ml.TerminateMember(m)
	}

	newTopology := &ClusterTopology{
		TopologyHash: newTopologyHash,
		Members:      activeMembers,
		Left:         left,
		Joined:       joined,
	}

	//recalculate member strategies
	ml.refreshMemberStrategies(newTopology)

	ml.cluster.ActorSystem.EventStream.Publish(newTopology)

	plog.Info("Updated ClusterTopology",
		log.Uint64("topologyHash", ml.topologyHash),
		log.Int("membersByMemberId", len(members)),
		log.Int("joined", len(newTopology.Joined)),
		log.Int("left", len(newTopology.Left)),
	)
}

func (ml *MemberList) getJoinedMembers(activeMembers []*Member) []*Member {
	joinedMembers := make([]*Member, 0)
	joinedMemberIds := make(map[string]bool)
	for _, m := range activeMembers {
		if _, isExisting := ml.membersByMemberId[m.Id]; isExisting {
			continue
		}
		joinedMembers = append(joinedMembers, m)
		joinedMemberIds[m.Id] = true
	}
	return joinedMembers
}

func (ml *MemberList) getLeftMembers(activeMembers []*Member) []*Member {
	activeMemberIds := MembersToSet(activeMembers)
	leftMembers := make([]*Member, 0)
	leftMemberIds := make(map[string]bool)
	for _, m := range ml.membersByMemberId {
		if _, isActive := activeMemberIds[m.Id]; isActive {
			continue
		}
		leftMembers = append(leftMembers, m)
		leftMemberIds[m.Id] = true
	}
	return leftMembers
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
