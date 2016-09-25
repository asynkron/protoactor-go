package main

import (
	"log"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/goconsole"
)

type myMessage struct{ i int }

func main() {
	act := func(context actor.Context) {
		switch context.Message().(type) {
		case myMessage:
			log.Printf("%v got message %d", context.Self(), context.Message().(myMessage).i)
		}
	}
	log.Println("Round robin routing:")
	props := actor.FromFunc(act).WithPoolRouter(actor.NewRoundRobinPool(5))
	pid := actor.Spawn(props)
	log.Println("spawned")
	for i := 0; i < 10; i++ {
		pid.Tell(myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("Random routing:")
	props = actor.FromFunc(act).WithPoolRouter(actor.NewRandomPool(5))
	pid = actor.Spawn(props)
	for i := 0; i < 10; i++ {
		pid.Tell(myMessage{i})
	}
	console.ReadLine()
}
