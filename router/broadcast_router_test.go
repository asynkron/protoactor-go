package router

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

var system = actor.NewActorSystem()

func TestBroadcastRouterThreadSafe(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	props := actor.PropsFromFunc(func(c actor.Context) {})

	grp := system.Root.Spawn(NewBroadcastGroup())
	go func() {
		count := 100
		for i := 0; i < count; i++ {
			pid, _ := system.Root.SpawnNamed(props, strconv.Itoa(i))
			system.Root.Send(grp, &AddRoutee{PID: pid})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()
	go func() {
		count := 100
		for c := 0; c < count; c++ {
			system.Root.Send(grp, struct{}{})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()

	wg.Wait()
}
