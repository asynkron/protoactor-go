package main

import (
	"fmt"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/goconsole"
)

type Hello struct{ Who string }
type ParentActor struct{}

func (state *ParentActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		props := actor.FromProducer(NewChildActor)
		child := context.Spawn(props)
		child.Tell(msg)
	}
}

func NewParentActor() actor.Actor {
	return &ParentActor{}
}

type ChildActor struct{}

func (state *ChildActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case actor.Started:
		fmt.Println("Starting, initialize actor here")
	case actor.Stopping:
		fmt.Println("Stopping, actor is about shut down")
	case actor.Stopped:
		fmt.Println("Stopped, actor and it's children are stopped")
	case actor.Restarting:
		fmt.Println("Restarting, actor is about restart")
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
		panic("Ouch")
	}
}

func NewChildActor() actor.Actor {
	return &ChildActor{}
}

func main() {
	decider := func(child *actor.PID, reason interface{}) actor.Directive {
		fmt.Println("handling failure for child")
		return actor.StopDirective
	}
	supervisor := actor.NewOneForOneStrategy(10, 1000, decider)
	props := actor.
		FromProducer(NewParentActor).
		WithSupervisor(supervisor)

	pid := actor.Spawn(props)
	pid.Tell(Hello{Who: "Roger"})

	console.ReadLine()
}
