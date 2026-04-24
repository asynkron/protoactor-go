package main

import (
	"fmt"

	"remoteadvertisedaddress/messages"

	console "github.com/asynkron/goconsole"
	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/remote"
)

var (
	system      = actor.NewActorSystem()
	rootContext = system.Root
)

func main() {
	cfg := remote.Configure("0.0.0.0", 8080, remote.WithAdvertisedHost("localhost:8080"))
	r := remote.NewRemote(system, cfg)
	if err := r.Start(); err != nil {
		panic(err)
	}

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
