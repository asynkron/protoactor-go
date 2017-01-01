package main

import (
	"fmt"
	"log"
	"time"

	"github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
)

type NoInfluence string

func (NoInfluence) NotInfluenceReceiveTimeout() {}

func main() {

	log.Println("Reveive timeout test")

	c := 0

	act := actor.FromFunc(func(context actor.Context) {
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

	pid := actor.Spawn(act)
	for i := 0; i < 6; i++ {
		pid.Tell("hello")
		time.Sleep(500 * time.Millisecond)
	}

	log.Println("hit [return] to send no-influence messages")
	console.ReadLine()

	for i := 0; i < 6; i++ {
		pid.Tell(NoInfluence("hello"))
		time.Sleep(500 * time.Millisecond)
	}

	log.Println("hit [return] to send a message to cancel the timeout")
	console.ReadLine()
	pid.Tell("cancel")

	log.Println("hit [return] to finish")
	console.ReadLine()

	pid.Stop()
}
