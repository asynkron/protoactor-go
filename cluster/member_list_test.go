package cluster

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

//func TestPublishRaceCondition(t *testing.T) {
//	actorSystem := actor.NewActorSystem()
//	c := New(actorSystem, Configure("mycluster", nil, nil, remote.Configure("127.0.0.1", 0)))
//	NewMemberList(c)
//	rounds := 1000
//
//	var wg sync.WaitGroup
//	wg.Add(2 * rounds)
//
//	go func() {
//		for i := 0; i < rounds; i++ {
//			actorSystem.EventStream.Publish(TopologyEvent(Members{{}, {}}))
//			actorSystem.EventStream.Publish(TopologyEvent(Members{{}}))
//			wg.Done()
//		}
//	}()
//
//	go func() {
//		for i := 0; i < rounds; i++ {
//			s := actorSystem.EventStream.Subscribe(func(evt interface{}) {})
//			actorSystem.EventStream.Unsubscribe(s)
//			wg.Done()
//		}
//	}()
//
//	if waitTimeout(&wg, 2*time.Second) {
//		t.Error("Should not run into a timeout")
//	}
//}

// https://stackoverflow.com/questions/32840687/timeout-for-waitgroup-wait
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

func TestMemberList_UpdateClusterTopology(t *testing.T) {
	c := newClusterForTest("test-UpdateClusterTopology", nil)
	obj := NewMemberList(c)
	empty := make([]*Member, 0)

	t.Run("init", func(t *testing.T) {
		assert := assert.New(t)
		members := newMembersForTest(2)
		changes, unchanged, actives, _, _ := obj.getTopologyChanges(members)
		assert.False(unchanged)
		expected := &ClusterTopology{TopologyHash: TopologyHash(members), Members: members, Joined: members, Left: empty}
		assert.Equal(expected.TopologyHash, changes.TopologyHash)

		var m1, m2 *MemberSet
		m1 = NewMemberSet(expected.Members)
		m2 = NewMemberSet(changes.Members)
		assert.Equal(m1, m2)

		m1 = NewMemberSet(expected.Joined)
		m2 = NewMemberSet(changes.Joined)
		assert.Equal(m1, m2)

		m1 = NewMemberSet(expected.Left)
		m2 = NewMemberSet(changes.Left)
		assert.Equal(m1, m2)

		// current members
		obj.members = actives
	})

	t.Run("join", func(t *testing.T) {
		assert := assert.New(t)
		assert.Equal(2, obj.members.Len())
		members := newMembersForTest(4)
		changes, unchanged, actives, _, _ := obj.getTopologyChanges(members)
		assert.False(unchanged)
		// _sorted(changes)
		expected := &ClusterTopology{TopologyHash: TopologyHash(members), Members: members, Joined: members[2:4], Left: empty}
		assert.Equal(expected.TopologyHash, changes.TopologyHash)

		var m1, m2 *MemberSet
		m1 = NewMemberSet(expected.Members)
		m2 = NewMemberSet(changes.Members)
		assert.Equal(m1, m2)

		m1 = NewMemberSet(expected.Joined)
		m2 = NewMemberSet(changes.Joined)
		assert.Equal(m1, m2)

		m1 = NewMemberSet(expected.Left)
		m2 = NewMemberSet(changes.Left)
		assert.Equal(m1, m2)

		obj.members = actives
	})

	t.Run("left", func(t *testing.T) {
		assert := assert.New(t)
		assert.Equal(4, obj.members.Len())
		members := newMembersForTest(4)
		changes, _, _, _, _ := obj.getTopologyChanges(members[2:4])
		expected := &ClusterTopology{TopologyHash: TopologyHash(members[2:4]), Members: members[2:4], Joined: empty, Left: members[0:2]}
		assert.Equal(expected.TopologyHash, changes.TopologyHash)

		var m1, m2 *MemberSet
		m1 = NewMemberSet(expected.Members)
		m2 = NewMemberSet(changes.Members)
		assert.Equal(m1, m2)

		m1 = NewMemberSet(expected.Joined)
		m2 = NewMemberSet(changes.Joined)
		assert.Equal(m1, m2)

		m1 = NewMemberSet(expected.Left)
		m2 = NewMemberSet(changes.Left)
		assert.Equal(m1, m2)
	})
}

func newMembersForTest(count int, kinds ...string) Members {
	if len(kinds) == 0 {
		kinds = append(kinds, "kind")
	}
	members := make(Members, count)
	for i := 0; i < count; i++ {
		members[i] = &Member{
			Id:    fmt.Sprintf("memberId-%d", i),
			Host:  "127.0.0.1",
			Port:  int32(i),
			Kinds: kinds,
		}
	}
	return members
}

