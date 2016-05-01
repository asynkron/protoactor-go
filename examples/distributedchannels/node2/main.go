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

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	remoting.StartServer("127.0.0.1:8080")
	//create the channel
	channel := make(chan *messages.MyMessage)

	//create an actor receiving messages and pushing them onto the channel
	pid := actor.SpawnReceiveFunc(func(context actor.Context) {
		switch msg := context.Message().(type) {
		case *messages.MyMessage:
			channel <- msg
		}
	})
	//expose a known endpoint
	actor.ProcessRegistry.Register("MyMessage", pid)

	//consume the channel just like you use to
	go func() {
		for msg := range channel {
			log.Println(msg)
		}
	}()

	bufio.NewReader(os.Stdin).ReadString('\n')
}
