package main

import (
	"log"

	"remotebenchmark/messages"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

type echoActor struct {
	sender *actor.PID
}

func (state *echoActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *messages.StartRemote:
		log.Printf("Starting for %s", msg.Sender)
		state.sender = msg.Sender
		context.Respond(&messages.Start{})
	case *messages.Ping:
		context.Send(state.sender, &messages.Pong{})
	}
}

func main() {
	// runtime.GOMAXPROCS(runtime.NumCPU() * 1)
	// runtime.GC()

	props := actor.
		PropsFromProducer(func() actor.Actor { return &echoActor{} },
			actor.WithMailbox(actor.Bounded(1000000)))

	system := actor.NewActorSystem()
	r := remote.NewRemote(system, remote.Configure("127.0.0.1", 12000 /*, remote.WithCallOptions(grpc.UseCompressor(gzip.Name))*/))
	r.Register("echo", props)
	r.Start()

	rootContext := system.Root

	rootContext.SpawnNamed(props, "remote")

	console.ReadLine()
	r.Shutdown(true)
}
