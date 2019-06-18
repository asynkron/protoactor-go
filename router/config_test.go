package router

import (
	"testing"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestSpawn(t *testing.T) {
	pr := &broadcastPoolRouter{PoolRouter{PoolSize: 1}}
	pid, err := spawn("foo", pr, actor.PropsFromFunc(func(context actor.Context) {}), actor.EmptyRootContext)
	assert.NoError(t, err)

	_, exists := actor.ProcessRegistry.Get(actor.NewLocalPID("foo/router"))
	assert.True(t, exists)

	err = rootContext.StopFuture(pid).Wait()
	assert.NoError(t, err)

	_, exists = actor.ProcessRegistry.Get(actor.NewLocalPID("foo/router"))
	assert.False(t, exists)
}
