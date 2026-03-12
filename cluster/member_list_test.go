package cluster

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPublishRaceCondition(t *testing.T) {
	cp := newInmemoryProvider()
	c := newClusterForTest("mycluster", cp)

	rounds := 1000
	var wg sync.WaitGroup
	wg.Add(2 * rounds)

	go func() {
		for i := 0; i < rounds; i++ {
			c.MemberList.UpdateClusterTopology([]*Member{
				{Id: "1", Host: "localhost", Port: 1},
				{Id: "2", Host: "localhost", Port: 2},
			})
			c.MemberList.UpdateClusterTopology([]*Member{
				{Id: "1", Host: "localhost", Port: 1},
			})
			wg.Done()
		}
	}()

	go func() {
		for i := 0; i < rounds; i++ {
			s := c.ActorSystem.EventStream.Subscribe(func(evt any) {})
			c.ActorSystem.EventStream.Unsubscribe(s)
			wg.Done()
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Error("Should not run into a timeout")
	}
}

func TestMemberList_UpdateClusterTopology(t *testing.T) {
	c := newClusterForTest("test-UpdateClusterTopology", newInmemoryProvider())
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
	c := newClusterForTest("test-UpdateClusterTopology", newInmemoryProvider())

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

func TestMemberList_GetActivatorMember(t *testing.T) {
	t.Parallel()

	c := newClusterForTest("test-activator", newInmemoryProvider())
	obj := NewMemberList(c)

	members := newMembersForTest(3) // these have kind "kind" by default
	obj.UpdateClusterTopology(members)

	t.Run("known kind returns a member", func(t *testing.T) {
		activator := obj.GetActivatorMember("kind", "some-source")
		assert.NotEmpty(t, activator)
	})

	t.Run("round-robin distributes across members", func(t *testing.T) {
		seen := make(map[string]struct{})
		// Call enough times to guarantee all members are visited regardless of counter position
		for i := 0; i < 3*3; i++ {
			addr := obj.GetActivatorMember("kind", "test-identity")
			assert.NotEmpty(t, addr)
			seen[addr] = struct{}{}
		}
		assert.Equal(t, 3, len(seen), "round-robin should distribute across all 3 members")
	})

	t.Run("unknown kind returns empty", func(t *testing.T) {
		activator := obj.GetActivatorMember("nonexistent-kind", "some-source")
		assert.Empty(t, activator)
	})
}

// TestConcurrentMemberListAccess exercises concurrent reads via ContainsMemberID,
// Length, and Members while UpdateClusterTopology is writing. This test is
// designed to trigger the race detector if any of the reader methods fail to
// acquire the mutex before accessing ml.members.
func TestConcurrentMemberListAccess(t *testing.T) {
	t.Parallel()

	c := newClusterForTest("test-concurrent-memberlist", newInmemoryProvider())
	ml := NewMemberList(c)

	const iterations = 500
	var wg sync.WaitGroup

	// Writer goroutine: alternates between two different topologies.
	wg.Add(1)
	go func() {
		defer wg.Done()
		twoMembers := newMembersForTest(2)
		threeMembers := newMembersForTest(3)
		for i := 0; i < iterations; i++ {
			if i%2 == 0 {
				ml.UpdateClusterTopology(twoMembers)
			} else {
				ml.UpdateClusterTopology(threeMembers)
			}
		}
	}()

	// Reader goroutine 1: ContainsMemberID
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = ml.ContainsMemberID("memberId-0")
		}
	}()

	// Reader goroutine 2: Length
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = ml.Length()
		}
	}()

	// Reader goroutine 3: Members
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = ml.Members()
		}
	}()

	// Reader goroutine 4: GetActivatorMember
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = ml.GetActivatorMember("kind", "source")
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed without race detector complaints.
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for concurrent member list access test")
	}
}

func TestMemberList_newMemberStrategies(t *testing.T) {
	t.Parallel()
	a := assert.New(t)

	c := newClusterForTest("test-memberlist", newInmemoryProvider())
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
