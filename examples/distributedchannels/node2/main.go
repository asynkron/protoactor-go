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

//spawn an inline actor that pushes messages onto the channel
func newMyMessageChannel() <-chan *messages.MyMessage {
	channel := make(chan *messages.MyMessage)
	pid := actor.SpawnReceiveFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *messages.MyMessage:
			channel <- msg
		}
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
