package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrategyManager_UsesDefaultWhenNoKindStrategy(t *testing.T) {
	c := newClusterForTest("test-mgr-default", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	sm := NewStrategyManager(c)
	m := newTestMember("m1", "host1", 1000, "myKind")
	sm.AddMember(m)

	result := sm.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Equal(t, m, result, "should use default RoundRobin strategy")
}

func TestStrategyManager_UsesKindSpecificStrategy(t *testing.T) {
	c := newClusterForTest("test-mgr-kind", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	sm := NewStrategyManager(c)

	// Register a custom strategy for "myKind" that always returns a specific member.
	custom := NewRoundRobinStrategy()
	specific := newTestMember("specific", "specific-host", 9999, "myKind")
	custom.AddMember(specific)
	sm.RegisterKindStrategy("myKind", custom)

	// Also add a different member to the default strategy (with "otherKind" so
	// that the kind filter in GetActivator can match it).
	defaultMember := newTestMember("default", "default-host", 8888, "otherKind")
	sm.defaultStrategy.AddMember(defaultMember)

	// "myKind" should use the custom strategy.
	result := sm.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Equal(t, specific, result, "should use kind-specific strategy")

	// "otherKind" should fall back to default.
	result2 := sm.GetActivator(newTestCI("otherKind", "id1"), "127.0.0.1:0")
	assert.Equal(t, defaultMember, result2, "should fall back to default for unknown kind")
}

func TestStrategyManager_AddMemberPropagates(t *testing.T) {
	c := newClusterForTest("test-mgr-propagate", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	sm := NewStrategyManager(c)
	custom := NewRoundRobinStrategy()
	sm.RegisterKindStrategy("myKind", custom)

	m := newTestMember("m1", "host1", 1000, "myKind")
	sm.AddMember(m)

	// Both default and kind strategy should see the member.
	r1 := sm.defaultStrategy.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	assert.Equal(t, m, r1, "default strategy should have the member")

	r2 := custom.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	assert.Equal(t, m, r2, "kind strategy should have the member")
}

func TestStrategyManager_RemoveMemberPropagates(t *testing.T) {
	c := newClusterForTest("test-mgr-remove", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	sm := NewStrategyManager(c)
	custom := NewRoundRobinStrategy()
	sm.RegisterKindStrategy("myKind", custom)

	m := newTestMember("m1", "host1", 1000, "myKind")
	sm.AddMember(m)
	sm.RemoveMember(m)

	r1 := sm.defaultStrategy.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	assert.Nil(t, r1, "default strategy should have no members after remove")

	r2 := custom.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	assert.Nil(t, r2, "kind strategy should have no members after remove")
}

func TestStrategyManager_DefaultIsRoundRobinWhenNotConfigured(t *testing.T) {
	c := newClusterForTest("test-mgr-rr-default", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	sm := NewStrategyManager(c)

	// Verify the default is a RoundRobinStrategy by type assertion.
	_, ok := sm.defaultStrategy.(*RoundRobinStrategy)
	assert.True(t, ok, "default strategy should be RoundRobinStrategy")
}

func TestStrategyManager_Close(t *testing.T) {
	c := newClusterForTest("test-mgr-close", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	sm := NewStrategyManager(c)
	sm.RegisterKindStrategy("kindA", NewRoundRobinStrategy())
	sm.RegisterKindStrategy("kindB", NewLocalAffinityStrategy())

	assert.NotPanics(t, func() { sm.Close() })
}
