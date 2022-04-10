package main

import (
	"fmt"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/actor/middleware"
)

type hello struct{ Who string }

func receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func main() {
	system := actor.NewActorSystem()
	rootContext := system.Root
	props := actor.PropsFromFunc(receive, actor.WithReceiverMiddleware(middleware.Logger))
	pid := rootContext.Spawn(props)
	rootContext.Send(pid, &hello{Who: "Roger"})
	_, _ = console.ReadLine()
}
