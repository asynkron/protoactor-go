package router_test

import (
	"strconv"
	"sync"
	"testing"
	"time"

	actor "github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/router"
)

type myMessage struct {
	i   int
	pid *actor.PID
}
type getRoutees struct {
	pid *actor.PID
}

func (m *myMessage) Hash() string {
	return strconv.Itoa(m.i)
}

var wait sync.WaitGroup

type routerActor struct{}
type tellerActor struct{}
type managerActor struct {
	set  []*actor.PID
	rpid *actor.PID
}

func (state *routerActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *myMessage:
		// log.Printf("%v got message %d", context.Self(), msg.i)
		msg.i++
		wait.Done()
	}
}
func (state *tellerActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *myMessage:
		for i := 0; i < 100; i++ {
			context.Send(msg.pid, msg)
			time.Sleep(10 * time.Millisecond)
		}

	}
}

func (state *managerActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *router.Routees:
		state.set = msg.PIDs
		for i, v := range state.set {
			if i%2 == 0 {
				context.Send(state.rpid, &router.RemoveRoutee{v})
				// log.Println(v)

			} else {
				props := actor.PropsFromProducer(func() actor.Actor { return &routerActor{} })
				pid := context.Spawn(props)
				context.Send(state.rpid, &router.AddRoutee{pid})
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

	rootContext := actor.EmptyRootContext()

	wait.Add(100 * 10000)
	rpid := rootContext.Spawn(router.NewConsistentHashPool(100).WithProducer(func() actor.Actor { return &routerActor{} }))

	props := actor.PropsFromProducer(func() actor.Actor { return &tellerActor{} })
	for i := 0; i < 10000; i++ {
		pid := rootContext.Spawn(props)
		rootContext.Send(pid, &myMessage{i, rpid})
	}

	props = actor.PropsFromProducer(func() actor.Actor { return &managerActor{} })
	pid := rootContext.Spawn(props)
	rootContext.Send(pid, &getRoutees{rpid})
	wait.Wait()
}
