package main

import (
	"fmt"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
)

type hello struct{ Who string }
type helloActor struct{}

func (state *helloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *actor.Started:
		fmt.Println("Started, initialize actor here")
	case *actor.Stopping:
		fmt.Println("Stopping, actor is about shut down")
	case *actor.Stopped:
		fmt.Println("Stopped, actor and it's children are stopped")
	case *actor.Restarting:
		fmt.Println("Restarting, actor is about restart")
	case *hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func main() {
	rootContext := actor.EmptyRootContext
	props := actor.PropsFromProducer(func() actor.Actor { return &helloActor{} })
	pid := rootContext.Spawn(props)
	rootContext.Send(pid, &hello{Who: "Roger"})

	// why wait?
	// Stop is a system message and is not processed through the user message mailbox
	// thus, it will be handled _before_ any user message
	// we only do this to show the correct order of events in the console
	time.Sleep(1 * time.Second)
	rootContext.Stop(pid)

	console.ReadLine()
}
