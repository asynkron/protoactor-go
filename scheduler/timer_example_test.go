package scheduler_test

import (
	"fmt"
	"sync"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/scheduler"
)

var system = actor.NewActorSystem()

// Use the timer scheduler to repeatedly send messages to an actor.
func ExampleTimerScheduler_sendRepeatedly() {
	var wg sync.WaitGroup
	wg.Add(2)

	count := 0
	props := actor.PropsFromFunc(func(c actor.Context) {
		switch v := c.Message().(type) {
		case string:
			count++
			fmt.Println(count, v)
			wg.Done()
		}
	})

	pid := system.Root.Spawn(props)

	s := scheduler.NewTimerScheduler(system.Root)
	cancel := s.SendRepeatedly(1*time.Millisecond, 1*time.Millisecond, pid, "Hello")
	wg.Wait()
	cancel()

	// Output:
	// 1 Hello
	// 2 Hello
}
