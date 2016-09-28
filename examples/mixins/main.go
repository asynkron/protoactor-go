package main

import (
	"fmt"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/plugin"
	"github.com/AsynkronIT/goconsole"
)

type myActor struct {
	pluggableMixin
}

func (state *myActor) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *actor.Started:
		//this actor have been initialized by the receive pipeline
		fmt.Printf("My name is %v\n", state.name)
	}
}

type pluggable interface {
	SetName(name string)
}

type pluggableMixin struct {
	name string
}

func (state *pluggableMixin) SetName(name string) {
	state.name = name
}

func pluggableInitializer(context actor.Context) {
	if p, ok := context.Actor().(pluggable); ok {
		p.SetName("GAM")
	}
}

func main() {
	props := actor.
		FromInstance(&myActor{}).
		WithReceivers(plugin.Use(pluggableInitializer))

	pid := actor.Spawn(props)
	pid.Tell("bar")
	console.ReadLine()
}
