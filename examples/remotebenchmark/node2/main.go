package main

import (
	"log"
	"runtime"

	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/examples/remotebenchmark/messages"
	"github.com/rogeralsing/gam/remoting"
	"github.com/rogeralsing/goconsole"
)

type remoteActor struct{}

func (*remoteActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.StartRemote:
		log.Println("Starting")
		msg.Sender.Tell(&messages.Start{})
	case *messages.Ping:
		msg.Sender.Tell(&messages.Pong{})
	}
}

func newRemoteActor() actor.ActorProducer {
	return func() actor.Actor {
		return &remoteActor{}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	remoting.StartServer("127.0.0.1:8080")
	props := actor.
		FromProducer(newRemoteActor()).
		WithMailbox(actor.NewBoundedMailbox(1000, 10000))

	actor.SpawnNamed(props, "remote")

	console.ReadLine()
}
