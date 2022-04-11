package main

import (
	"fmt"

	"remoteadvertisedaddress/messages"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

var (
	system      = actor.NewActorSystem()
	rootContext = system.Root
)

func main() {
	cfg := remote.Configure("0.0.0.0", 8080, remote.WithAdvertisedHost("localhost:8080"))
	r := remote.NewRemote(system, cfg)
	r.Start()

	props := actor.
		PropsFromFunc(
			func(context actor.Context) {
				switch context.Message().(type) {
				case *messages.Ping:
					fmt.Println("Received ping from sender with address: " + context.Sender().Address)
					context.Respond(&messages.Pong{})
				}
			})

	rootContext.SpawnNamed(props, "remote")

	console.ReadLine()
}
