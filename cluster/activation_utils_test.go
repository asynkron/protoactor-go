package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateActivationMember_MemberAlive(t *testing.T) {
	c := newClusterForTest("test-validate", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	// The cluster's own member should be alive.
	host, port, _ := c.ActorSystem.GetHostPort()
	selfID := fmt.Sprintf("%s@%s:%d", "test-validate", host, port)
	assert.True(t, ValidateActivationMember(c.MemberList, selfID),
		"should return true for alive member")
}

func TestValidateActivationMember_MemberDead(t *testing.T) {
	c := newClusterForTest("test-validate-dead", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	assert.False(t, ValidateActivationMember(c.MemberList, "nonexistent-member-id"),
		"should return false for unknown member")
}
