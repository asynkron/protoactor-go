package main

import (
	"log"
	"runtime"

	"remotebenchmark/messages"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/mailbox"
	"github.com/AsynkronIT/protoactor-go/remote"
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
	runtime.GOMAXPROCS(runtime.NumCPU() * 1)
	runtime.GC()

	props := actor.
		PropsFromProducer(func() actor.Actor { return &echoActor{} }).
		WithMailbox(mailbox.Bounded(1000000))

	system := actor.NewActorSystem()
	r := remote.NewRemote(system, remote.Configure("127.0.0.1", 12000))
	r.Register("echo", props)
	r.Start()

	rootContext := system.Root

	rootContext.SpawnNamed(props, "remote")

	console.ReadLine()
	r.Shutdown(true)
}
