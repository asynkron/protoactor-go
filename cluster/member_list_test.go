package cluster

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
)

func TestPublishRaceCondition(t *testing.T) {
	actorSystem := actor.NewActorSystem()
	c := New(actorSystem, Configure("mycluster", nil, remote.Configure("127.0.0.1", 0)))
	setupMemberList(c)
	rounds := 1000

	var wg sync.WaitGroup
	wg.Add(2 * rounds)

	go func() {
		for i := 0; i < rounds; i++ {
			actorSystem.EventStream.Publish(TopologyEvent([]*MemberStatus{{}, {}}))
			actorSystem.EventStream.Publish(TopologyEvent([]*MemberStatus{{}}))
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

func Test_getPartitionMember(t *testing.T) {
	assert := assert.New(t)

	actorSystem := actor.NewActorSystem()
	c := New(actorSystem, Configure("mycluster", nil, remote.Configure("127.0.0.1", 0)))
	memberList := setupMemberList(c)
	members := []*MemberStatus{
		{MemberID: "1", Host: "127.0.0.1", Port: 1, Kinds: []string{}},
		{MemberID: "2", Host: "127.0.0.1", Port: 2, Kinds: []string{}},
		{MemberID: "3", Host: "127.0.0.1", Port: 3, Kinds: []string{"kind"}},
	}
	actorSystem.EventStream.Publish(TopologyEvent(members))
	address := memberList.getPartitionMember("name", "kind")
	assert.NotEmpty(address)
}

func _newTopologyEventForTest(membersCount int) TopologyEvent {
	members := make([]*MemberStatus, membersCount)
	for i := 0; i < membersCount; i++ {
		memberId := fmt.Sprintf("memberId-%d", i)
		members[i] = &MemberStatus{
			MemberID: memberId,
			Host:     "127.0.0.1",
			Port:     i,
			Kinds:    []string{"kind"},
			Alive:    true,
		}
	}
	return TopologyEvent(members)
}

func Test_getPartitionMember_WithTopologyEvent(t *testing.T) {
	actorSystem := actor.NewActorSystem()
	c := New(actorSystem, Configure("mycluster", nil, remote.Configure("127.0.0.1", 0)))
	memberList := setupMemberList(c)
	for _, v := range []int{1, 2, 10, 100, 1000} {
		res := _newTopologyEventForTest(v)
		actorSystem.EventStream.Publish(res)
		testName := fmt.Sprintf("member*%d", v)
		t.Run(testName, func(t *testing.T) {
			assert := assert.New(t)
			address := memberList.getPartitionMember("name", "kind")
			assert.NotEmpty(address)
		})
	}
}

func Benchmark_getPartitionMember(b *testing.B) {
	actorSystem := actor.NewActorSystem()
	c := New(actorSystem, Configure("mycluster", nil, remote.Configure("127.0.0.1", 0)))
	memberList := setupMemberList(c)
	for _, v := range []int{1, 2, 3, 5, 10, 100, 1000, 2000} {
		res := _newTopologyEventForTest(v)
		actorSystem.EventStream.Publish(res)
		testName := fmt.Sprintf("member*%d", v)
		runtime.GC()
		b.Run(testName, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				address := memberList.getPartitionMember("name", "kind")
				if address == "" {
					b.Fatalf("empty address res=%d", len(res))
				}
			}
		})
	}
}
