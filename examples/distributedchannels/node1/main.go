package main

import (
	"fmt"
	"runtime"

	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/examples/distributedchannels/messages"
	"github.com/rogeralsing/gam/remoting"
	"github.com/rogeralsing/goconsole"
)

func newMyMessageSenderChannel() chan<- *messages.MyMessage {
	channel := make(chan *messages.MyMessage)
	remote := actor.NewPID("127.0.0.1:8080", "MyMessage")
	go func() {
		for msg := range channel {
			remote.Tell(msg)
		}
	}()

	return channel
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remoting.Start("127.0.0.1:0")
	channel := newMyMessageSenderChannel()

	for i := 0; i < 10; i++ {
		message := &messages.MyMessage{
			Message: fmt.Sprintf("hello %v", i),
		}
		channel <- message
	}

	console.ReadLine()
}
