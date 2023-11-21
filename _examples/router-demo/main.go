package main

import (
	"log"
	"log/slog"
	"strconv"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/router"
)

type myMessage struct{ i int }

func (m *myMessage) Hash() string {
	return strconv.Itoa(m.i)
}

func main() {
	log.Println("Round robin routing:")
	system := actor.NewActorSystem()
	rootContext := system.Root
	act := func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *myMessage:
			context.Logger().Info("got message", slog.Any("self", context.Self()), slog.Any("message", msg))
		}
	}

	pid := rootContext.Spawn(router.NewRoundRobinPool(5, actor.WithFunc(act)))
	for i := 0; i < 10; i++ {
		rootContext.Send(pid, &myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("Random routing:")
	pid = rootContext.Spawn(router.NewRandomPool(5, actor.WithFunc(act)))
	for i := 0; i < 10; i++ {
		rootContext.Send(pid, &myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("ConsistentHash routing:")
	pid = rootContext.Spawn(router.NewConsistentHashPool(5, actor.WithFunc(act)))
	for i := 0; i < 10; i++ {
		rootContext.Send(pid, &myMessage{i})
	}
	time.Sleep(1 * time.Second)
	log.Println("BroadcastPool routing:")
	pid = rootContext.Spawn(router.NewBroadcastPool(5, actor.WithFunc(act)))
	for i := 0; i < 10; i++ {
		rootContext.Send(pid, &myMessage{i})
	}
	_, _ = console.ReadLine()
}
