package main

import (
	"log"
	"strconv"
	"time"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/routing"
	"github.com/AsynkronIT/goconsole"
)

type myMessage struct{ i int }

func (m *myMessage) HashBy() string {
	return strconv.Itoa(m.i)
}

func main() {

	log.Println("Round robin routing:")
	act := actor.FromFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *myMessage:
			log.Printf("%v got message %d", context.Self(), msg.i)
		}
	})
	props := act.WithPoolRouter(routing.NewRoundRobinPool(5))
	pid := actor.Spawn(props)
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("Random routing:")
	props = act.WithPoolRouter(routing.NewRandomPool(5))
	pid = actor.Spawn(props)
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("ConsistentHash routing:")
	props = act.WithPoolRouter(routing.NewConsistentHashPool(5))
	pid = actor.Spawn(props)
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("BroadcastPool routing:")
	props = act.WithPoolRouter(routing.NewBroadcastPool(5))
	pid = actor.Spawn(props)
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	console.ReadLine()
}
