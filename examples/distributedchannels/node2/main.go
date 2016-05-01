package main

import (
	"bufio"
	"log"
	"os"
	"runtime"

	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/examples/distributedchannels/messages"
	"github.com/rogeralsing/gam/remoting"
)

//actor state has ref to the receiving channel
type myMessageChannelReceiver struct {
	channel chan<- *messages.MyMessage
}

func (state *myMessageChannelReceiver) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.MyMessage:
		state.channel <- msg
	}
}

func newMyMessageChannel() <-chan *messages.MyMessage {
	channel := make(chan *messages.MyMessage)
	pid := actor.SpawnTemplate(&myMessageChannelReceiver{
		channel: channel,
	})
	actor.ProcessRegistry.Register("MyMessage", pid)
	return channel
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remoting.StartServer("127.0.0.1:8080")
	channel := newMyMessageChannel()

	go func() {
		for msg := range channel {
			log.Println(msg)
		}
	}()

	bufio.NewReader(os.Stdin).ReadString('\n')
}
