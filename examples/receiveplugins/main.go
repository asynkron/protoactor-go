package main

import (
	"fmt"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/goconsole"
)

type Hello struct{ Who string }
type HelloActor struct{}

func (state *HelloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func myReceivePlugin(context actor.Context) interface{} {
	message := context.Message()
	fmt.Printf("Received message %v\n", message)

	switch msg := context.Message().(type) {
	case Hello:
		return Hello{
			Who: msg.Who + " Modified",
		}
	}

	return message
}

func main() {
	props := actor.FromInstance(&HelloActor{}).WithReceivePlugin(myReceivePlugin)
	pid := actor.Spawn(props)
	pid.Tell(Hello{Who: "Roger"})
	console.ReadLine()
}