func TestMemberList_UpdateClusterTopology2(t *testing.T) {
	c := newClusterForTest("test-UpdateClusterTopology", nil)

	obj := NewMemberList(c)
	dumpMembers := func(list Members) {
		t.Logf("membersByMemberId=%d", len(list))

		for _, m := range list {
			t.Logf("\t%s", m.Address())
		}
	}

	empty := make([]*Member, 0)

	_ = dumpMembers
	_sorted := func(tpl *ClusterTopology) {
		_sortMembers := func(list Members) {
			sort.Slice(list, func(i, j int) bool {
				return (list)[i].Port < (list)[j].Port
			})
		}
		_sortMembers(tpl.Members)
		_sortMembers(tpl.Left)
		_sortMembers(tpl.Joined)
	}

	a := assert.New(t)
	members := newMembersForTest(2)
	changes, _, _, _, _ := obj.getTopologyChanges(members) //nolint:dogsled
	_sorted(changes)

	expected := &ClusterTopology{TopologyHash: TopologyHash(members), Members: members, Joined: members, Left: empty}

	a.Equal(expected.TopologyHash, changes.TopologyHash)

	var m1, m2 *MemberSet
	m1 = NewMemberSet(expected.Members)
	m2 = NewMemberSet(changes.Members)
	a.Equal(m1, m2)

	m1 = NewMemberSet(expected.Joined)
	m2 = NewMemberSet(changes.Joined)
	a.Equal(m1, m2)

	m1 = NewMemberSet(expected.Left)
	m2 = NewMemberSet(changes.Left)
	a.Equal(m1, m2)
}

func TestMemberList_getPartitionMember(t *testing.T) {
	t.Parallel()

	c := newClusterForTest("test-memberlist", nil)
	obj := NewMemberList(c)

	for _, v := range []int{1, 2, 10, 100, 1000} {
		members := newMembersForTest(v)
		obj.UpdateClusterTopology(members)

		testName := fmt.Sprintf("member*%d", v)
		t.Run(testName, func(t *testing.T) {
			//assert := assert.New(t)
			//
			//identity := NewClusterIdentity("name", "kind")
			////	address := obj.getPartitionMemberV2(identity)
			////	assert.NotEmpty(address)
			//
			//identity = NewClusterIdentity("name", "nonkind")
			////		address = obj.getPartitionMemberV2(identity)
			////	assert.Empty(address)
		})
	}
}

//func BenchmarkMemberList_getPartitionMemberV2(b *testing.B) {
//	SetLogLevel(log.ErrorLevel)
//	actorSystem := actor.NewActorSystem()
//	c := New(actorSystem, Configure("mycluster", nil, nil, remote.Configure("127.0.0.1", 0)))
//	obj := NewMemberList(c)
//	for i, v := range []int{1, 2, 3, 5, 10, 100, 1000, 2000} {
//		members := _newTopologyEventForTest(v)
//		obj.UpdateClusterTopology(members)
//		testName := fmt.Sprintf("member*%d", v)
//		runtime.GC()
//
//		identity := &ClusterIdentity{Identity: fmt.Sprintf("name-%d", rand.Int()), Kind: "kind"}
//		b.Run(testName, func(b *testing.B) {
//			for i := 0; i < b.N; i++ {
//				address := obj.getPartitionMemberV2(identity)
//				if address == "" {
//					b.Fatalf("empty address membersByMemberId=%d", v)
//				}
//			}
//		})
//	}
//}

//func TestMemberList_getPartitionMemberV2(t *testing.T) {
//	assert := assert.New(t)
//
//	tplg := _newTopologyEventForTest(10)
//	c := _newClusterForTest("test-memberlist")
//	obj := NewMemberList(c)
//	obj.UpdateClusterTopology(tplg, 1)
//
//	assert.Contains(obj.memberStrategyByKind, "kind")
//	addr := obj.getPartitionMemberV2(&ClusterIdentity{Kind: "kind", Identity: "name"})
//	assert.NotEmpty(addr)
//
//	// consistent
//	for i := 0; i < 10; i++ {
//		addr2 := obj.getPartitionMemberV2(&ClusterIdentity{Kind: "kind", Identity: "name"})
//		assert.NotEmpty(addr2)
//		assert.Equal(addr, addr2)
//	}
//}

func TestMemberList_newMemberStrategies(t *testing.T) {
	t.Parallel()
	a := assert.New(t)

	c := newClusterForTest("test-memberlist", nil)
	obj := NewMemberList(c)

	for _, v := range []int{1, 10, 100, 1000} {
		members := newMembersForTest(v, "kind1", "kind2")
		obj.UpdateClusterTopology(members)
		a.Equal(2, len(obj.memberStrategyByKind))
		a.Contains(obj.memberStrategyByKind, "kind1")

		a.Equal(v, len(obj.memberStrategyByKind["kind1"].GetAllMembers()))
		a.Equal(v, len(obj.memberStrategyByKind["kind2"].GetAllMembers()))
	}
}
