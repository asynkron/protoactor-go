package main

import (
	"flag"
	"log"
	"net"
	"runtime"
	"strconv"

	"remoterouting/messages"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

var (
	flagBind = flag.String("bind", "localhost:8100", "Bind to address")
	flagName = flag.String("name", "node1", "Name")

	system  = actor.NewActorSystem()
	context = system.Root
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
	host, _port, err := net.SplitHostPort(bind)
	if err != nil {
		panic(err)
	}
	port, err := strconv.Atoi(_port)
	if err != nil {
		panic(err)
	}

	r := remote.NewRemote(system, remote.Configure(host, port))
	r.Start()

	props := actor.
		PropsFromProducer(newRemoteActor(name),
			actor.WithMailbox(actor.Bounded(10000)))

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
