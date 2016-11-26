package actor

import (
	"testing"

	"log"
	"reflect"

	"github.com/stretchr/testify/assert"
)

type ShortLivingActor struct {
}

func (self *ShortLivingActor) Receive(ctx Context) {

}

func TestStopFuture(t *testing.T) {
	ID := "UniqueID"
	{
		props := FromInstance(&ShortLivingActor{})
		actor := SpawnNamed(props, ID)

		fut := actor.StopFuture()

		res, errR := fut.Result()
		if errR != nil {
			assert.Fail(t, "Failed to wait stop actor %s", errR)
			return
		}
		log.Printf("Res = %s", res)

		stopped, ok := res.(*Terminated)
		if !ok {
			assert.Fail(t, "Cannot cast %s", reflect.TypeOf(res))
			return
		}

		log.Printf("Received %s", stopped)

		_, found := ProcessRegistry.get(actor)
		assert.False(t, found)
	}
}
