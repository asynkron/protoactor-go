package main

import (
	"fmt"
	"log"
	"time"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
)

type NoInfluence string

func (NoInfluence) NotInfluenceReceiveTimeout() {}

func main() {
	log.Println("Receive timeout test")

	system := actor.NewActorSystem()
	c := 0

	rootContext := system.Root
	props := actor.PropsFromFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *actor.Started:
			context.SetReceiveTimeout(1 * time.Second)

		case *actor.ReceiveTimeout:
			c++
			log.Printf("ReceiveTimeout: %d", c)

		case string:
			log.Printf("received '%s'", msg)
			if msg == "cancel" {
				fmt.Println("Cancelling")
				context.CancelReceiveTimeout()
			}

		case NoInfluence:
			log.Println("received a no-influence message")

		}
	})

	pid := rootContext.Spawn(props)
	for i := 0; i < 6; i++ {
		rootContext.Send(pid, "hello")
		time.Sleep(500 * time.Millisecond)
	}

	log.Println("hit [return] to send no-influence messages")
	_, _ = console.ReadLine()

	for i := 0; i < 6; i++ {
		rootContext.Send(pid, NoInfluence("hello"))
		time.Sleep(500 * time.Millisecond)
	}

	log.Println("hit [return] to send a message to cancel the timeout")
	_, _ = console.ReadLine()
	rootContext.Send(pid, "cancel")

	log.Println("hit [return] to finish")
	_, _ = console.ReadLine()

	rootContext.Stop(pid)
}
