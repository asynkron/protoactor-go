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

	case *persistence.ReplayComplete: //will be triggered once the persistence plugin have replayed all events
		log.Println("Replay Complete")
		context.Receive(&messages.DumpCommand{})
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
			actor.MessageLogging,  //<- logging receive pipeline
			persistence.Using(cb)) //<- persistence receive pipeline

	pid := actor.Spawn(props)
	pid.Tell(&messages.AddItemCommand{Item: "Banana"})
	pid.Tell(&messages.AddItemCommand{Item: "Apple"})
	pid.Tell(&messages.AddItemCommand{Item: "Orange"})
	pid.Tell(&messages.RenameCommand{Name: "Acme Inc"})
	pid.Tell(&messages.DumpCommand{}) //dump current state to console
	pid.Tell(&actor.PoisonPill{})     //force restart of actor to show that we can handle failure
	pid.Tell(&messages.DumpCommand{})
	console.ReadLine()
}
