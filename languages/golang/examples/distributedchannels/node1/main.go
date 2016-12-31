package main

import (
	"fmt"
	"runtime"

	"github.com/AsynkronIT/gam/languages/golang/examples/distributedchannels/messages"
	"github.com/AsynkronIT/gam/languages/golang/src/actor"
	"github.com/AsynkronIT/gam/languages/golang/src/remoting"
	"github.com/AsynkronIT/goconsole"
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
