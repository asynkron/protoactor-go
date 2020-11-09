package main

import (
	"log"
	"runtime"

	"remotebenchmark/messages"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/mailbox"
	"github.com/AsynkronIT/protoactor-go/remote"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() * 1)
	runtime.GC()

	cfg := remote.Configure("127.0.0.1", 8080)
	remote.Start(cfg)
	var sender *actor.PID
	rootContext := actor.System.Root
	props := actor.
		PropsFromFunc(
			func(context actor.Context) {
				switch msg := context.Message().(type) {
				case *messages.StartRemote:
					log.Println("Starting")
					sender = msg.Sender
					context.Respond(&messages.Start{})
				case *messages.Ping:
					context.Send(sender, &messages.Pong{})
				}
			}).
		WithMailbox(mailbox.Bounded(1000000))

	rootContext.SpawnNamed(props, "remote")

	console.ReadLine()
}
