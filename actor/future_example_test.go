package actor_test

import (
	"fmt"
	"sync"
	"time"

	"github.com/otherview/protoactor-go/actor"
)

func ExampleFuture_PipeTo() {
	var wg sync.WaitGroup
	wg.Add(1)

	// test actor that will be the target of the future PipeTo
	pid := actor.EmptyRootContext.Spawn(actor.PropsFromFunc(func(ctx actor.Context) {
		// check if the message is a string and therefore
		// the "hello world" message piped from the future
		if m, ok := ctx.Message().(string); ok {
			fmt.Println(m)
			wg.Done()
		}
	}))

	f := actor.NewFuture(50 * time.Millisecond)
	f.PipeTo(pid)
	// resolve the future and pipe to waiting actor
	actor.EmptyRootContext.Send(f.PID(), "hello world")
	wg.Wait()

	// Output: hello world
}
