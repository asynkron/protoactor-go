package main

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/persistence"
	"github.com/AsynkronIT/goconsole"
)

type persistentActor struct {
	name  string
	items []string
}

//CQRS style messages
type RenameCommand struct {
	Name string
}

type RenamedEvent struct {
	Name string
}

func (RenamedEvent) PersistentMessage() {} //mark event as persistent

type AddItemCommand struct {
	Item string
}

type AddedItemEvent struct {
	Item string
}

func (AddedItemEvent) PersistentMessage() {} //mark event as persistent

type DumpCommand struct{}

func (self *persistentActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case RenameCommand: //command handler, you can have side effects here
		event := RenamedEvent{Name: msg.Name}
		log.Printf("Rename %v\n", msg.Name)
		context.Handle(event)
	case RenamedEvent: //event handler, only mutate state here
		self.name = msg.Name
	case AddItemCommand:
		event := AddedItemEvent{Item: msg.Item}
		log.Printf("Add item %v", msg.Item)
		context.Handle(event)
	case AddedItemEvent:
		self.items = append(self.items, msg.Item)
	case DumpCommand: //just so we can manually trigger a console dump of state
		log.Printf("%+v", self)
	case persistence.ReplayComplete: //will be triggered once the persistence plugin have replayed all events
		log.Println("Replay Complete")
		log.Printf("%+v", self)
	}
}

func newPersistentActor() actor.Actor {
	return &persistentActor{
		name: "Initial Name",
	}
}

func main() {
	props := actor.
		FromProducer(newPersistentActor).
		WithReceivePlugin(actor.AutoReceive, persistence.PersistenceReceive(&persistence.InMemoryProvider{}))

	pid := actor.Spawn(props)
	pid.Tell(AddItemCommand{Item: "Banana"})
	pid.Tell(AddItemCommand{Item: "Apple"})
	pid.Tell(AddItemCommand{Item: "Orange"})
	pid.Tell(RenameCommand{Name: "Acme Inc"})
	pid.Tell(DumpCommand{})
	pid.Tell(actor.PoisonPill{})
	pid.Tell(DumpCommand{})
	console.ReadLine()
}
