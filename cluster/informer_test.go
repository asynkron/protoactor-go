package cluster

import (
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/asynkron/gofun/set"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestInformer_SetState(t *testing.T) {
	t.Parallel()

	a := func() set.Set[string] {
		return set.New[string]()
	}

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}

	i := newInformer("member1", a, 3, 3, slog.Default())
	i.SetState("heartbeat", s)
}

func TestInformer_GetState(t *testing.T) {
	t.Parallel()

	a := func() set.Set[string] {
		return set.New[string]()
	}

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}

	i := newInformer("member1", a, 3, 3, slog.Default())
	i.SetState("heartbeat", s)

	m := i.GetState("heartbeat")

	x, ok := m["member1"]

	if !ok {
		t.Error("not ok")
	}

	var s2 MemberHeartbeat
	err := x.Value.UnmarshalTo(&s2)
	if err != nil {
		t.Error("unmarshal state error")
	}
}

func TestInformer_ReceiveState(t *testing.T) {
	t.Parallel()

	a := func() set.Set[string] {
		return set.New[string]()
	}

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}
	dummyValue, _ := anypb.New(s)

	i := newInformer("member1", a, 3, 3, slog.Default())
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

	m1, ok := m["member1"]

	if !ok {
		t.Error("member1 is missing")
	}

	var s1 MemberHeartbeat

	err := m1.Value.UnmarshalTo(&s1)
	if err != nil {
		t.Error("unmarshal member1 state error")
	}

	// ensure we see member2 after receiving state
	m2, ok := m["member2"]

	if !ok {
		t.Error("member2 is missing")
	}

	var s2 MemberHeartbeat

	err = m2.Value.UnmarshalTo(&s2)

	if err != nil {
		t.Error("unmarshal member2 state error")
	}
}

func TestInformer_SendState(t *testing.T) {
	t.Parallel()

	a := func() set.Set[string] {
		return set.New[string]()
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)

	sendState := func(memberStateDelta *MemberStateDelta, member *Member) {
		fmt.Printf("%+v\n", memberStateDelta) //nolint:forbidigo
		wg.Done()
	}

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}

	i := newInformer("member1", a, 3, 3, slog.Default())
	i.SetState("heartbeat", s)
	// the cluster sees two nodes. itself and member2
	i.UpdateClusterTopology(&ClusterTopology{
		Members: []*Member{
			{
				Id:   "member2",
				Host: "member2",
				Port: 123,
			},
			{
				Id:   "member1",
				Host: "member1",
				Port: 333,
			},
		},
	})

	// gossip never sends to self, so the only member we can send to is member2
	i.SendState(sendState)
	wg.Wait()
}

func TestInformer_UpdateClusterTopology(t *testing.T) {
	t.Parallel()

	a := func() set.Set[string] {
		return set.New[string]()
	}

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}
	i := newInformer("member1", a, 3, 3, slog.Default())
	i.SetState("heartbeat", s)
	// the cluster sees two nodes. itself and member2
	i.UpdateClusterTopology(&ClusterTopology{
		Members: []*Member{
			{
				Id:   "member2",
				Host: "member2",
				Port: 123,
			},
			{
				Id:   "member1",
				Host: "member1",
				Port: 333,
			},
		},
	})

	// TODO: how do we check that the cluster topology was updated?
}

func TestInformer_GetMemberStateDelta(t *testing.T) {
	t.Parallel()

	a := func() set.Set[string] {
		return set.New[string]()
	}

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}

	i := newInformer("member1", a, 3, 3, slog.Default())
	i.SetState("heartbeat", s)

	m := i.GetMemberStateDelta("member1")

	if m == nil {
		t.Error("member state delta is nil")
	}
}
