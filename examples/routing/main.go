package main

import (
	"log"
	"time"

	"github.com/AsynkronIT/gam/actor"
	router "github.com/AsynkronIT/gam/routers"
	"github.com/AsynkronIT/goconsole"
)

type myMessage struct{ i int }

func main() {
	act := func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *myMessage:
			log.Printf("%v got message %d", context.Self(), msg.i)
		}
	}
	log.Println("Round robin routing:")
	props := actor.FromFunc(act).WithPoolRouter(router.NewRoundRobinPool(5))
	pid := actor.Spawn(props)
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("Random routing:")
	props = actor.FromFunc(act).WithPoolRouter(router.NewRandomPool(5))
	pid = actor.Spawn(props)
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	console.ReadLine()
}
