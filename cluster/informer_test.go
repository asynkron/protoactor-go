package cluster

import (
	"google.golang.org/protobuf/types/known/anypb"
	"testing"

	"github.com/asynkron/gofun/set"
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
		Members: map[string]*GossipMemberState{
			"member2": {
				Values: map[string]*GossipKeyValue{
					"heartbeat": {
						Value: dummyValue,
					},
				},
			},
		},
	}

	i.ReceiveState(remoteState)

	m := i.GetState("heartbeat")

	_, ok := m["member1"]

	if !ok {
		t.Error("member1 is missing")
	}

	_, ok = m["member2"]

	if !ok {
		t.Error("member2 is missing")
	}
}
