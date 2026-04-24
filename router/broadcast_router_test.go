package router

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
)

var system = actor.NewActorSystem()

func TestBroadcastRouterThreadSafe(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	props := actor.PropsFromFunc(func(_ actor.Context) {})

	grp := system.Root.Spawn(NewBroadcastGroup())
	go func() {
		count := 100
		for i := 0; i < count; i++ {
			pid, _ := system.Root.SpawnNamed(props, "broadcast-"+strconv.Itoa(i))
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

func TestRandomRouterThreadSafe(t *testing.T) {
	sys := actor.NewActorSystem()

	props := actor.PropsFromFunc(func(_ actor.Context) {})
	pid1 := sys.Root.Spawn(props)
	pid2 := sys.Root.Spawn(props)

	grp := sys.Root.Spawn(NewRandomGroup(pid1, pid2))

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		for i := 0; i < 100; i++ {
			p := sys.Root.Spawn(props)
			sys.Root.Send(grp, &AddRoutee{PID: p})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()
	go func() {
		for c := 0; c < 100; c++ {
			sys.Root.Send(grp, struct{}{})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()

	wg.Wait()
}

func TestRoundRobinRouterThreadSafe(t *testing.T) {
	sys := actor.NewActorSystem()

	props := actor.PropsFromFunc(func(_ actor.Context) {})
	pid1 := sys.Root.Spawn(props)
	pid2 := sys.Root.Spawn(props)

	grp := sys.Root.Spawn(NewRoundRobinGroup(pid1, pid2))

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		for i := 0; i < 100; i++ {
			p := sys.Root.Spawn(props)
			sys.Root.Send(grp, &AddRoutee{PID: p})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()
	go func() {
		for c := 0; c < 100; c++ {
			sys.Root.Send(grp, struct{}{})
			time.Sleep(10 * time.Millisecond)
		}
		wg.Done()
	}()

	wg.Wait()
}
