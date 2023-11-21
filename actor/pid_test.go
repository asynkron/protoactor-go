package actor

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type ShortLivingActor struct{}

func (sl *ShortLivingActor) Receive(Context) {
}

func TestStopFuture(t *testing.T) {

	ID := "UniqueID"
	{
		props := PropsFromProducer(func() Actor { return &ShortLivingActor{} })
		a, _ := rootContext.SpawnNamed(props, ID)

		fut := rootContext.StopFuture(a)

		res, errR := fut.Result()
		if errR != nil {
			assert.Fail(t, "Failed to wait stop actor %pids", errR)
			return
		}

		_, ok := res.(*Terminated)
		if !ok {
			assert.Fail(t, "Cannot cast %pids", reflect.TypeOf(res))
			return
		}

		_, found := system.ProcessRegistry.Get(a)
		assert.False(t, found)
	}
}
