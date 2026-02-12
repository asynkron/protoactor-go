package actor

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootContext_TrySpawn(t *testing.T) {
	props := PropsFromProducer(func() Actor { return nullReceive })
	pid, err := rootContext.TrySpawn(props)
	require.NoError(t, err)
	require.NotNil(t, pid)
	defer rootContext.Stop(pid)

	assert.NotEmpty(t, pid.Id)
}

func TestRootContext_TrySpawn_Error(t *testing.T) {
	spawnErr := errors.New("test spawn error")
	props := PropsFromProducer(func() Actor { return nullReceive },
		WithSpawnFunc(func(actorSystem *ActorSystem, id string, props *Props, parentContext SpawnerContext) (*PID, error) {
			return nil, spawnErr
		}),
	)

	pid, err := rootContext.TrySpawn(props)
	assert.Nil(t, pid)
	assert.ErrorIs(t, err, spawnErr)
}

func TestRootContext_TrySpawnPrefix(t *testing.T) {
	props := PropsFromProducer(func() Actor { return nullReceive })
	pid, err := rootContext.TrySpawnPrefix(props, "myprefix")
	require.NoError(t, err)
	require.NotNil(t, pid)
	defer rootContext.Stop(pid)

	assert.True(t, strings.HasPrefix(pid.Id, "myprefix"), "expected PID Id to start with 'myprefix', got: %s", pid.Id)
}
