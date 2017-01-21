package actor_test

import (
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type emptyActor struct{}

func (*emptyActor) Receive(actor.Context) {}

// Subscribe subscribes to events on the EventStream, the given predicate can be used to filter out events
func ExampleEventStream_Subscribe() {
	var wg sync.WaitGroup
	wg.Add(1)

	//create an actor
	a := actor.Spawn(actor.FromInstance(&emptyActor{}))

	//subscribe to the DeadLetterEvent
	sub := actor.EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*actor.DeadLetterEvent); ok {
			if deadLetter.PID == a {
				wg.Done()
			}
		}
	})
	defer actor.EventStream.Unsubscribe(sub)

	//stop the actor
	a.
		StopFuture().
		Wait()

	//send a message to the now stopped actor
	a.Tell("hello")

	//we should now get a DeadLetterEvent
	wg.Wait()
}
