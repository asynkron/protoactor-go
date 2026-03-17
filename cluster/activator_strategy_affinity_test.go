package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalAffinityStrategy_PrefersLocal(t *testing.T) {
	s := NewLocalAffinityStrategy()
	local := newTestMember("m1", "127.0.0.1", 8080, "myKind")
	remote := newTestMember("m2", "192.168.1.1", 8080, "myKind")
	s.AddMember(local)
	s.AddMember(remote)

	// Should always return the local member when senderAddress matches.
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:8080")
		assert.Equal(t, local, r, "should prefer local member")
	}
}

func TestLocalAffinityStrategy_FallsBackToRoundRobin(t *testing.T) {
	s := NewLocalAffinityStrategy()
	m1 := newTestMember("m1", "192.168.1.1", 8080, "myKind")
	m2 := newTestMember("m2", "192.168.1.2", 8080, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	// Sender is not in the member list — should fall back to round-robin.
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "10.0.0.1:9999")
		if r != nil {
			seen[r.Id] = true
		}
	}
	assert.True(t, seen["m1"], "should cycle to m1")
	assert.True(t, seen["m2"], "should cycle to m2")
}

func TestLocalAffinityStrategy_LocalDoesNotSupportKind(t *testing.T) {
	s := NewLocalAffinityStrategy()
	local := newTestMember("m1", "127.0.0.1", 8080, "kindA")
	remote := newTestMember("m2", "192.168.1.1", 8080, "kindB")
	s.AddMember(local)
	s.AddMember(remote)

	// Local supports kindA but request is for kindB — should return remote.
	r := s.GetActivator(newTestCI("kindB", "id1"), "127.0.0.1:8080")
	assert.Equal(t, remote, r)
}

func TestLocalAffinityStrategy_ReturnsNilWhenNoMembers(t *testing.T) {
	s := NewLocalAffinityStrategy()
	r := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Nil(t, r)
}

func TestLocalAffinityStrategy_RemoveMember(t *testing.T) {
	s := NewLocalAffinityStrategy()
	m1 := newTestMember("m1", "127.0.0.1", 8080, "myKind")
	m2 := newTestMember("m2", "192.168.1.1", 8080, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.RemoveMember(m1)

	r := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:8080")
	assert.Equal(t, m2, r, "after removing local, should fall back to remote")
}

func TestLocalAffinityStrategy_Close(t *testing.T) {
	s := NewLocalAffinityStrategy()
	assert.NotPanics(t, func() { s.Close() })
}
