package cluster

import (
	"sort"
	"strings"
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
	members              map[string]*Member
	memberStrategyByKind map[string]MemberStrategy
	banned               map[string]*Member
	topologyHash         uint64
	chashByKind          map[string]chash.ConsistentHash
}

func NewMemberList(cluster *Cluster) *MemberList {
	return setupMemberList(cluster)
}

func setupMemberList(cluster *Cluster) *MemberList {
	memberList := &MemberList{
		cluster:              cluster,
		members:              make(map[string]*Member),
		memberStrategyByKind: make(map[string]MemberStrategy),
		banned:               make(map[string]*Member),
	}
	return memberList
}

func (ml *MemberList) getPartitionMemberV2(clusterIdentity *ClusterIdentity) string {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
	if ms, ok := ml.memberStrategyByKind[clusterIdentity.Kind]; ok {
		return ms.GetPartition(clusterIdentity.Identity)
	}
	return ""
}

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
	return len(ml.members)
}

func (ml *MemberList) UpdateClusterTopology(members []*Member) {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()

	//1. filter out banned and dead members
	activeMembers := make([]*Member, 0)
	activeMemberIds := make(map[string]bool)
	for _, m := range members {
		if _, isBaned := ml.banned[m.Id]; isBaned {
			continue
		}
		activeMembers = append(activeMembers, m)
		activeMemberIds[m.Id] = true
	}

	//2. Compute hash
	newTopologyHash := GetMembershipHashCode(activeMembers)

	//3. if nothing has changed, bail out...
	if newTopologyHash == ml.topologyHash {
		return
	}

	ml.topologyHash = newTopologyHash

	//4. create the new topology
	newTopology := &ClusterTopology{
		TopologyHash: newTopologyHash,
		Members:      activeMembers,
	}

	//5. find members that existed before but not anymore
	leftMembers := make([]*Member, 0)
	leftMemberIds := make(map[string]bool)
	for _, m := range ml.members {
		if _, isActive := activeMemberIds[m.Id]; isActive {
			continue
		}
		leftMembers = append(leftMembers, m)
		leftMemberIds[m.Id] = true
		ml.banned[m.Id] = m

		//tell the world that this endpoint should is no longer relevant
		endpointTerminated := &remote.EndpointTerminatedEvent{
			Address: m.Address(),
		}

		ml.cluster.ActorSystem.EventStream.Publish(endpointTerminated)
	}

	newTopology.Left = leftMembers

	//6. get all banned members
	bannedMembers := make([]string, 0)
	for _, m := range ml.banned {
		bannedMembers = append(bannedMembers, m.Id)
	}

	newTopology.Banned = bannedMembers

	//7. find members that joined
	joinedMembers := make([]*Member, 0)
	joinedMemberIds := make(map[string]bool)
	for _, m := range activeMembers {
		if _, isExisting := ml.members[m.Id]; isExisting {
			continue
		}
		joinedMembers = append(joinedMembers, m)
		joinedMemberIds[m.Id] = true
	}

	newTopology.Joined = joinedMembers

	//newTopology now contains:
	//TopologyHash
	//Members
	//Left Members
	//Joined Members
	//Banned Members

	ml.cluster.ActorSystem.EventStream.Publish(newTopology)

	plog.Info("Updated ClusterTopology",
		log.Uint64("topologyHash", ml.topologyHash),
		log.Int("members", len(members)),
		log.Int("joined", len(newTopology.Joined)),
		log.Int("left", len(newTopology.Left)),
	)
}

func (ml *MemberList) onMembersUpdated(tplg *ClusterTopology) {
	groups := GroupMembersByKind(tplg.Members)
	strategies := map[string]MemberStrategy{}
	chashes := map[string]chash.ConsistentHash{}
	for kind, members := range groups {
		strategies[kind] = newDefaultMemberStrategyV2(kind, members)
		chashes[kind] = NewRendezvousV2(members)
	}
	ml.memberStrategyByKind = strategies
	ml.chashByKind = chashes
}

func (ml *MemberList) onMemberLeft(member *Member) {
	// notify left
	meta := MemberMeta{
		Host:  member.Host,
		Port:  int(member.Port),
		Kinds: member.Kinds,
	}
	left := &MemberLeftEvent{MemberMeta: meta}
	ml.cluster.ActorSystem.EventStream.PublishUnsafe(left)

	addr := member.Address()
	delete(ml.members, addr)
	rt := &remote.EndpointTerminatedEvent{Address: addr}
	ml.cluster.ActorSystem.EventStream.PublishUnsafe(rt)
	return
}

func (ml *MemberList) onMemberJoined(member *Member) {
	addr := member.Address()
	ml.members[addr] = member
	// notify joined
	meta := MemberMeta{
		Host:  member.Host,
		Port:  int(member.Port),
		Kinds: member.Kinds,
	}
	joined := &MemberJoinedEvent{MemberMeta: meta}
	ml.cluster.ActorSystem.EventStream.PublishUnsafe(joined)
}

func (ml *MemberList) buildSortedMembers(m map[string]*Member) []*Member {
	list := make([]*Member, len(m))
	i := 0
	for _, member := range m {
		list[i] = member
		i++
	}
	sortMembers(list)
	return list
}

func sortMembers(members []*Member) {
	sort.Slice(members, func(i, j int) bool {
		addrI := members[i].Address()
		addrJ := members[j].Address()
		return strings.Compare(addrI, addrJ) > 0
	})
}

func GroupMembersByKind(members []*Member) map[string][]*Member {
	groups := map[string][]*Member{}
	for _, member := range members {
		for _, kind := range member.Kinds {
			if list, ok := groups[kind]; ok {
				groups[kind] = append(list, member)
			} else {
				groups[kind] = []*Member{member}
			}
		}
	}
	return groups
}
