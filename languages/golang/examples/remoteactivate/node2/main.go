package main

import (
	"runtime"

	"github.com/AsynkronIT/gam/languages/golang/examples/remoteactivate/messages"
	"github.com/AsynkronIT/gam/languages/golang/src/actor"
	"github.com/AsynkronIT/gam/languages/golang/src/remoting"
	"github.com/AsynkronIT/goconsole"
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
	remoting.Register("hello", actor.FromProducer(newHelloActor))
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	remoting.Start("127.0.0.1:8080")

	console.ReadLine()
}
