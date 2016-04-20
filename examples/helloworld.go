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
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
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
