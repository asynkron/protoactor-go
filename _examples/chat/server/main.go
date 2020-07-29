package main

import (
	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/examples/chat/messages"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/emirpasic/gods/sets/hashset"
)

// define root context
var rootContext = actor.EmptyRootContext

func notifyAll(clients *hashset.Set, message interface{}) {
	for _, tmp := range clients.Values() {
		client := tmp.(*actor.PID)
		rootContext.Send(client, message)
	}
}

func main() {
	remote.Start("127.0.0.1:8080")
	clients := hashset.New()

	props := actor.PropsFromFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *messages.Connect:
			log.Printf("Client %v connected", msg.Sender)
			clients.Add(msg.Sender)
			context.Send(msg.Sender, &messages.Connected{Message: "Welcome!"})
		case *messages.SayRequest:
			notifyAll(clients, &messages.SayResponse{
				UserName: msg.UserName,
				Message:  msg.Message,
			})
		case *messages.NickRequest:
			notifyAll(clients, &messages.NickResponse{
				OldUserName: msg.OldUserName,
				NewUserName: msg.NewUserName,
			})
		}
	})

	rootContext.SpawnNamed(props, "chatserver")
	console.ReadLine()
}
