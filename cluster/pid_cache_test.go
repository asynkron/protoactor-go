package cluster

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
)

func PidCache_Set_Test(t *testing.T) {
	pidCache := NewPidCache()
	pid := actor.NewPID("abc", "def")
	pidCache.Set("abc", "k", pid)
	res, _ := pidCache.Get("abc", "k")

	if !res.Equal(pid) {
		t.Errorf("Expected %v, got %v", pid, res)
	}
}
