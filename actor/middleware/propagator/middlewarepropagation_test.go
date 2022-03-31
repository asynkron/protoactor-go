package propagator

import (
	"sync"
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestPropagator(t *testing.T) {
	mutex := &sync.Mutex{}
	spawningCounter := 0
	system := actor.NewActorSystem()

	propagator := New().
		WithItselfForwarded().
		WithSpawnMiddleware(func(next actor.SpawnFunc) actor.SpawnFunc {
			return func(actorSystem *actor.ActorSystem, id string, props *actor.Props, parentContext actor.SpawnerContext) (pid *actor.PID, e error) {
				mutex.Lock()
				spawningCounter++
				mutex.Unlock()
				return next(actorSystem, id, props, parentContext)
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

	rootContext := actor.NewRootContext(system, nil).WithSpawnMiddleware(propagator.SpawnMiddleware)
	root := rootContext.Spawn(start(5))

	_ = rootContext.StopFuture(root).Wait()

	assert.Equal(t, spawningCounter, 5)
}
