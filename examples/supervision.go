package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/rogeralsing/goactor"
)

type Hello struct{ Who string }
type HelloActor struct{}

func (state *HelloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case actor.Starting:
		fmt.Println("Starting, initialize actor here")
	case actor.Stopping:
		fmt.Println("Stopping, actor is about sto shut down")
	case actor.Stopped:
		fmt.Println("Stopped, actor and it's children are stopped")
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
        panic("Ouch")
	}
}

func NewHelloActor() actor.Actor {
	return &HelloActor{}
}

func main() {
	actor := actor.ActorOf(actor.Props(NewHelloActor))
	actor.Tell(Hello{Who: "Roger"})
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}
