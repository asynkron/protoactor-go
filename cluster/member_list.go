package cluster

import (
	"context"
	"log/slog"
	"sync"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/eventstream"
	"github.com/awevoke/protoactor-go/remote"
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
	memberList.eventSteam.Subscribe(func(evt any) {
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

// InitializeTopologyConsensus registers a consensus check for the cluster topology hash.
func (ml *MemberList) InitializeTopologyConsensus() {
	ml.topologyConsensus = ml.cluster.Gossip.RegisterConsensusCheck("topology", func(any *anypb.Any) (uint64, error) {
		var topology ClusterTopology
		if err := any.UnmarshalTo(&topology); err != nil {
			ml.cluster.Logger().Error("could not unpack topology message", slog.Any("error", err))
			return 0, err
		}
		return topology.TopologyHash, nil
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
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
	return ml.members.Len()
}

func (ml *MemberList) Members() *MemberSet {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
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

	// Detect kind changes for members present in both old and new sets.
	// This MUST run BEFORE ml.members is overwritten so we can compare
	// old kinds (from ml.members) against new kinds (from active).
	ml.processKindChangesForStayingMembers(active)

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
	// Note: MemberSet.Equals only compares TopologyHash which is based on
	// member IDs alone. We must also check for kind changes so that a
	// topology update where only a member's Kinds changed is not silently
	// dropped.
	if active.Equals(ml.members) && !ml.hasKindChanges(active) {
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

func (ml *MemberList) BroadcastEvent(message any, includeSelf bool) {
	ml.mutex.RLock()
	members := ml.members
	ml.mutex.RUnlock()

	for _, m := range members.members {
		if !includeSelf && m.Id == ml.cluster.ActorSystem.ID {
			continue
		}

		pid := actor.NewPID(m.Address(), "eventstream")
		ml.cluster.ActorSystem.Root.Send(pid, message)
	}
}

func (ml *MemberList) ContainsMemberID(memberID string) bool {
	ml.mutex.RLock()
	defer ml.mutex.RUnlock()
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

// KindsEqual reports whether two kind slices contain the same elements,
// regardless of order.
func KindsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	aSet := make(map[string]struct{}, len(a))
	for _, k := range a {
		aSet[k] = struct{}{}
	}
	for _, k := range b {
		if _, ok := aSet[k]; !ok {
			return false
		}
	}
	return true
}

// hasKindChanges checks whether any member in the new set has different
// Kinds compared to the same member (by ID) in ml.members.
func (ml *MemberList) hasKindChanges(newActive *MemberSet) bool {
	for _, newM := range newActive.Members() {
		if oldM := ml.members.GetMemberById(newM.Id); oldM != nil {
			if !KindsEqual(oldM.Kinds, newM.Kinds) {
				return true
			}
		}
	}
	return false
}

// processKindChangesForStayingMembers detects and applies Kind changes
// for members present in both old (ml.members) and new (newActive) sets.
// Must be called BEFORE ml.members is overwritten with the new set.
func (ml *MemberList) processKindChangesForStayingMembers(newActive *MemberSet) {
	for _, newM := range newActive.Members() {
		oldM := ml.members.GetMemberById(newM.Id)
		if oldM == nil {
			continue // new member, handled by memberJoin
		}
		if KindsEqual(oldM.Kinds, newM.Kinds) {
			continue // no change
		}
		ml.memberKindsChanged(oldM, newM)
	}
}

func (ml *MemberList) memberKindsChanged(oldMember, newMember *Member) {
	ml.cluster.Logger().Info("Member kinds changed",
		slog.String("member", newMember.Id),
		slog.Any("old", oldMember.Kinds),
		slog.Any("new", newMember.Kinds))

	oldKindSet := make(map[string]struct{}, len(oldMember.Kinds))
	for _, k := range oldMember.Kinds {
		oldKindSet[k] = struct{}{}
	}
	newKindSet := make(map[string]struct{}, len(newMember.Kinds))
	for _, k := range newMember.Kinds {
		newKindSet[k] = struct{}{}
	}

	// Remove member from strategies for removed kinds
	for _, kind := range oldMember.Kinds {
		if _, inNew := newKindSet[kind]; !inNew {
			if strategy, ok := ml.memberStrategyByKind[kind]; ok {
				strategy.RemoveMember(oldMember)
			}
		}
	}

	// Add member to strategies for added kinds
	for _, kind := range newMember.Kinds {
		if _, inOld := oldKindSet[kind]; !inOld {
			if ml.memberStrategyByKind[kind] == nil {
				ml.memberStrategyByKind[kind] = ml.getMemberStrategyByKind(kind)
			}
			ml.memberStrategyByKind[kind].AddMember(newMember)
		}
	}
}
