package main

import (
	"log"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/examples/remotebenchmark/messages"
	"github.com/otherview/protoactor-go/remote"
)

func main() {
	remote.Start("127.0.0.1:8081")

	rootContext := actor.EmptyRootContext
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
