package main

import (
	"bufio"
	"log"
	"os"
	"runtime"

	"github.com/rogeralsing/gam/actor"
	"github.com/rogeralsing/gam/examples/remoting/messages"
	"github.com/rogeralsing/gam/remoting"
)

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

func newRemoteActor() actor.ActorProducer {
	return func() actor.Actor {
		return &remoteActor{}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	// f, err := os.Create("cpuprofile")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()

	remoting.StartServer("127.0.0.1:8080")
	pid := actor.Spawn(actor.Props(newRemoteActor()).WithMailbox(actor.NewBoundedMailbox(1000, 1000)))
	actor.ProcessRegistry.Register("remote", pid)

	// f, err = os.Create("memprof")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.WriteHeapProfile(f)
	// f.Close()

	bufio.NewReader(os.Stdin).ReadString('\n')
}
