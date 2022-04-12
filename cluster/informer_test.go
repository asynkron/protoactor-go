package cluster

import (
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

	i := newInformer("123", a, 3, 3)
	i.SetState("heartbeat", s)
}

func TestInformer_GetState(t *testing.T) {
	t.Parallel()

	s := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{},
	}

	i := newInformer("123", a, 3, 3)
	i.SetState("heartbeat", s)

	m := i.GetState("heartbeat")

	x, ok := m["123"]

	if !ok {
		t.Error("not ok")
	}

	var s2 *MemberHeartbeat
	_ = x.UnmarshalTo(s2)

}
