package cluster

import (
	"context"
	"log/slog"
	"sync"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/eventstream"
	"github.com/asynkron/protoactor-go/remote"
	"google.golang.org/protobuf/types/known/anypb"
)

// MemberList is responsible to keep track of the current cluster topology
// it does so by listening to changes from the ClusterProvider.
// the default ClusterProvider is consul.ConsulProvider which uses the Consul HTTP API to scan for changes
type MemberList struct {
	cluster              *Cluster
	mutex                sync.RWMutex
	members              *MemberSet
	memberStrategyByKind map[string]MemberStrategy

	eventSteam        *eventstream.EventStream
	topologyConsensus ConsensusHandler
}

func NewMemberList(cluster *Cluster) *MemberList {
	memberList := &MemberList{
		cluster:              cluster,
		members:              emptyMemberSet,
		memberStrategyByKind: make(map[string]MemberStrategy),
		eventSteam:           cluster.ActorSystem.EventStream,
	}
	memberList.eventSteam.Subscribe(func(evt interface{}) {
		switch t := evt.(type) {
		case *GossipUpdate:
			if t.Key != "topology" {
				break
			}

			// get blocked members from all other member states
			// and merge that without own blocked set
			var topology ClusterTopology
			if err := t.Value.UnmarshalTo(&topology); err != nil {
				cluster.Logger().Warn("could not unpack into ClusterTopology proto.Message form Any", slog.Any("error", err))

				break
			}
			blocked := topology.Blocked
			memberList.cluster.Remote.BlockList().Block(blocked...)
		}
	})

	return memberList
}

func (ml *MemberList) stopMemberList() {
	// ml.cluster.ActorSystem.EventStream.Unsubscribe(ml.membershipSub)
}

func (ml *MemberList) InitializeTopologyConsensus() {
	ml.topologyConsensus = ml.cluster.Gossip.RegisterConsensusCheck("topology", func(any *anypb.Any) interface{} {
		var topology ClusterTopology
		if unpackErr := any.UnmarshalTo(&topology); unpackErr != nil {
			ml.cluster.Logger().Error("could not unpack topology message", slog.Any("error", unpackErr))

			return nil
		}

		return topology.TopologyHash
	})
}

func (ml *MemberList) TopologyConsensus(ctx context.Context) (uint64, bool) {
	result, ok := ml.topologyConsensus.TryGetConsensus(ctx)
	if ok {
		res, _ := result.(uint64)

		return res, true
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
	// this method basically filters out any member status in the blocked list
	// then makes a delta between new and old members
	// notifying the cluster accordingly which members left or joined

	topology, done, active, joined, left := ml.getTopologyChanges(members)
	if done {
		return
	}

	// include any new blocked members into the known set of blocked members
	for _, m := range left.Members() {
		ml.cluster.Remote.BlockList().Block(m.Id)
	}

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

	ml.cluster.Logger().Info("Updated ClusterTopology",
		slog.Uint64("topology-hash", topology.TopologyHash),
		slog.Int("members", len(topology.Members)),
		slog.Int("joined", len(topology.Joined)),
		slog.Int("left", len(topology.Left)),
		slog.Int("blocked", len(topology.Blocked)),
		slog.Int("membersFromProvider", len(members)))
}

func (ml *MemberList) memberJoin(joiningMember *Member) {
	ml.cluster.Logger().Info("member joined", slog.String("member", joiningMember.Id))

	for _, kind := range joiningMember.Kinds {
		if ml.memberStrategyByKind[kind] == nil {
			ml.memberStrategyByKind[kind] = ml.getMemberStrategyByKind(kind)
		}

		ml.memberStrategyByKind[kind].AddMember(joiningMember)
	}
}

func (ml *MemberList) memberLeave(leavingMember *Member) {
	for _, kind := range leavingMember.Kinds {
		if ml.memberStrategyByKind[kind] == nil {
			continue
		}

		ml.memberStrategyByKind[kind].RemoveMember(leavingMember)
	}
}

func (ml *MemberList) getTopologyChanges(members Members) (topology *ClusterTopology, unchanged bool, active *MemberSet, joined *MemberSet, left *MemberSet) {
	memberSet := NewMemberSet(members)

	// get active members
	// (this bit means that we will never allow a member that failed a health check to join back in)
	blocked := ml.cluster.GetBlockedMembers().ToSlice()

	active = memberSet.ExceptIds(blocked)

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
		if !includeSelf && m.Id == ml.cluster.ActorSystem.ID {
			continue
		}

		pid := actor.NewPID(m.Address(), "eventstream")
		ml.cluster.ActorSystem.Root.Send(pid, message)
	}
}

func (ml *MemberList) ContainsMemberID(memberID string) bool {
	return ml.members.ContainsID(memberID)
}

func (ml *MemberList) getMemberStrategyByKind(kind string) MemberStrategy {
	ml.cluster.Logger().Info("creating member strategy", slog.String("kind", kind))

	clusterKind, ok := ml.cluster.TryGetClusterKind(kind)

	if ok {
		if clusterKind.Strategy != nil {
			return clusterKind.Strategy
		}
	}

	strategy := ml.cluster.Config.MemberStrategyBuilder(ml.cluster, kind)
	if strategy != nil {
		return strategy
	}

	return newDefaultMemberStrategy(ml.cluster, kind)
}
