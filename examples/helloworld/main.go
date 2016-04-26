package main

import (
	"bufio"
    "os"
    "fmt"
	"github.com/rogeralsing/gam/actor"
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
    pid := actor.Spawn(actor.Props(NewHelloActor))
    pid.Tell(Hello{Who: "Roger"})
    bufio.NewReader(os.Stdin).ReadString('\n')
}