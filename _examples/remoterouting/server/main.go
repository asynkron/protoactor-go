package main

import (
	"flag"
	"runtime"

	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/examples/remoterouting/messages"
	"github.com/AsynkronIT/protoactor-go/mailbox"
	"github.com/AsynkronIT/protoactor-go/remote"
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
	switch context.Message().(type) {
	case *messages.Ping:
		context.Respond(&messages.Pong{})
	}
}

func newRemoteActor(name string) actor.Producer {
	return func() actor.Actor {
		return &remoteActor{
			name: name,
		}
	}
}

func newRemote(bind, name string) {
	remote.Start(bind)

	context := actor.EmptyRootContext
	props := actor.
		PropsFromProducer(newRemoteActor(name)).
		WithMailbox(mailbox.Bounded(10000))

	context.SpawnNamed(props, "remote")

	log.Println(name, "Ready")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	flag.Parse()

	newRemote(*flagBind, *flagName)

	console.ReadLine()
}
