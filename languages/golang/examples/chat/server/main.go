package main

import (
	"log"
	"runtime"

	"github.com/AsynkronIT/gam/languages/golang/examples/chat/messages"
	"github.com/AsynkronIT/gam/languages/golang/src/actor"
	"github.com/AsynkronIT/gam/languages/golang/src/remoting"
	"github.com/AsynkronIT/goconsole"
	"github.com/emirpasic/gods/sets/hashset"
)

func notifyAll(clients *hashset.Set, message interface{}) {
	for _, tmp := range clients.Values() {
		client := tmp.(*actor.PID)
		client.Tell(message)
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remoting.Start("127.0.0.1:8080")
	clients := hashset.New()
	props := actor.FromFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *messages.Connect:
			log.Printf("Client %v connected", msg.Sender)
			clients.Add(msg.Sender)
			msg.Sender.Tell(&messages.Connected{Message: "Welcome!"})
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
	actor.SpawnNamed(props, "chatserver")
	console.ReadLine()
}
