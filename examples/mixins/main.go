package main

import (
	"fmt"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/plugin"
	"github.com/AsynkronIT/goconsole"
)

type myActor struct {
	NameAwareHolder
}

func (state *myActor) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *actor.Started:
		//this actor have been initialized by the receive pipeline
		fmt.Printf("My name is %v\n", state.name)
	}
}

type NameAware interface {
	SetName(name string)
}

type NameAwareHolder struct {
	name string
}

func (state *NameAwareHolder) SetName(name string) {
	state.name = name
}

type NamerPlugin struct {}
func (p *NamerPlugin) OnStart(ctx actor.Context) {
	if p, ok := ctx.Actor().(NameAware); ok {
		p.SetName("GAM")
	}
}
func (p *NamerPlugin) OnMessage(ctx actor.Context, usrMsg interface{}) {}

func main() {
	props := actor.
		FromInstance(&myActor{}).
		WithReceivers(plugin.Use(&NamerPlugin{}))

	pid := actor.Spawn(props)
	pid.Tell("bar")
	console.ReadLine()
}
