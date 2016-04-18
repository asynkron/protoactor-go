package main

import "fmt"
import "bufio"
import "os"
import "github.com/rogeralsing/goactor/actor"

func main() {
	props := actor.Props(NewParentActor).WithRouter(actor.NewRoundRobinGroupRouter())
	parent := actor.Spawn(props)
	parent.Tell(Hello{Name: "Roger"})
	parent.Tell(Hello{Name: "Go"})
	bufio.NewReader(os.Stdin).ReadString('\n')
}

type Ping struct {
	Sender actor.ActorRef
	Name   string
}
type Pong struct{}
type Hello struct{ Name string }

type ChildActor struct{ messageCount int }

func NewChildActor() actor.Actor {
	return &ChildActor{}
}

func (state *ChildActor) Receive(context *actor.Context) {
	switch msg := context.Message.(type) {
	case actor.Starting:
		fmt.Println("Im starting")
	case Ping:
		fmt.Printf("Hello %v\n", msg.Name)
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

func (state *ParentActor) Receive(context *actor.Context) {
	switch msg := context.Message.(type) {
	case actor.Starting:
		state.Child = context.SpawnChild(actor.Props(NewChildActor))
	case actor.Stopping:
		fmt.Println("stopping")
	case Hello:
		fmt.Printf("Parent got hello %v\n", msg.Name)
		state.Child.Tell(Ping{
			Name:   msg.Name,
			Sender: context.Self,
		})
		context.Become(state.Other)
	}
}

func (state *ParentActor) Other(context *actor.Context) {
	switch context.Message.(type) {
	case Pong:
		fmt.Println("Got pong")
		context.Self.Stop()
	}
}
