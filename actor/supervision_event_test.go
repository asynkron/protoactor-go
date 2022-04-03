package actor

import (
	"sync"
	"testing"
	"time"
)

type panicActor struct{}

func (a *panicActor) Receive(ctx Context) {
	switch ctx.Message().(type) {
	case string:
		panic("Boom!")
	}
}

func TestSupervisorEventHandleFromEventstream(t *testing.T) {
	supervisors := []struct {
		name     string
		strategy SupervisorStrategy
	}{
		{
			name:     "all_for_one",
			strategy: NewAllForOneStrategy(10, 10*time.Second, DefaultDecider),
		},
		{
			name:     "exponential_backoff",
			strategy: NewExponentialBackoffStrategy(10*time.Millisecond, 10*time.Millisecond),
		},
		{
			name:     "one_for_one",
			strategy: NewOneForOneStrategy(10, 10*time.Second, DefaultDecider),
		},
		{
			name:     "restarting",
			strategy: NewRestartingStrategy(),
		},
	}

	for _, v := range supervisors {
		t.Run(v.name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			sid := system.EventStream.Subscribe(func(evt interface{}) {
				if _, ok := evt.(*SupervisorEvent); ok {
					wg.Done()
				}
			})
			defer system.EventStream.Unsubscribe(sid)

			props := PropsFromProducer(func() Actor { return &panicActor{} }, WithSupervisor(v.strategy))
			pid := rootContext.Spawn(props)

			wg.Add(1)
			rootContext.Send(pid, "Fail!")

			wg.Wait()
		})
	}
}
