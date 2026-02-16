package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPubSub_Start_DoesNotPanic(t *testing.T) {
	// Start on a fresh PubSub should not panic even if actor system
	// has issues - it should return an error instead.
	cp := newInmemoryProvider()
	c := newClusterForTest("test-pubsub-start", cp)

	// Starting member initializes the PubSub, which calls Start().
	// This should succeed without panic.
	err := c.StartMember()
	assert.NoError(t, err)
}
