package main

import (
	"bufio"
	"fmt"
	"os"

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

func main() {
	pid := actor.SpawnTemplate(&HelloActor{})
	pid.Tell(Hello{Who: "Roger"})
	bufio.NewReader(os.Stdin).ReadString('\n')
}
