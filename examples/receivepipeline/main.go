package main

import (
	"fmt"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/actor/middleware"
)

type hello struct{ Who string }

func receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *hello:
		fmt.Printf("Hello %v\n", msg.Who)
	}
}

func main() {
	rootContext := actor.EmptyRootContext
	props := actor.PropsFromFunc(receive).WithReceiverMiddleware(middleware.Logger)
	pid := rootContext.Spawn(props)
	rootContext.Send(pid, &hello{Who: "Roger"})
	console.ReadLine()
}
