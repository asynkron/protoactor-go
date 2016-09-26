package main

import (
	"fmt"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/goconsole"
)

type hello struct{ Who string }

func receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func main() {
	props := actor.FromFunc(receive).WithReceivers(actor.MessageLogging) //using built in plugin
	pid := actor.Spawn(props)
	pid.Tell(&hello{Who: "Roger"})
	console.ReadLine()
}
