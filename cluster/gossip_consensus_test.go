package cluster_test

import (
	"testing"
	"time"

	cluster "github.com/asynkron/protoactor-go/cluster"
	cluster_test_tool "github.com/asynkron/protoactor-go/cluster/cluster_test_tool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Test that multiple in-memory cluster members propagate gossip state
// and agree on the same topology hash value.
func TestGossipConsensusOnTopologyHash(t *testing.T) {
	fixture := cluster_test_tool.NewBaseInMemoryClusterFixture(3)
	fixture.Initialize()
	defer fixture.ShutDown()

	members := fixture.GetMembers()
	expectedHash := members[0].MemberList.Members().TopologyHash()

	// Every member sets the same state and sends it to others.
	for _, m := range members {
		m.Gossip.SetState("hash", wrapperspb.UInt64(expectedHash))
		m.Gossip.SendState()
	}

	// Wait until all members have the state from every other member.
	for _, m := range members {
		cluster_test_tool.WaitUntil(t, func() bool {
			state, err := m.Gossip.GetState("hash")
			if err != nil || len(state) != len(members) {
				return false
			}
			for _, kv := range state {
				var v wrapperspb.UInt64Value
				if err := kv.Value.UnmarshalTo(&v); err != nil || v.Value != expectedHash {
					return false
				}
			}
			return true
		}, "members did not reach consensus", 5*time.Second)
	}

	// Verify topology hash helper produces the same value.
	actualHash := cluster.TopologyHash(members[0].MemberList.Members().Members())
	require.Equal(t, expectedHash, actualHash)
}
