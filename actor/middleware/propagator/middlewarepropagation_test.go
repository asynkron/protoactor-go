package propagator

import (
	"sync"
	"testing"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestPropagator(t *testing.T) {
	mutex := &sync.Mutex{}
	spawningCounter := 0

	propagator := New().
		WithItselfForwarded().
		WithSpawnMiddleware(func(next actor.SpawnFunc) actor.SpawnFunc {
			return func(id string, props *actor.Props, parentContext actor.SpawnerContext) (pid *actor.PID, e error) {
				mutex.Lock()
				spawningCounter++
				mutex.Unlock()
				return next(id, props, parentContext)
			}
		})

	var start func(input int) *actor.Props
	start = func(input int) *actor.Props {
		return actor.PropsFromFunc(func(c actor.Context) {
			switch c.Message().(type) {
			case *actor.Started:
				if input > 0 {
					c.Spawn(start(input - 1))
				}
			}
		})
	}

	root := actor.NewRootContext(nil).WithSpawnMiddleware(propagator.SpawnMiddleware).Spawn(start(5))

	root.StopFuture().Wait()

	assert.Equal(t, spawningCounter, 5)
}
