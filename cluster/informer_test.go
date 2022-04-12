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

	s := &MemberHeartbeat{}

	i := newInformer("123", a, 3, 3)
	i.SetState("heartbeat", s)
}
