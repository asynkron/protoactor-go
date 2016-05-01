package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"

	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/examples/distributedchannels/messages"
	"github.com/rogeralsing/gam/remoting"
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
	remoting.StartServer("127.0.0.1:0")
	channel := newMyMessageSenderChannel()

	for i := 0; i < 10; i++ {
		message := &messages.MyMessage{
			Message: fmt.Sprintf("hello %v", i),
		}
		channel <- message
	}

	bufio.NewReader(os.Stdin).ReadString('\n')
}
