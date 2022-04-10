package main

import (
	"log"

	"remoteheader/messages"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

var (
	system      = actor.NewActorSystem()
	rootContext = system.Root
)

func main() {
	cfg := remote.Configure("127.0.0.1", 8080)
	r := remote.NewRemote(system, cfg)
	r.Start()

	var sender *actor.PID
	props := actor.
		PropsFromFunc(
			func(context actor.Context) {
				switch msg := context.Message().(type) {
				case *messages.StartRemote:
					log.Println("Starting")
					sender = msg.Sender
					context.Respond(&messages.Start{})
				case *messages.Ping:
					context.Send(sender, &messages.Pong{})
				}
			},
			actor.WithSenderMiddleware(
				func(next actor.SenderFunc) actor.SenderFunc {
					return func(ctx actor.SenderContext, target *actor.PID, envelope *actor.MessageEnvelope) {
						envelope.SetHeader("test_header", "header_from_node2")
						log.Println("set header")
						next(ctx, target, envelope)
					}
				}))

	rootContext.SpawnNamed(props, "remote")

	console.ReadLine()
}
