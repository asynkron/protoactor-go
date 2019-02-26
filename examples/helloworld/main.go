package main

import (
	"fmt"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
)

type hello struct{ Who string }
type helloActor struct{}

func (state *helloActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func main() {
	props := actor.PropsFromProducer(func() actor.Actor { return &helloActor{} })
	rootContext := actor.EmptyRootContext
	pid := rootContext.Spawn(props)
	rootContext.Send(pid, &hello{Who: "Roger"})
	console.ReadLine()
}
