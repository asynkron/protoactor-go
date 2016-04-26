package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/rogeralsing/gam/actor"
)

type Hello struct{ Who string }
type ParentActor struct{}

func (state *ParentActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		child := context.Spawn(actor.Props(NewChildActor))
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
	pid := actor.Spawn(actor.Props(NewParentActor).WithSupervisor(supervisor))
	pid.Tell(Hello{Who: "Roger"})

	bufio.NewReader(os.Stdin).ReadString('\n')
}
