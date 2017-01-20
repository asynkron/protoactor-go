package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Increment struct {
}

type Incrementable interface {
	Increment()
}

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
		props := FromInstance(&GorgeousActor{Counter: Counter{value: 0}})
		actor, _ := SpawnNamed(props, ID)
		defer actor.Stop()

		result := actor.RequestFuture(Increment{}, testTimeout)
		value, err := result.Result()
		if err != nil {
			assert.Fail(t, "timed out")
			return
		}
		assert.IsType(t, int(0), value)
		assert.Equal(t, 1, value.(int))
	}
	{
		props := FromInstance(&GorgeousActor{Counter: Counter{value: 0}})
		actor, _ := SpawnNamed(props, ID)
		result := actor.RequestFuture(Increment{}, testTimeout)
		value, err := result.Result()
		if err != nil {
			assert.Fail(t, "timed out")
			return
		}
		assert.Equal(t, 2, value.(int))
	}
}
