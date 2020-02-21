package cluster

import (
	"sync"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/eventstream"
)

func TestPublishRaceCondition(t *testing.T) {
	setupMemberList()
	rounds := 1000

	var wg sync.WaitGroup
	wg.Add(2 * rounds)

	go func() {
		for i := 0; i < rounds; i++ {
			eventstream.Publish(ClusterTopologyEvent([]*MemberStatus{{}, {}}))
			eventstream.Publish(ClusterTopologyEvent([]*MemberStatus{{}}))
			wg.Done()
		}
	}()

	go func() {
		for i := 0; i < rounds; i++ {
			s := eventstream.Subscribe(func(evt interface{}) {})
			eventstream.Unsubscribe(s)
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
