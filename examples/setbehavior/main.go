package main

import (
	"fmt"

	"github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
)

type Hello struct{ Who string }
type SetBehaviorActor struct{}

func (state *SetBehaviorActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
		context.SetBehavior(state.Other)
	}
}

func (state *SetBehaviorActor) Other(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("%v, ey we are now handling messages in another behavior", msg.Who)
	}
}

func NewSetBehaviorActor() actor.Actor {
	return &SetBehaviorActor{}
}

func main() {
	props := actor.FromProducer(NewSetBehaviorActor)
	pid := actor.Spawn(props)
	pid.Tell(Hello{Who: "Roger"})
	pid.Tell(Hello{Who: "Roger"})
	console.ReadLine()
}
