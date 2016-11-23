package main

import (
	"flag"
	"runtime"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/remoting"
	console "github.com/AsynkronIT/goconsole"
)

var (
	flagBind = flag.String("bind", "localhost:8100", "Bind to address")
	flagName = flag.String("name", "node1", "Name")
)

type remoteActor struct {
	name string
}

func (a *remoteActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *model.StartRemote:
		log.Println("Starting")
		msg.Sender.Tell(&model.Start{})
	case *model.Ping:
		log.Println(a.name, "got message")
		msg.Sender.Tell(&model.Pong{})
	}
}

func newRemoteActor(name string) actor.ActorProducer {
	return func() actor.Actor {
		return &remoteActor{
			name: name,
		}
	}
}

func NewRemote(bind, name string) {
	remoting.Start(bind)
	props := actor.
		FromProducer(newRemoteActor(name)).
		WithMailbox(actor.NewBoundedMailbox(1000, 10000))

	actor.SpawnNamed(props, "remote")

	log.Println(name, "Ready")
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	flag.Parse()

	NewRemote(*flagBind, *flagName)

	console.ReadLine()
}
