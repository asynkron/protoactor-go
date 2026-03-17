package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Shared test helpers used by all strategy test files.

func newTestMember(id, host string, port int32, kinds ...string) *Member {
	return &Member{Id: id, Host: host, Port: port, Kinds: kinds}
}

func newTestCI(kind, identity string) *ClusterIdentity {
	return &ClusterIdentity{Kind: kind, Identity: identity}
}

func TestRoundRobinStrategy_ReturnsNilWhenNoMembers(t *testing.T) {
	s := NewRoundRobinStrategy()
	result := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Nil(t, result)
}

func TestRoundRobinStrategy_SingleMember(t *testing.T) {
	s := NewRoundRobinStrategy()
	m := newTestMember("m1", "host1", 1000, "myKind")
	s.AddMember(m)

	result := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Equal(t, m, result)

	// Single member should always return the same member.
	result2 := s.GetActivator(newTestCI("myKind", "id2"), "127.0.0.1:0")
	assert.Equal(t, m, result2)
}

func TestRoundRobinStrategy_CyclesThroughMembers(t *testing.T) {
	s := NewRoundRobinStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	m3 := newTestMember("m3", "host3", 1002, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.AddMember(m3)

	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
		if r != nil {
			seen[r.Id]++
		}
	}

	// Each member selected exactly 3 times out of 9 requests.
	assert.Equal(t, 3, seen["m1"])
	assert.Equal(t, 3, seen["m2"])
	assert.Equal(t, 3, seen["m3"])
}

func TestRoundRobinStrategy_FiltersbyKind(t *testing.T) {
	s := NewRoundRobinStrategy()
	s.AddMember(newTestMember("m1", "host1", 1000, "kindA"))
	s.AddMember(newTestMember("m2", "host2", 1001, "kindB"))
	s.AddMember(newTestMember("m3", "host3", 1002, "kindA", "kindB"))

	// Only m1 and m3 support kindA.
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("kindA", "id"), "127.0.0.1:0")
		assert.NotNil(t, r)
		assert.Contains(t, []string{"m1", "m3"}, r.Id, "should only select members with kindA")
	}
}

func TestRoundRobinStrategy_RemoveMember(t *testing.T) {
	s := NewRoundRobinStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	s.RemoveMember(m1)

	for i := 0; i < 5; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
		assert.Equal(t, m2, r, "after removing m1, only m2 should be returned")
	}
}

func TestRoundRobinStrategy_AddDuplicate(t *testing.T) {
	s := NewRoundRobinStrategy()
	m := newTestMember("m1", "host1", 1000, "myKind")
	s.AddMember(m)
	s.AddMember(m) // duplicate

	// Should still only have one member.
	count := 0
	for i := 0; i < 5; i++ {
		if s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0") != nil {
			count++
		}
	}
	assert.Equal(t, 5, count)
}

func TestRoundRobinStrategy_NoMatchingKind(t *testing.T) {
	s := NewRoundRobinStrategy()
	s.AddMember(newTestMember("m1", "host1", 1000, "kindA"))

	result := s.GetActivator(newTestCI("kindB", "id1"), "127.0.0.1:0")
	assert.Nil(t, result, "should return nil when no member supports the kind")
}

func TestRoundRobinStrategy_Close(t *testing.T) {
	s := NewRoundRobinStrategy()
	// Close is a no-op, should not panic.
	assert.NotPanics(t, func() { s.Close() })
}
