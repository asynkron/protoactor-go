package cluster

import (
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
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

func TestPidCache_EntryExpires(t *testing.T) {
	cache := NewPidCacheWithTTL(100 * time.Millisecond)
	pid := actor.NewPID("localhost", "test")

	cache.Set("id1", "kind1", pid)

	got, ok := cache.Get("id1", "kind1")
	assert.True(t, ok)
	assert.Equal(t, pid, got)

	time.Sleep(150 * time.Millisecond)

	_, ok = cache.Get("id1", "kind1")
	assert.False(t, ok, "entry should have expired")
}

func TestPidCache_ZeroTTLNeverExpires(t *testing.T) {
	cache := NewPidCache()
	pid := actor.NewPID("localhost", "test")

	cache.Set("id1", "kind1", pid)

	time.Sleep(50 * time.Millisecond)

	got, ok := cache.Get("id1", "kind1")
	assert.True(t, ok, "entry with zero TTL should not expire")
	assert.Equal(t, pid, got)
}

func TestPidCache_TTLRemoveByValue(t *testing.T) {
	cache := NewPidCacheWithTTL(5 * time.Second)
	pid := actor.NewPID("localhost", "test1")
	pid2 := actor.NewPID("localhost", "test2")

	cache.Set("id1", "kind1", pid)

	// Should not remove because pid2 != pid
	cache.RemoveByValue("id1", "kind1", pid2)
	got, ok := cache.Get("id1", "kind1")
	assert.True(t, ok)
	assert.Equal(t, pid, got)

	// Should remove because pid matches
	cache.RemoveByValue("id1", "kind1", pid)
	_, ok = cache.Get("id1", "kind1")
	assert.False(t, ok)
}

func TestPidCache_TTLRemoveByMember(t *testing.T) {
	cache := NewPidCacheWithTTL(5 * time.Second)

	member := &Member{
		Host: "abc",
		Port: 123,
	}

	pid := actor.NewPID("abc:123", "def")
	cache.Set("abc", "k", pid)

	cache.RemoveByMember(member)

	_, ok := cache.Get("abc", "k")
	assert.False(t, ok, "entry should have been removed by member")
}
