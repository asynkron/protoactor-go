package main

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/couchbase_persistence"
	"github.com/AsynkronIT/gam/examples/persistence/messages"
	"github.com/AsynkronIT/gam/persistence"
	"github.com/AsynkronIT/goconsole"
)

type persistentActor struct {
	persistence.Mixin
	state messages.State
}

//CQRS style messages
func (self *persistentActor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {

	case *messages.RenameCommand: //command handler, you can have side effects here
		event := &messages.RenamedEvent{Name: msg.Name}
		log.Printf("Rename %v\n", msg.Name)
		self.PersistReceive(event)

	case *messages.RenamedEvent: //event handler, only mutate state here
		self.state.Name = msg.Name

	case *messages.AddItemCommand:
		event := &messages.AddedItemEvent{Item: msg.Item}
		log.Printf("Add item %v", msg.Item)
		self.PersistReceive(event)

	case *messages.AddedItemEvent:
		self.state.Items = append(self.state.Items, msg.Item)

	case *messages.DumpCommand: //just so we can manually trigger a console dump of state
		log.Printf("%+v", self)

	case *persistence.RequestSnapshot:
		self.PersistSnapshot(&self.state)

	case *messages.State:
		self.state = *msg
	}
}

func newPersistentActor() actor.Actor {
	return &persistentActor{}
}

func main() {

	cb := couchbase_persistence.New("labb", "couchbase://localhost")
	props := actor.
		FromProducer(newPersistentActor).
		WithReceivers(
			//	actor.MessageLogging,  //<- logging receive pipeline
			persistence.Using(cb)) //<- persistence receive pipeline

	pid := actor.Spawn(props)
	pid.Tell(&messages.DumpCommand{})
	// pid.Tell(&messages.RenameCommand{Name: "Acme Inc"})
	// pid.Tell(&messages.AddItemCommand{Item: "Banana"})
	// pid.Tell(&messages.AddItemCommand{Item: "Apple"})
	// pid.Tell(&messages.AddItemCommand{Item: "Orange"})

	console.ReadLine()
}
