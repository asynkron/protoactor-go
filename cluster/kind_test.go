package cluster

import (
	"context"
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKind_Defaults(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	k := NewKind("TestKind", props)

	assert.Equal(t, "TestKind", k.Kind)
	assert.NotNil(t, k.Props)
	assert.Nil(t, k.StrategyBuilder)
	assert.Nil(t, k.CanSpawnIdentity)
	assert.Nil(t, k.ActivatorStrategyBuilder)
}

func TestKind_WithCanSpawnIdentity(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	predicate := func(ctx context.Context, identity string) (bool, error) {
		return identity != "blocked", nil
	}

	k := NewKind("TestKind", props).WithCanSpawnIdentity(predicate)

	require.NotNil(t, k.CanSpawnIdentity)
	ok, err := k.CanSpawnIdentity(context.Background(), "allowed")
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = k.CanSpawnIdentity(context.Background(), "blocked")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestKind_WithActivatorStrategy(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	builder := func(c *Cluster) ActivatorStrategy { return nil }

	k := NewKind("TestKind", props).WithActivatorStrategy(builder)

	require.NotNil(t, k.ActivatorStrategyBuilder)
}

func TestKind_BuildPreservesNewFields(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	predicate := func(ctx context.Context, identity string) (bool, error) {
		return true, nil
	}

	k := NewKind("TestKind", props).WithCanSpawnIdentity(predicate)

	// Build with nil cluster (no strategy builder set).
	ak := k.Build(nil)
	assert.Equal(t, "TestKind", ak.Kind)
	assert.NotNil(t, ak.CanSpawnIdentity)

	ok, err := ak.CanSpawnIdentity(context.Background(), "any")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestKind_Chaining(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})

	// All builder methods should return *Kind for chaining.
	k := NewKind("TestKind", props).
		WithCanSpawnIdentity(func(ctx context.Context, id string) (bool, error) { return true, nil }).
		WithActivatorStrategy(func(c *Cluster) ActivatorStrategy { return nil })

	assert.NotNil(t, k.CanSpawnIdentity)
	assert.NotNil(t, k.ActivatorStrategyBuilder)
}
