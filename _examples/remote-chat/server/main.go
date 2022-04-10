package main

import (
	"log"

	"chat/messages"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

// define root context

func notifyAll(context actor.Context, clients *actor.PIDSet, message interface{}) {
	for _, client := range clients.Values() {
		context.Send(client, message)
	}
}

func main() {
	system := actor.NewActorSystem()
	config := remote.Configure("127.0.0.1", 8080)
	remoter := remote.NewRemote(system, config)
	remoter.Start()

	clients := actor.NewPIDSet()

	props := actor.PropsFromFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *messages.Connect:
			log.Printf("Client %v connected", msg.Sender)
			clients.Add(msg.Sender)
			context.Send(msg.Sender, &messages.Connected{Message: "Welcome!"})
		case *messages.SayRequest:
			notifyAll(context, clients, &messages.SayResponse{
				UserName: msg.UserName,
				Message:  msg.Message,
			})
		case *messages.NickRequest:
			notifyAll(context, clients, &messages.NickResponse{
				OldUserName: msg.OldUserName,
				NewUserName: msg.NewUserName,
			})
		}
	})

	_, _ = system.Root.SpawnNamed(props, "chatserver")
	_, _ = console.ReadLine()
}
