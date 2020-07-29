package main

import (
	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/examples/remotebenchmark/messages"
	"github.com/AsynkronIT/protoactor-go/remote"
)

func main() {
	remote.Start("127.0.0.1:8080")
	var sender *actor.PID
	rootContext := actor.EmptyRootContext
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
			}).
		WithSenderMiddleware(
			func(next actor.SenderFunc) actor.SenderFunc {
				return func(ctx actor.SenderContext, target *actor.PID, envelope *actor.MessageEnvelope) {
					envelope.SetHeader("test_header", "header_from_node2")
					log.Println("set header")
					next(ctx, target, envelope)
				}
			})

	rootContext.SpawnNamed(props, "remote")

	console.ReadLine()
}
