package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGossipConsensusHandler_SetAndGet(t *testing.T) {
	h := NewGossipConsensusHandler()
	h.TrySetConsensus("value1")

	val, ok := h.TryGetConsensus(context.Background())
	assert.True(t, ok)
	assert.Equal(t, "value1", val)
}

func TestGossipConsensusHandler_Reset(t *testing.T) {
	h := NewGossipConsensusHandler()
	h.TrySetConsensus("value1")

	h.TryResetConsensus()

	_, ok := h.TryGetConsensus(context.Background())
	assert.False(t, ok, "consensus should be reset")
}

func TestGossipConsensusHandler_SetAfterReset(t *testing.T) {
	h := NewGossipConsensusHandler()
	h.TrySetConsensus("value1")
	h.TryResetConsensus()
	h.TrySetConsensus("value2")

	val, ok := h.TryGetConsensus(context.Background())
	assert.True(t, ok)
	assert.Equal(t, "value2", val)
}
