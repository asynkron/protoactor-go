package cluster

import (
	"testing"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestDeactivationReason_String(t *testing.T) {
	tests := []struct {
		reason DeactivationReason
		want   string
	}{
		{DeactivationReasonUnknown, "unknown"},
		{DeactivationReasonPassivation, "passivation"},
		{DeactivationReasonShutdown, "shutdown"},
		{DeactivationReasonTopologyChange, "topology-change"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.reason.String())
	}
}

func TestDeactivationReasons_SetAndPop(t *testing.T) {
	dr := newDeactivationReasons()
	pid := actor.NewPID("127.0.0.1:8080", "test-actor")

	assert.Equal(t, DeactivationReasonUnknown, dr.Pop(pid))

	dr.Set(pid, DeactivationReasonPassivation)
	assert.Equal(t, DeactivationReasonPassivation, dr.Pop(pid))

	assert.Equal(t, DeactivationReasonUnknown, dr.Pop(pid))
}

func TestDeactivationReasons_MultiplePIDs(t *testing.T) {
	dr := newDeactivationReasons()
	pid1 := actor.NewPID("127.0.0.1:8080", "actor-1")
	pid2 := actor.NewPID("127.0.0.1:8080", "actor-2")

	dr.Set(pid1, DeactivationReasonShutdown)
	dr.Set(pid2, DeactivationReasonTopologyChange)

	assert.Equal(t, DeactivationReasonShutdown, dr.Pop(pid1))
	assert.Equal(t, DeactivationReasonTopologyChange, dr.Pop(pid2))
}
