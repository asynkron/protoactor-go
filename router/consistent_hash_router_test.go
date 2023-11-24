package router_test

import (
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/router"
)

var system = actor.NewActorSystem()

type myMessage struct {
	i   int32
	pid *actor.PID
}

type getRoutees struct {
	pid *actor.PID
}

func (m *myMessage) Hash() string {
	i := atomic.LoadInt32(&m.i)
	return strconv.Itoa(int(i))
}

var wait sync.WaitGroup

type (
	routerActor  struct{}
	tellerActor  struct{}
	managerActor struct {
		set  []*actor.PID
		rpid *actor.PID
	}
)

func (state *routerActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *myMessage:
		//context.Logger().Info("%v got message", slog.Any("self", context.Self()), slog.Int("msg", int(msg.i)))
		atomic.AddInt32(&msg.i, 1)
		wait.Done()
	}
}

func (state *tellerActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *myMessage:
		start := msg.i
		for i := 0; i < 100; i++ {
			context.Send(msg.pid, msg)
			time.Sleep(10 * time.Millisecond)
		}
		if msg.i != start+100 {
			context.Logger().Error("Expected to send 100 messages", slog.Int("start", int(start)), slog.Int("end", int(msg.i)))
		}
	}
}

func (state *managerActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *router.Routees:
		state.set = msg.PIDs
		for i, v := range state.set {
			if i%2 == 0 {
				context.Send(state.rpid, &router.RemoveRoutee{PID: v})
				// log.Println(v)
			} else {
				props := actor.PropsFromProducer(func() actor.Actor { return &routerActor{} })
				pid := context.Spawn(props)
				context.Send(state.rpid, &router.AddRoutee{PID: pid})
				// log.Println(v)
			}
		}
		context.Send(context.Self(), &getRoutees{state.rpid})
	case *getRoutees:
		state.rpid = msg.pid
		context.Request(msg.pid, &router.GetRoutees{})
	}
}

func TestConcurrency(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	wait.Add(100 * 1000)
	rpid := system.Root.Spawn(router.NewConsistentHashPool(100).Configure(actor.WithProducer(func() actor.Actor { return &routerActor{} })))

	props := actor.PropsFromProducer(func() actor.Actor { return &tellerActor{} })
	for i := 0; i < 1000; i++ {
		pid := system.Root.Spawn(props)
		system.Root.Send(pid, &myMessage{int32(i), rpid})
	}

	props = actor.PropsFromProducer(func() actor.Actor { return &managerActor{} })
	pid := system.Root.Spawn(props)
	system.Root.Send(pid, &getRoutees{rpid})

	// Implementing the timeout
	timeout := time.After(5 * time.Second)
	done := make(chan bool)
	go func() {
		wait.Wait()
		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("Test timed out")
	case <-done:
		// Test completed within timeout
	}

}
