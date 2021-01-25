package cluster

import (
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
)

func _newClusterForTest(name string) *Cluster {
	actorSystem := actor.NewActorSystem()
	c := New(actorSystem, Configure(name, nil, remote.Configure("127.0.0.1", 0)))
	return c
}

func TestPublishRaceCondition(t *testing.T) {
	actorSystem := actor.NewActorSystem()
	c := New(actorSystem, Configure("mycluster", nil, remote.Configure("127.0.0.1", 0)))
	setupMemberList(c)
	rounds := 1000

	var wg sync.WaitGroup
	wg.Add(2 * rounds)

	go func() {
		for i := 0; i < rounds; i++ {
			actorSystem.EventStream.Publish(TopologyEvent([]*Member{{}, {}}))
			actorSystem.EventStream.Publish(TopologyEvent([]*Member{{}}))
			wg.Done()
		}
	}()

	go func() {
		for i := 0; i < rounds; i++ {
			s := actorSystem.EventStream.Subscribe(func(evt interface{}) {})
			actorSystem.EventStream.Unsubscribe(s)
			wg.Done()
		}
	}()

	if waitTimeout(&wg, 2*time.Second) {
		t.Error("Should not run into a timeout")
	}
}

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

func TestMemberList_UpdateClusterToplogy(t *testing.T) {
	c := _newClusterForTest("test-UpdateClusterToplogy")
	obj := setupMemberList(c)
	dumpMembers := func(list []*Member) {
		t.Logf("members=%d", len(list))
		for _, m := range list {
			t.Logf("\t%s", m.Address())
		}
	}
	_ = dumpMembers
	_sorted := func(tpl *ClusterTopology) {
		_sortMembers := func(list []*Member) {
			sort.Slice(list, func(i, j int) bool {
				return (list)[i].Port < (list)[j].Port
			})
		}
		// dumpMembers(tpl.Members)
		_sortMembers(tpl.Members)
		// dumpMembers(tpl.Members)
		_sortMembers(tpl.Left)
		_sortMembers(tpl.Joined)
	}

	t.Run("init", func(t *testing.T) {
		assert := assert.New(t)
		members := _newTopologyEventForTest(2)
		changes := obj._updateClusterTopoLogy(members, 0)
		_sorted(changes)
		expected := &ClusterTopology{Members: members, Joined: members}
		assert.Equalf(expected, changes, "%s\n%s", expected, changes)
	})

	t.Run("join", func(t *testing.T) {
		assert := assert.New(t)
		members := _newTopologyEventForTest(4)
		changes := obj._updateClusterTopoLogy(members, 0)
		_sorted(changes)
		expected := &ClusterTopology{Members: members, Joined: members[2:4]}
		assert.Equalf(expected, changes, "%s\n%s", expected, changes)
	})

	t.Run("left", func(t *testing.T) {
		assert := assert.New(t)
		members := _newTopologyEventForTest(4)
		changes := obj._updateClusterTopoLogy(members[2:4], 0)
		_sorted(changes)
		expected := &ClusterTopology{Members: members[2:4], Left: members[0:2]}
		assert.Equal(expected, changes)
	})
}

func _newTopologyEventForTest(membersCount int, kinds ...string) TopologyEvent {
	if len(kinds) <= 0 {
		kinds = append(kinds, "kind")
	}
	members := make([]*Member, membersCount)
	for i := 0; i < membersCount; i++ {
		memberId := fmt.Sprintf("memberId-%d", i)
		members[i] = &Member{
			Id:    memberId,
			Host:  "127.0.0.1",
			Port:  int32(i),
			Kinds: kinds,
		}
	}
	return TopologyEvent(members)
}

func TestMemberList_getPartitionMember(t *testing.T) {
	actorSystem := actor.NewActorSystem()
	c := New(actorSystem, Configure("mycluster", nil, remote.Configure("127.0.0.1", 0)))
	obj := setupMemberList(c)

	for _, v := range []int{1, 2, 10, 100, 1000} {
		members := _newTopologyEventForTest(v)
		obj.UpdateClusterTopology(members, 1)

		testName := fmt.Sprintf("member*%d", v)
		t.Run(testName, func(t *testing.T) {
			assert := assert.New(t)

			id := &ClusterIdentity{Identity: "name", Kind: "kind"}
			address := obj.getPartitionMemberV2(id)
			assert.NotEmpty(address)

			id = &ClusterIdentity{Identity: "name", Kind: "nonkind"}
			address = obj.getPartitionMemberV2(id)
			assert.Empty(address)
		})
	}
}

func BenchmarkMemberList_getPartitionMemberV2(b *testing.B) {
	SetLogLevel(log.ErrorLevel)
	actorSystem := actor.NewActorSystem()
	c := New(actorSystem, Configure("mycluster", nil, remote.Configure("127.0.0.1", 0)))
	obj := setupMemberList(c)
	for i, v := range []int{1, 2, 3, 5, 10, 100, 1000, 2000} {
		members := _newTopologyEventForTest(v)
		obj.UpdateClusterTopology(members, uint64(i+1))
		testName := fmt.Sprintf("member*%d", v)
		runtime.GC()

		id := &ClusterIdentity{Identity: fmt.Sprintf("name-%d", rand.Int()), Kind: "kind"}
		b.Run(testName, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				address := obj.getPartitionMemberV2(id)
				if address == "" {
					b.Fatalf("empty address members=%d", v)
				}
			}
		})
	}
}

func TestMemberList_getPartitionMemberV2(t *testing.T) {
	assert := assert.New(t)

	tplg := _newTopologyEventForTest(10)
	c := _newClusterForTest("test-memberlist")
	obj := setupMemberList(c)
	obj.UpdateClusterTopology(tplg, 1)

	assert.Contains(obj.memberStrategyByKind, "kind")
	addr := obj.getPartitionMemberV2(&ClusterIdentity{Kind: "kind", Identity: "name"})
	assert.NotEmpty(addr)

	// consistent
	for i := 0; i < 10; i++ {
		addr2 := obj.getPartitionMemberV2(&ClusterIdentity{Kind: "kind", Identity: "name"})
		assert.NotEmpty(addr2)
		assert.Equal(addr, addr2)
	}
}

func TestMemberList_newMemberStrategies(t *testing.T) {
	assert := assert.New(t)

	c := _newClusterForTest("test-memberslist")
	obj := setupMemberList(c)
	for i, v := range []int{1, 10, 100, 1000} {
		members := _newTopologyEventForTest(v, "kind1", "kind2")
		obj.UpdateClusterTopology(members, uint64(i+1))
		assert.Equal(2, len(obj.memberStrategyByKind))
		assert.Contains(obj.memberStrategyByKind, "kind1")

		assert.Equal(v, len(obj.memberStrategyByKind["kind1"].GetAllMembers()))
		assert.Equal(v, len(obj.memberStrategyByKind["kind2"].GetAllMembers()))
	}

}
