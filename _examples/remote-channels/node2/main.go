package main

import (
	"distributedchannels/messages"
	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
)

func main() {
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8080)
	remoting := remote.NewRemote(system, remoteConfig)
	remoting.Start()

	// create the channel
	channel := make(chan *messages.MyMessage)

	// create an actor receiving messages and pushing them onto the channel
	props := actor.PropsFromFunc(func(context actor.Context) {
		if msg, ok := context.Message().(*messages.MyMessage); ok {
			channel <- msg
		}
	})

	// spawn
	_, _ = system.Root.SpawnNamed(props, "MyMessage")

	// consume the channel just like you use to
	go func() {
		for msg := range channel {
			log.Println(msg)
		}
	}()

	_, _ = console.ReadLine()
}
