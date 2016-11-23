package main

import (
	"flag"
	"runtime"

	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/examples/remoterouting/messages"
	"github.com/AsynkronIT/gam/remoting"
	console "github.com/AsynkronIT/goconsole"
)

var (
	flagBind = flag.String("bind", "localhost:8100", "Bind to address")
	flagName = flag.String("name", "node1", "Name")
)

type remoteActor struct {
	name  string
	count int
}

func (a *remoteActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.Ping:
		a.count++
		if a.count%10000 == 0 {
			log.Println(a.name, "got", a.count, "messages")
		}
		msg.Sender.Tell(&messages.Pong{})
	}
}

func newRemoteActor(name string) actor.Producer {
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
