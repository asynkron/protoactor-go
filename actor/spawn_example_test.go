package actor_test

import (
	"fmt"
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
)

// Spawn creates instances of actors, similar to 'new' or 'make' but for actors.
func ExampleSpawn() {
	var wg sync.WaitGroup
	wg.Add(1)

	//define the actor props
	//props define the creation process of an actor
	props := actor.FromFunc(func(ctx actor.Context) {
		// check if the message is a *actor.Started message
		// this is the first message all actors get
		// actor.Started is received async and can be used
		// to initialize your actors initial state
		if _, ok := ctx.Message().(*actor.Started); ok {
			fmt.Println("hello world")
			wg.Done()
		}
	})

	// spawn the actor based on the props
	actor.Spawn(props)
	wg.Wait()
	// Output: hello world
}
