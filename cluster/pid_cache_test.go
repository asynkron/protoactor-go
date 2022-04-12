package cluster

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
)

func TestPidCacheValue_Set_Test(t *testing.T) {
	t.Parallel()

	pidCache := NewPidCache()
	pid := actor.NewPID("abc", "def")
	pidCache.Set("abc", "k", pid)
	res, _ := pidCache.Get("abc", "k")

	if !res.Equal(pid) {
		t.Errorf("Expected %v, got %v", pid, res)
	}
}

func TestPidCacheValue_Get(t *testing.T) {
	t.Parallel()

	pidCache := NewPidCache()
	pid := actor.NewPID("abc", "def")
	pidCache.Set("abc", "k", pid)
	res, _ := pidCache.Get("abc", "k")

	if !res.Equal(pid) {
		t.Errorf("Expected %v, got %v", pid, res)
	}
}

func TestPidCacheValue_Remove(t *testing.T) {
	t.Parallel()

	pidCache := NewPidCache()
	pid := actor.NewPID("abc", "def")
	pidCache.Set("abc", "k", pid)
	pidCache.Remove("abc", "k")
	res, _ := pidCache.Get("abc", "k")

	if res != nil {
		t.Errorf("Expected nil, got %v", res)
	}
}

func TestPidCacheValue_RemoveByMember(t *testing.T) {
	t.Parallel()

	member := &Member{
		Host: "abc",
		Port: 123,
	}

	pidCache := NewPidCache()
	pid := actor.NewPID("abc:123", "def")
	pidCache.Set("abc", "k", pid)
	pidCache.RemoveByMember(member)
	res, _ := pidCache.Get("abc", "k")

	if res != nil {
		t.Errorf("Expected nil, got %v", res)
	}
}

func TestPidCacheValue_RemoveByValue(t *testing.T) {
	t.Parallel()

	pidCache := NewPidCache()
	pid := actor.NewPID("abc", "def1234")
	pid2 := actor.NewPID("abc", "def3532534")

	pidCache.Set("abc", "k", pid)
	pidCache.RemoveByValue("abc", "k", pid2)
	res, _ := pidCache.Get("abc", "k")

	if res == nil {
		t.Errorf("Expected %v, got %v", pid, res)
	}

	pidCache.RemoveByValue("abc", "k", pid)
	res, _ = pidCache.Get("abc", "k")

	if res != nil {
		t.Errorf("Expected nil, got %v", res)
	}
}
