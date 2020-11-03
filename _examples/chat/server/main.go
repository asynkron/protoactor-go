package main

import (
	"log"

	"chat/messages"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/emirpasic/gods/sets/hashset"
)

// define root context

func notifyAll(context actor.Context, clients *hashset.Set, message interface{}) {
	for _, tmp := range clients.Values() {
		client := tmp.(*actor.PID)
		context.Send(client, message)
	}
}

func main() {
	system := actor.NewActorSystem()
	config := remote.BindTo("127.0.0.1", 8080)
	remoter := remote.NewRemote(system, config)
	remoter.Start()

	clients := hashset.New()

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
