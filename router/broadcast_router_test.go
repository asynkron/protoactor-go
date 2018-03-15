package router

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
)

func TestBroadcastRouterThreadSafe(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	props := actor.FromFunc(func(c actor.Context) {})

	grp := actor.Spawn(NewBroadcastGroup())
	go func() {
		count := 100
		for i := 0; i < count; i++ {
			pid, _ := actor.SpawnNamed(props, strconv.Itoa(i))
			grp.Tell(&AddRoutee{pid})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()
	go func() {
		count := 100
		for c := 0; c < count; c++ {
			grp.Tell(struct{}{})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()

	wg.Wait()
}
