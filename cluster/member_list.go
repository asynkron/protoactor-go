package cluster

import (
	"sort"
	"strings"
	"sync"

	"github.com/AsynkronIT/protoactor-go/cluster/chash"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

type ClusterTopologyEventV2 struct {
	*ClusterTopology
	chashByKind map[string]chash.ConsistentHash
}

// MemberList is responsible to keep track of the current cluster topology
// it does so by listening to changes from the ClusterProvider.
// the default ClusterProvider is consul.ConsulProvider which uses the Consul HTTP API to scan for changes
type MemberList struct {
	cluster              *Cluster
	mutex                sync.RWMutex
	members              map[string]*Member
	memberStrategyByKind map[string]MemberStrategy
	banned               map[string]struct{}
	lastEventId          uint64
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
		banned:               map[string]struct{}{},
	}
	return memberList
}

func (ml *MemberList) stopMemberList() {
	// ml.cluster.ActorSystem.EventStream.Unsubscribe(ml.membershipSub)
}

func (ml *MemberList) getPartitionMember(name, kind string) string {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	var res string
	if memberStrategy, ok := ml.memberStrategyByKind[kind]; ok {
		res = memberStrategy.GetPartition(name)
	}
	return res
}

func (ml *MemberList) getPartitionMemberV2(clusterIdentity *ClusterIdentity) string {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
	if ms, ok := ml.memberStrategyByKind[clusterIdentity.Kind]; ok {
		return ms.GetPartition(clusterIdentity.Identity)
	}
	return ""
}

func (ml *MemberList) getActivatorMember(kind string) string {
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

func (ml *MemberList) UpdateClusterTopology(members []*Member, eventId uint64) {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()
	if ml.lastEventId >= eventId {
		// plog.Debug("Skipped ClusterTopology", log.Int("members", len(members)), log.Uint64("eventId", eventId))
		return
	}
	ml.lastEventId = eventId
	tplg := ml._updateClusterTopoLogy(members, eventId)

	ml.onMembersUpdated(tplg)
	ml.cluster.ActorSystem.EventStream.PublishUnsafe(&ClusterTopologyEventV2{
		ClusterTopology: tplg,
		chashByKind:     ml.chashByKind,
	})
	plog.Info("Updated ClusterTopology",
		log.Uint64("eventId", ml.lastEventId),
		log.Int("members", len(members)),
		log.Int("joined", len(tplg.Joined)),
		log.Int("left", len(tplg.Left)),
		log.Int("alives", len(tplg.Members)))
}

func (ml *MemberList) _updateClusterTopoLogy(members []*Member, eventId uint64) *ClusterTopology {
	tplg := ClusterTopology{EventId: eventId}

	alives := map[string]*Member{}
	for _, member := range members {
		if _, isBaned := ml.banned[member.Id]; isBaned {
			continue
		}
		addr := member.Address()
		alives[addr] = member
		if old, isOld := ml.members[addr]; isOld {
			if len(old.Kinds) != len(member.Kinds) {
				plog.Error("member.Kinds is different to the old one",
					log.String("old", old.String()), log.String("new", member.String()))
			}
			continue
		}
		tplg.Joined = append(tplg.Joined, member)
		ml.onMemberJoined(member)
	}

	for _, member := range ml.members {
		addr := member.Address()
		if _, isExist := alives[addr]; !isExist {
			ml.onMemberLeft(member)
			tplg.Left = append(tplg.Left, member)
		}
	}
	tplg.Members = ml.buildSortedMembers(alives)
	return &tplg
}

func (ml *MemberList) onMembersUpdated(tplg *ClusterTopology) {
	groups := groupMembersByKind(tplg.Members)
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

func groupMembersByKind(members []*Member) map[string][]*Member {
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
