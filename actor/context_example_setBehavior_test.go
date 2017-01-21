package actor_test

import (
	"fmt"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type setBehaviorActor struct{}

// Receive is the default message handler when an actor is started
func (f *setBehaviorActor) Receive(context actor.Context) {
	if msg, ok := context.Message().(string); ok && msg == "other" {
		// Change actor's receive message handler to Other
		context.SetBehavior(f.Other)
	}
}

func (f *setBehaviorActor) Other(context actor.Context) {
	fmt.Println(context.Message())
}

func ExampleContext_setBehavior() {
	pid := actor.Spawn(actor.FromInstance(&setBehaviorActor{}))
	defer pid.Stop()

	pid.Tell("other")
	pid.RequestFuture("hello from other", 10*time.Millisecond).Wait()

	// Output: hello from other
}
