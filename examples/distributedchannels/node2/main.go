package main

import (
	"log"
	"runtime"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/examples/distributedchannels/messages"
	"github.com/AsynkronIT/protoactor-go/remote"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remote.Start("127.0.0.1:8080")
	// create the channel
	channel := make(chan *messages.MyMessage)

	// create an actor receiving messages and pushing them onto the channel
	props := actor.PropsFromFunc(func(context actor.Context) {
		if msg, ok := context.Message().(*messages.MyMessage); ok {
			channel <- msg
		}
	})

	// define root context
	rootContext := actor.EmptyRootContext

	// spawn
	rootContext.SpawnNamed(props, "MyMessage")

	// consume the channel just like you use to
	go func() {
		for msg := range channel {
			log.Println(msg)
		}
	}()

	console.ReadLine()
}
