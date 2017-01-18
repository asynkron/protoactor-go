package main

import (
	"log"
	"strconv"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/router"
)

type myMessage struct{ i int }

func (m *myMessage) Hash() string {
	return strconv.Itoa(m.i)
}

func main() {

	log.Println("Round robin routing:")
	act := func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *myMessage:
			log.Printf("%v got message %d", context.Self(), msg.i)
		}
	}

	pid := actor.Spawn(router.NewRoundRobinPool(5).WithFunc(act))
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("Random routing:")
	pid = actor.Spawn(router.NewRandomPool(5).WithFunc(act))
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("ConsistentHash routing:")
	pid = actor.Spawn(router.NewConsistentHashPool(5).WithFunc(act))
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("BroadcastPool routing:")
	pid = actor.Spawn(router.NewBroadcastPool(5).WithFunc(act))
	for i := 0; i < 10; i++ {
		pid.Tell(&myMessage{i})
	}
	console.ReadLine()
}
