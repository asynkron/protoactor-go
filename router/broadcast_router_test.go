package router

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/otherview/protoactor-go/actor"
)

func TestBroadcastRouterThreadSafe(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	props := actor.PropsFromFunc(func(c actor.Context) {})

	grp := rootContext.Spawn(NewBroadcastGroup())
	go func() {
		count := 100
		for i := 0; i < count; i++ {
			pid, _ := rootContext.SpawnNamed(props, strconv.Itoa(i))
			rootContext.Send(grp, &AddRoutee{pid})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()
	go func() {
		count := 100
		for c := 0; c < count; c++ {
			rootContext.Send(grp, struct{}{})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()

	wg.Wait()
}
