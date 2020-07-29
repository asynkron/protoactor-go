package main

import (
	"runtime"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/examples/remoteactivate/messages"
	"github.com/AsynkronIT/protoactor-go/remote"
)

type helloActor struct{}

func (*helloActor) Receive(ctx actor.Context) {
	switch ctx.Message().(type) {
	case *messages.HelloRequest:
		ctx.Respond(&messages.HelloResponse{
			Message: "Hello from remote node",
		})
	}
}

func newHelloActor() actor.Actor {
	return &helloActor{}
}

func init() {
	remote.Register("hello", actor.PropsFromProducer(newHelloActor))
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	remote.Start("127.0.0.1:8080")

	console.ReadLine()
}
