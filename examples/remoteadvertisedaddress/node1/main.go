package main

import (
	"log"
	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/examples/remotebenchmark/messages"
	"github.com/otherview/protoactor-go/remote"
)

func main() {
	remote.Start("127.0.0.1:8081", remote.WithAdvertisedAddress("localhost:8081"))
	remotePid := actor.NewPID("127.0.0.1:8080", "remote")

	rootContext := actor.EmptyRootContext
	props := actor.
		PropsFromFunc(func(context actor.Context) {
			switch context.Message().(type) {
			case *actor.Started:
				message := &messages.Ping{}
				context.Request(remotePid, message)

			case *messages.Pong:
				log.Println("Received pong from sender")
			}
		})

	rootContext.Spawn(props)



	console.ReadLine()
}
