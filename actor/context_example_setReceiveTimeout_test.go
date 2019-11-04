package actor_test

import (
	"fmt"
	"sync"
	"time"

	"github.com/otherview/protoactor-go/actor"
)

type setReceiveTimeoutActor struct {
	*sync.WaitGroup
}

// Receive is the default message handler when an actor is started
func (f *setReceiveTimeoutActor) Receive(context actor.Context) {
	switch context.Message().(type) {
	case *actor.Started:
		// when the actor starts, set the receive timeout to 10 milliseconds.
		//
		// the system will send an *actor.ReceiveTimeout message every time the timeout
		// expires until SetReceiveTimeout is called again with a duration < 1 ms]
		context.SetReceiveTimeout(10 * time.Millisecond)
	case *actor.ReceiveTimeout:
		fmt.Println("timed out")
		f.Done()
	}
}

// SetReceiveTimeout allows an actor to be notified repeatedly if it does not receive any messages
// for a specified duration
func ExampleContext_setReceiveTimeout() {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	pid := actor.EmptyRootContext.Spawn(actor.PropsFromProducer(func() actor.Actor { return &setReceiveTimeoutActor{wg} }))
	defer func() {
		actor.EmptyRootContext.StopFuture(pid).Wait()
	}()

	wg.Wait() // wait for the ReceiveTimeout message

	// Output: timed out
}
