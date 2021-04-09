package main

import (
	"log"
	"remote-benchmark/messages"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var (
	system      = actor.NewActorSystem()
	rootContext = system.Root
)

func main() {
	// configure the remote with an advertised host address
	cfg := cfg.WithAdvertisedHost("localhost:8081")
	r := remote.NewRemote(system, cfg)
	r.Start()

	// use the advertised address of the remote actor in combination
	// with the remote services name
	remotePid := actor.NewPID("127.0.0.1:8080", "remote")

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
