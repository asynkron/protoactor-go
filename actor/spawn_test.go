package actor

import (
	"log"
	"testing"
	"time"

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
		log.Printf("Started %s", a)
	case Increment:
		log.Printf("Incrementing %v", a)
		a.Increment()
		context.Sender().Tell(a.value)
	}
}

func TestLookupById(t *testing.T) {
	ID := "UniqueID"
	responsePID, result := RequestResponsePID()
	defer result.Stop()
	{
		props := FromInstance(&GorgeousActor{Counter: Counter{value: 0}})
		actor := SpawnNamed(props, ID)
		defer actor.Stop()
		actor.Ask(Increment{}, responsePID)
		value, err := result.ResultOrTimeout(testTimeout)
		if err != nil {
			assert.Fail(t, "timed out")
			return
		}
		assert.IsType(t, int(0), value)
		assert.Equal(t, 1, value.(int))
	}
	{
		props := FromInstance(&GorgeousActor{Counter: Counter{value: 0}})
		actor := SpawnNamed(props, ID)
		actor.Ask(Increment{}, responsePID)
		value, err := result.ResultOrTimeout(10 * time.Second)
		if err != nil {
			assert.Fail(t, "timed out")
			return
		}
		assert.Equal(t, 2, value.(int))
	}
}
