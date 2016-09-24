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

//Custom plugin
// func ConsoleLogging(context actor.Context) {
// 	message := context.Message()
// 	fmt.Printf("Before message %v\n", message)
// 	switch msg := context.Message().(type) {
// 	case Hello:
// 		context.Handle(Hello{Who: msg.Who + "Modified"})
// 	default:
// 		context.Handle(msg)
// 	}
// 	fmt.Printf("After message %v\n", message)
// }

func main() {
	props := actor.FromFunc(receive).WithReceivePlugin(actor.MessageLogging) //using built in plugin
	pid := actor.Spawn(props)
	pid.Tell(Hello{Who: "Roger"})
	console.ReadLine()
}
