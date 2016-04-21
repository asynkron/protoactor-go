package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/rogeralsing/gam"
)

type Hello struct{ Who string }
type ParentActor struct{}

func (state *ParentActor) Receive(context gam.Context) {
	switch msg := context.Message().(type) {
	case Hello:
		child := context.SpawnTemplate(&ChildActor{})
		child.Tell(msg)
	}
}

type ChildActor struct{}

func (state *ChildActor) Receive(context gam.Context) {
	switch msg := context.Message().(type) {
	case gam.Started:
		fmt.Println("Starting, initialize actor here")
	case gam.Stopping:
		fmt.Println("Stopping, actor is about shut down")
	case gam.Stopped:
		fmt.Println("Stopped, actor and it's children are stopped")
	case gam.Restarting:
		fmt.Println("Restarting, actor is about restart")
	case Hello:
		fmt.Printf("Hello %v\n", msg.Who)
		panic("Ouch")
	}
}

func main() {
	pid := gam.SpawnTemplate(&ParentActor{})
	pid.Tell(Hello{Who: "Roger"})
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}
