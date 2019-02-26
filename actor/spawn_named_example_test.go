package actor_test

import (
	"fmt"
	"log"
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
)

// Spawn creates instances of actors, similar to 'new' or 'make' but for actors.
func ExampleSpawnNamed() {
	var wg sync.WaitGroup
	wg.Add(1)

	// create root context
	context := actor.EmptyRootContext

	// define the actor props
	// props define the creation process of an actor
	props := actor.PropsFromFunc(func(ctx actor.Context) {
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
	_, err := context.SpawnNamed(props, "my-actor")
	if err != nil {
		log.Fatal("The actor name is already in use")
	}
	wg.Wait()
	// Output: hello world
}
