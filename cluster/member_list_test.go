package cluster

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
	"sync"
	"testing"
	"time"
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
