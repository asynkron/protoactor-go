package main

import "github.com/rogeralsing/gam/actor"
import "github.com/rogeralsing/gam/remoting"
import "bufio"
import "os"
import "github.com/rogeralsing/gam/examples/remoting/messages"
import "runtime"
import "log"

type remoteActor struct{}

func (*remoteActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.StartRemote:
		log.Println("Starting")
		msg.Sender.Tell(&messages.Start{})	
	case *messages.Ping:
		msg.Sender.Tell(&messages.Pong{})
	}	
}

func main() {
	runtime.GOMAXPROCS(8)
	remoting.StartServer("localhost:8091")
	pid := actor.SpawnTemplate(&remoteActor{})
	actor.ProcessRegistry.Register("remote", pid)

	bufio.NewReader(os.Stdin).ReadString('\n')
}
