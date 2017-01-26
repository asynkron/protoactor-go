package actor

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type ShortLivingActor struct {
}

func (self *ShortLivingActor) Receive(ctx Context) {

}

func TestStopFuture(t *testing.T) {
	plog.Debug("hello world")

	ID := "UniqueID"
	{
		props := FromInstance(&ShortLivingActor{})
		actor, _ := SpawnNamed(props, ID)

		fut := actor.StopFuture()

		res, errR := fut.Result()
		if errR != nil {
			assert.Fail(t, "Failed to wait stop actor %s", errR)
			return
		}

		_, ok := res.(*Terminated)
		if !ok {
			assert.Fail(t, "Cannot cast %s", reflect.TypeOf(res))
			return
		}

		_, found := ProcessRegistry.Get(actor)
		assert.False(t, found)
	}
}
