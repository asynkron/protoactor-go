package main

import (
	"bufio"
	"fmt"
	"os"
)
import "github.com/rogeralsing/goactor/actor"

func main() {
	props := actor.
		Props(NewParentActor).
		WithMailbox(actor.NewUnboundedMailbox()).
		WithSupervisor(actor.DefaultStrategy())

	parent := actor.Spawn(props)
	parent.Tell(Hello{Name: "Roger"})
	parent.Tell(Hello{Name: "Go"})

	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}

type Ping struct{ Sender actor.ActorRef }
type Pong struct{}
type Hello struct{ Name string }

type ChildActor struct{ messageCount int }

func NewChildActor() actor.Actor {
	return &ChildActor{}
}

func (state *ChildActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Ping:
		state.messageCount++
		fmt.Printf("message count %v \n", state.messageCount)
		msg.Sender.Tell(Pong{})
	}
}

type ParentActor struct {
	Child actor.ActorRef
}

func NewParentActor() actor.Actor {
	return &ParentActor{}
}

func (state *ParentActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case actor.Starting:
		state.Child = context.SpawnChild(actor.Props(NewChildActor))
	case Hello:
		fmt.Printf("Parent got hello %v\n", msg.Name)
		state.Child.Tell(Ping{Sender: context.Self()})
	}
}
