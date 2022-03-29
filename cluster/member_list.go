package cluster

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster/chash"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"sync"

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
	blockedMembers       *MemberSet
	banned               map[string]struct{}
	lastEventId          uint64
	chashByKind          map[string]chash.ConsistentHash
	eventSteam           *eventstream.EventStream
	topologyConsensus    ConsensusHandler
}

func NewMemberList(cluster *Cluster) *MemberList {
	memberList := &MemberList{
		cluster:              cluster,
		members:              emptyMemberSet,
		memberStrategyByKind: make(map[string]MemberStrategy),
		blockedMembers:       emptyMemberSet,
		banned:               map[string]struct{}{},
		eventSteam:           cluster.ActorSystem.EventStream,
	}
	memberList.eventSteam.Subscribe(func(evt interface{}) {

		switch t := evt.(type) {
		case *GossipUpdate:
			if t.Key != "topology" {
				break
			}

			// get banned members from all other member states
			// and merge that with out own banned set
			var topology ClusterTopology
			if err := types.UnmarshalAny(t.Value, &topology); err != nil {
				plog.Warn("could not unpack into ClusterToplogy proto.Message form Any", log.Error(err))
				break
			}
			banned := topology.Banned
			memberList.updateBannedMembers(banned)
		}
	})
	return memberList
}

func (ml *MemberList) updateBannedMembers(members []*Member) {

	ml.mutex.Lock()
	defer ml.mutex.Unlock()

	for _, member := range members {
		ml.banned[member.Id] = struct{}{}
	}
}

func (ml *MemberList) stopMemberList() {
	// ml.cluster.ActorSystem.EventStream.Unsubscribe(ml.membershipSub)
}

func (ml *MemberList) InitializeTopologyConsensus() {

	ml.topologyConsensus = ml.cluster.Gossip.RegisterConsensusCheck("topology", func(any *types.Any) interface{} {

		var topology ClusterTopology
		if unpackErr := types.UnmarshalAny(any, &topology); unpackErr != nil {
			plog.Error("could not unpack topology message", log.Error(unpackErr))
			return nil
		}
		return topology.EventId
	})
}

func (ml *MemberList) TopologyConsensus(ctx context.Context) (uint64, bool) {

	result, ok := ml.topologyConsensus.TryGetConsensus(ctx)
	if ok {
		return result.(uint64), true
	}

	return 0, false
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

func (ml *MemberList) UpdateClusterTopology(members Members) {
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
	ml.blockedMembers = ml.blockedMembers.Union(left)
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

func (ml *MemberList) getTopologyChanges(members Members) (topology *ClusterTopology, unchanged bool, active *MemberSet, joined *MemberSet, left *MemberSet) {
	memberSet := NewMemberSet(members)

	// get active members
	// (this bit means that we will never allow a member that failed a health check to join back in)
	active = memberSet.Except(ml.blockedMembers)

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
	ml.cluster.ActorSystem.EventStream.Publish(&remote.EndpointTerminatedEvent{
		Address: m.Address(),
	})
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

func (ml *MemberList) ContainsMemberID(memberID string) bool {

	_, ok := ml.members[memberID]
	return ok
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
