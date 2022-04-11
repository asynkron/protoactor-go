package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Increment struct{}

type GorgeousActor struct {
	Counter
}

type Counter struct {
	value int
}

func (counter *Counter) Increment() {
	counter.value = counter.value + 1
}

func (a *GorgeousActor) Receive(context Context) {
	switch context.Message().(type) {
	case *Started:
	case Increment:
		a.Increment()
		context.Respond(a.value)
	}
}

func TestLookupById(t *testing.T) {
	ID := "UniqueID"
	{
		props := PropsFromProducer(func() Actor { return &GorgeousActor{Counter: Counter{value: 0}} })
		pid, _ := rootContext.SpawnNamed(props, ID)
		defer rootContext.Stop(pid)

		result := rootContext.RequestFuture(pid, Increment{}, testTimeout)
		value, err := result.Result()
		if err != nil {
			assert.Fail(t, "timed out")
			return
		}
		assert.IsType(t, 0, value)
		assert.Equal(t, 1, value.(int))
	}
	{
		props := PropsFromProducer(func() Actor { return &GorgeousActor{Counter: Counter{value: 0}} })
		pid, _ := rootContext.SpawnNamed(props, ID)
		result := rootContext.RequestFuture(pid, Increment{}, testTimeout)
		value, err := result.Result()
		if err != nil {
			assert.Fail(t, "timed out")
			return
		}
		assert.Equal(t, 2, value.(int))
	}
}
