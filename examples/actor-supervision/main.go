package main

import (
	"fmt"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
)

type (
	hello       struct{ Who string }
	parentActor struct{}
)

func (state *parentActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *hello:
		props := actor.PropsFromProducer(newChildActor)
		child := context.Spawn(props)
		context.Send(child, msg)
	}
}

func newParentActor() actor.Actor {
	return &parentActor{}
}

type childActor struct{}

func (state *childActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		fmt.Println("Starting, initialize actor here")
	case *actor.Stopping:
		fmt.Println("Stopping, actor is about to shut down")
	case *actor.Stopped:
		fmt.Println("Stopped, actor and its children are stopped")
	case *actor.Restarting:
		fmt.Println("Restarting, actor is about to restart")
	case *hello:
		fmt.Printf("Hello %v\n", msg.Who)
		panic("Ouch")
	}
}

func newChildActor() actor.Actor {
	return &childActor{}
}

func main() {
	system := actor.NewActorSystem()
	decider := func(reason interface{}) actor.Directive {
		fmt.Println("handling failure for child")
		return actor.StopDirective
	}
	supervisor := actor.NewOneForOneStrategy(10, 1000, decider)
	rootContext := system.Root
	props := actor.
		PropsFromProducer(newParentActor,
			actor.WithSupervisor(supervisor))

	pid := rootContext.Spawn(props)
	rootContext.Send(pid, &hello{Who: "Roger"})

	_, _ = console.ReadLine()
}
