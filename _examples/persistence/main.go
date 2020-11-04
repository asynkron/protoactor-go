package main

import (
	"fmt"
	"log"
	"strconv"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/persistence"
)

type Provider struct {
	providerState persistence.ProviderState
}

func NewProvider(snapshotInterval int) *Provider {
	return &Provider{
		providerState: persistence.NewInMemoryProvider(snapshotInterval),
	}
}

func (p *Provider) InitState(actorName string, eventNum, eventIndexAfterSnapshot int) {
	for i := 0; i < eventNum; i++ {
		p.providerState.PersistEvent(
			actorName,
			i,
			&Message{protoMsg: protoMsg{state: "state" + strconv.Itoa(i)}},
		)
	}
	p.providerState.PersistSnapshot(
		actorName,
		eventIndexAfterSnapshot,
		&Snapshot{protoMsg: protoMsg{state: "state" + strconv.Itoa(eventIndexAfterSnapshot-1)}},
	)
}

func (p *Provider) GetState() persistence.ProviderState {
	return p.providerState
}

type protoMsg struct{ state string }

func (p *protoMsg) Reset()         {}
func (p *protoMsg) String() string { return p.state }
func (p *protoMsg) ProtoMessage()  {}

type Message struct{ protoMsg }
type Snapshot struct{ protoMsg }

type Actor struct {
	persistence.Mixin
	state string
}

func (a *Actor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		log.Println("actor started")
	case *persistence.RequestSnapshot:
		log.Printf("snapshot internal state '%v'", a.state)
		a.PersistSnapshot(&Snapshot{protoMsg: protoMsg{state: a.state}})
	case *Snapshot:
		a.state = msg.state
		log.Printf("recovered from snapshot, internal state changed to '%v'", a.state)
	case *persistence.ReplayComplete:
		log.Printf("replay completed, internal state changed to '%v'", a.state)
	case *Message:
		scenario := "received replayed event"
		if !a.Recovering() {
			a.PersistReceive(msg)
			scenario = "received new message"
		}
		a.state = msg.state
		log.Printf("%s, internal state changed to '%v'\n", scenario, a.state)
	}
}

func main() {

	system := actor.NewActorSystem()
	provider := NewProvider(3)
	provider.InitState("persistent", 4, 3)

	rootContext := system.Root
	props := actor.PropsFromProducer(func() actor.Actor { return &Actor{} }).WithReceiverMiddleware(persistence.Using(provider))
	pid, _ := rootContext.SpawnNamed(props, "persistent")
	rootContext.Send(pid, &Message{protoMsg: protoMsg{state: "state4"}})
	rootContext.Send(pid, &Message{protoMsg: protoMsg{state: "state5"}})

	rootContext.PoisonFuture(pid).Wait()
	fmt.Printf("*** restart ***\n")
	pid, _ = rootContext.SpawnNamed(props, "persistent")

	_, _ = console.ReadLine()
}
