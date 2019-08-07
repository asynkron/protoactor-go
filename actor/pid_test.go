package actor

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type ShortLivingActor struct {
}

func (sl *ShortLivingActor) Receive(ctx Context) {

}

func TestStopFuture(t *testing.T) {
	plog.Debug("hello world")

	ID := "UniqueID"
	{
		props := PropsFromProducer(func() Actor { return &ShortLivingActor{} })
		a, _ := rootContext.SpawnNamed(props, ID)

		fut := rootContext.StopFuture(a)

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

		_, found := ProcessRegistry.Get(a)
		assert.False(t, found)
	}
}
