package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestMemberStats_ParsesHeartbeatData(t *testing.T) {
	hb := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{
			ActorCount: map[string]int64{
				"kind-a": 5,
				"kind-b": 3,
			},
		},
	}

	anyVal, err := anypb.New(hb)
	require.NoError(t, err)

	gkv := &GossipKeyValue{
		Value: anyVal,
	}

	var parsed MemberHeartbeat
	err = gkv.Value.UnmarshalTo(&parsed)
	require.NoError(t, err)

	assert.Equal(t, int64(5), parsed.ActorStatistics.ActorCount["kind-a"])
	assert.Equal(t, int64(3), parsed.ActorStatistics.ActorCount["kind-b"])
}

func TestMemberStats_NilActorStatistics(t *testing.T) {
	hb := &MemberHeartbeat{}

	anyVal, err := anypb.New(hb)
	require.NoError(t, err)

	gkv := &GossipKeyValue{
		Value: anyVal,
	}

	var parsed MemberHeartbeat
	err = gkv.Value.UnmarshalTo(&parsed)
	require.NoError(t, err)

	assert.Nil(t, parsed.ActorStatistics)
}
