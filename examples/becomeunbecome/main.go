package main

import (
	"fmt"

	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/goconsole"
)

type Become struct{}
type Hello struct{ Who string }
type BecomeActor struct{}

func (state *BecomeActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
		context.Become(state.Other)
	}
}

func (state *BecomeActor) Other(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("%v, ey we are now handling messages in another behavior", msg.Who)
	}
}

func NewBecomeActor() actor.Actor {
	return &BecomeActor{}
}

func main() {
	pid := actor.Spawn(actor.Props(NewBecomeActor))
	pid.Tell(Hello{Who: "Roger"})
	pid.Tell(Hello{Who: "Roger"})
	console.ReadLine()
}
