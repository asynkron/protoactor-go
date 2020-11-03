package router

import (
	"testing"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestSpawn(t *testing.T) {
	pr := &broadcastPoolRouter{PoolRouter{PoolSize: 1}}
	pid, err := spawn("foo", pr, actor.PropsFromFunc(func(context actor.Context) {}), system.Root)
	assert.NoError(t, err)

	_, exists := system.ProcessRegistry.Get(actor.NewPID(system.ProcessRegistry.Address, "foo/router"))
	assert.True(t, exists)

	err = system.Root.StopFuture(pid).Wait()
	assert.NoError(t, err)

	_, exists = system.ProcessRegistry.Get(actor.NewPID(system.ProcessRegistry.Address, "foo/router"))
	assert.False(t, exists)
}
