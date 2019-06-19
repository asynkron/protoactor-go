package main

import (
	"fmt"
	"log"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
)

type NoInfluence string

func (NoInfluence) NotInfluenceReceiveTimeout() {}

func main() {
	log.Println("Receive timeout test")

	c := 0

	rootContext := actor.EmptyRootContext
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
				context.SetReceiveTimeout(0)
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
	console.ReadLine()

	for i := 0; i < 6; i++ {
		rootContext.Send(pid, NoInfluence("hello"))
		time.Sleep(500 * time.Millisecond)
	}

	log.Println("hit [return] to send a message to cancel the timeout")
	console.ReadLine()
	rootContext.Send(pid, "cancel")

	log.Println("hit [return] to finish")
	console.ReadLine()

	rootContext.Stop(pid)
}
