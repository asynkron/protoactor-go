package actor_test

import (
	"fmt"
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type setBehaviorActor struct {
	*sync.WaitGroup
}

// Receive is the default message handler when an actor is started
func (f *setBehaviorActor) Receive(context actor.Context) {
	if msg, ok := context.Message().(string); ok && msg == "other" {
		// Change actor's receive message handler to Other
		context.SetBehavior(f.Other)
	}
}

func (f *setBehaviorActor) Other(context actor.Context) {
	if msg, ok := context.Message().(string); ok && msg == "foo" {
		fmt.Println(msg)
		f.Done()
	}
}

// SetBehavior allows an actor to change its Receive handler, providing basic support for state machines
func ExampleContext_setBehavior() {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	pid := actor.Spawn(actor.FromProducer(func() actor.Actor { return &setBehaviorActor{wg} }))
	defer pid.GracefulStop()

	pid.Tell("other")
	pid.Tell("foo")
	wg.Wait()

	// Output: foo
}
