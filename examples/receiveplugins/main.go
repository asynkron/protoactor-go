package main

import (
	"fmt"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/goconsole"
)

type Hello struct{ Who string }

func receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func main() {
	props := actor.FromFunc(receive).WithReceivers(actor.MessageLogging) //using built in plugin
	pid := actor.Spawn(props)
	pid.Tell(Hello{Who: "Roger"})
	console.ReadLine()
}
