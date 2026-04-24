package cluster

import (
	"testing"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKindsEqual_SameKinds(t *testing.T) {
	assert.True(t, KindsEqual([]string{"a", "b"}, []string{"a", "b"}))
}

func TestKindsEqual_DifferentOrder(t *testing.T) {
	assert.True(t, KindsEqual([]string{"b", "a"}, []string{"a", "b"}))
}

func TestKindsEqual_DifferentLength(t *testing.T) {
	assert.False(t, KindsEqual([]string{"a"}, []string{"a", "b"}))
}

func TestKindsEqual_DifferentKinds(t *testing.T) {
	assert.False(t, KindsEqual([]string{"a", "b"}, []string{"a", "c"}))
}

func TestKindsEqual_BothEmpty(t *testing.T) {
	assert.True(t, KindsEqual([]string{}, []string{}))
}

func TestKindsEqual_BothNil(t *testing.T) {
	assert.True(t, KindsEqual(nil, nil))
}

func TestKindsEqual_NilVsEmpty(t *testing.T) {
	assert.True(t, KindsEqual(nil, []string{}))
}

func newTestClusterForMemberList() *Cluster {
	system := actor.NewActorSystem()
	rc := remote.Configure("localhost", 0)
	c := &Cluster{
		ActorSystem: system,
		Config: &Config{
			Name:                   "test-cluster",
			ClusterContextProducer: newDefaultClusterContext,
			MemberStrategyBuilder:  newDefaultMemberStrategy,
			PubSubConfig:           newPubSubConfig(),
		},
		Remote: remote.NewRemote(system, rc),
		kinds:  map[string]*ActivatedKind{},
	}
	c.MemberList = NewMemberList(c)
	return c
}

func TestMemberList_KindChange_Detected(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	// Register kinds so getMemberStrategyByKind can find them
	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
		NewKind("kindB", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
	)

	// Initial topology: member1 has kindA
	initialMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(initialMembers)

	// Verify member1 is in kindA strategy
	ml.mutex.RLock()
	stratA := ml.memberStrategyByKind["kindA"]
	ml.mutex.RUnlock()
	require.NotNil(t, stratA)
	assert.Len(t, stratA.GetAllMembers(), 1)

	// Update: member1 now has kindA AND kindB (same member ID, different kinds)
	updatedMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA", "kindB"}},
	}
	ml.UpdateClusterTopology(updatedMembers)

	// Verify member1 is now also in kindB strategy
	ml.mutex.RLock()
	stratB := ml.memberStrategyByKind["kindB"]
	ml.mutex.RUnlock()
	require.NotNil(t, stratB, "kindB strategy should exist after kind change")
	assert.Len(t, stratB.GetAllMembers(), 1, "member1 should be in kindB strategy")
}

func TestMemberList_KindChange_RemovedKind(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
		NewKind("kindB", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
	)

	// Initial: member1 has kindA and kindB
	initialMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA", "kindB"}},
	}
	ml.UpdateClusterTopology(initialMembers)

	ml.mutex.RLock()
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), 1)
	assert.Len(t, ml.memberStrategyByKind["kindB"].GetAllMembers(), 1)
	ml.mutex.RUnlock()

	// Update: member1 drops kindB
	updatedMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(updatedMembers)

	ml.mutex.RLock()
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), 1, "kindA should still have member1")
	assert.Len(t, ml.memberStrategyByKind["kindB"].GetAllMembers(), 0, "kindB should no longer have member1")
	ml.mutex.RUnlock()
}

func TestMemberList_KindChange_NoChangeIsNoop(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
	)

	members := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(members)

	ml.mutex.RLock()
	stratA := ml.memberStrategyByKind["kindA"]
	memberCountBefore := len(stratA.GetAllMembers())
	ml.mutex.RUnlock()

	// Call again with same data -- no change expected
	ml.UpdateClusterTopology(members)

	ml.mutex.RLock()
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), memberCountBefore,
		"strategy should not have duplicate members after no-op update")
	ml.mutex.RUnlock()
}

func TestMemberList_KindChange_WithJoinAndLeave(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
		NewKind("kindB", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
		NewKind("kindC", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
	)

	// Initial: member1 (kindA), member2 (kindA)
	initialMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
		{Id: "member2", Host: "h2", Port: 2, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(initialMembers)

	// Update: member1 adds kindB (kind change), member2 leaves, member3 joins with kindC
	updatedMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA", "kindB"}},
		{Id: "member3", Host: "h3", Port: 3, Kinds: []string{"kindC"}},
	}
	ml.UpdateClusterTopology(updatedMembers)

	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	// member1: still in kindA, now also in kindB
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), 1, "only member1 in kindA")
	assert.Len(t, ml.memberStrategyByKind["kindB"].GetAllMembers(), 1, "member1 added to kindB")

	// member3: in kindC
	assert.Len(t, ml.memberStrategyByKind["kindC"].GetAllMembers(), 1, "member3 in kindC")
}

func TestMemberList_KindChange_StrategyMembersCorrect(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
		NewKind("kindB", actor.PropsFromProducer(func() actor.Actor { return &testActor{} })),
	)

	// Two members, both with kindA
	initialMembers := Members{
		{Id: "m1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
		{Id: "m2", Host: "h2", Port: 2, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(initialMembers)

	// m1 adds kindB, m2 drops kindA and adds kindB
	updatedMembers := Members{
		{Id: "m1", Host: "h1", Port: 1, Kinds: []string{"kindA", "kindB"}},
		{Id: "m2", Host: "h2", Port: 2, Kinds: []string{"kindB"}},
	}
	ml.UpdateClusterTopology(updatedMembers)

	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	// kindA: only m1
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), 1)
	// kindB: both m1 and m2
	assert.Len(t, ml.memberStrategyByKind["kindB"].GetAllMembers(), 2)
}
