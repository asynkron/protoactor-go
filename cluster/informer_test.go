package cluster

import (
	"testing"

	"github.com/asynkron/gofun/set"
	"google.golang.org/protobuf/types/known/anypb"
)

func a() set.Set[string] {
	return set.New[string]()
}

func TestInformer_SetState(t *testing.T) {
	t.Parallel()

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}

	i := newInformer("member1", a, 3, 3)
	i.SetState("heartbeat", s)
}

func TestInformer_GetState(t *testing.T) {
	t.Parallel()

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}

	i := newInformer("member1", a, 3, 3)
	i.SetState("heartbeat", s)

	m := i.GetState("heartbeat")

	x, ok := m["member1"]

	if !ok {
		t.Error("not ok")
	}

	var s2 *MemberHeartbeat
	_ = x.UnmarshalTo(s2)
}

func TestInformer_ReceiveState(t *testing.T) {
	t.Parallel()

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}
	dummyValue, _ := anypb.New(s)

	i := newInformer("member1", a, 3, 3)
	i.SetState("heartbeat", s)

	remoteState := &GossipState{
		Members: GossipMemberStates{
			"member2": {
				Values: GossipKeyValues{
					"heartbeat": {
						Value:          dummyValue,
						SequenceNumber: 1,
					},
				},
			},
		},
	}

	i.ReceiveState(remoteState)

	m := i.GetState("heartbeat")

	var ok bool

	_, ok = m["member1"]

	if !ok {
		t.Error("member1 is missing")
	}

	// ensure we see member2 after receiving state
	_, ok = m["member2"]

	if !ok {
		t.Error("member2 is missing")
	}
}
