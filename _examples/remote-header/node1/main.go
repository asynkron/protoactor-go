package main

import (
	"log"
	"time"

	"remoteheader/messages"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

var (
	system      = actor.NewActorSystem()
	rootContext = system.Root
)

func main() {
	cfg := remote.Configure("127.0.0.1", 8081)
	r := remote.NewRemote(system, cfg)
	r.Start()

	props := actor.
		PropsFromFunc(func(context actor.Context) {
			switch context.Message().(type) {
			case *messages.Pong:
				v := context.MessageHeader().Get("test_header")
				log.Println("Receive pong message with header:" + v)
			}
		})

	pid := rootContext.Spawn(props)

	remotePid := actor.NewPID("127.0.0.1:8080", "remote")
	rootContext.RequestFuture(remotePid, &messages.StartRemote{
		Sender: pid,
	}, 5*time.Second).
		Wait()

	message := &messages.Ping{}
	rootContext.Send(remotePid, message)

	console.ReadLine()
}
