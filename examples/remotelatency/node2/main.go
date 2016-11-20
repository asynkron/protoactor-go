package main

import (
	"log"
	"runtime"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/examples/remotebenchmark/messages"
	"github.com/AsynkronIT/gam/remoting"
	"github.com/AsynkronIT/goconsole"
)

type remoteActor struct {
	i        int
	messages []*messages.Ping
}

func (state *remoteActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.Ping:
		state.messages[state.i] = msg
		state.i++
		if state.i%50000 == 0 {
			log.Println(state.i)
		}
		if state.i == 1000000 {
			log.Println("Done")
		}
	}
}

func newRemoteActor() actor.Producer {
	return func() actor.Actor {
		return &remoteActor{
			i:        0,
			messages: make([]*messages.Ping, 1000000),
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GC()

	remoting.Start("127.0.0.1:8080")
	props := actor.
		FromProducer(newRemoteActor()).
		WithMailbox(actor.NewBoundedMailbox(1000, 10000))

	actor.SpawnNamed(props, "remote")

	console.ReadLine()
}
