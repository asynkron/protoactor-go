package main

import (
	"log"

	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/goconsole"
)

type myMessage struct{}

func main() {
	props := actor.FromFunc(func(context actor.Context) {
		switch context.Message().(type) {
		case myMessage:
			log.Printf("%v got message", context.Self())
		}
	}).WithRouter(actor.NewRoundRobinPoolRouter(10))
	pid := actor.Spawn(props)
	for i := 0; i < 10; i++ {
		pid.Tell(myMessage{})
	}
	console.ReadLine()
}
