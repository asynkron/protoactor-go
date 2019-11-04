package main

import (
	"fmt"
	"runtime"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/examples/distributedchannels/messages"
	"github.com/otherview/protoactor-go/remote"
)

// define root context
var rootContext = actor.EmptyRootContext

func newMyMessageSenderChannel() chan<- *messages.MyMessage {
	channel := make(chan *messages.MyMessage)
	remote := actor.NewPID("127.0.0.1:8080", "MyMessage")
	go func() {
		for msg := range channel {
			rootContext.Send(remote, msg)
		}
	}()

	return channel
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remote.Start("127.0.0.1:0")
	channel := newMyMessageSenderChannel()

	for i := 0; i < 10; i++ {
		message := &messages.MyMessage{
			Message: fmt.Sprintf("hello %v", i),
		}
		channel <- message
	}

	console.ReadLine()
}
