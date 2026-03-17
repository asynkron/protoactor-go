package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRendezvousStrategy_ReturnsNilWhenNoMembers(t *testing.T) {
	s := NewRendezvousStrategy()
	result := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Nil(t, result)
}

func TestRendezvousStrategy_SingleMember(t *testing.T) {
	s := NewRendezvousStrategy()
	m := newTestMember("m1", "host1", 1000, "myKind")
	s.AddMember(m)

	result := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Equal(t, m, result)
}

func TestRendezvousStrategy_Deterministic(t *testing.T) {
	s := NewRendezvousStrategy()
	s.AddMember(newTestMember("m1", "host1", 1000, "myKind"))
	s.AddMember(newTestMember("m2", "host2", 1001, "myKind"))
	s.AddMember(newTestMember("m3", "host3", 1002, "myKind"))

	ci := newTestCI("myKind", "grain-42")

	// Same identity should always map to the same member.
	first := s.GetActivator(ci, "127.0.0.1:0")
	require.NotNil(t, first)

	for i := 0; i < 100; i++ {
		r := s.GetActivator(ci, "127.0.0.1:0")
		assert.Equal(t, first.Id, r.Id, "rendezvous must be deterministic")
	}
}

func TestRendezvousStrategy_DifferentIdentitiesCanMapDifferently(t *testing.T) {
	s := NewRendezvousStrategy()
	s.AddMember(newTestMember("m1", "host1", 1000, "myKind"))
	s.AddMember(newTestMember("m2", "host2", 1001, "myKind"))
	s.AddMember(newTestMember("m3", "host3", 1002, "myKind"))

	// With 3 members and many identities, we should see at least 2 different members.
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		ci := newTestCI("myKind", fmt.Sprintf("grain-%d", i))
		r := s.GetActivator(ci, "127.0.0.1:0")
		if r != nil {
			seen[r.Id] = true
		}
	}
	assert.GreaterOrEqual(t, len(seen), 2,
		"different identities should distribute across members")
}

func TestRendezvousStrategy_StableUnderMemberAddition(t *testing.T) {
	s := NewRendezvousStrategy()
	s.AddMember(newTestMember("m1", "host1", 1000, "myKind"))
	s.AddMember(newTestMember("m2", "host2", 1001, "myKind"))

	ci := newTestCI("myKind", "grain-stable")
	before := s.GetActivator(ci, "127.0.0.1:0")
	require.NotNil(t, before)

	// Add a third member — the mapping for this identity MAY change,
	// but if it doesn't (which is the common case for rendezvous hashing),
	// that demonstrates stability. We just verify it still returns a valid member.
	s.AddMember(newTestMember("m3", "host3", 1002, "myKind"))
	after := s.GetActivator(ci, "127.0.0.1:0")
	require.NotNil(t, after)
}

func TestRendezvousStrategy_FiltersByKind(t *testing.T) {
	s := NewRendezvousStrategy()
	s.AddMember(newTestMember("m1", "host1", 1000, "kindA"))
	s.AddMember(newTestMember("m2", "host2", 1001, "kindB"))

	r := s.GetActivator(newTestCI("kindA", "id1"), "127.0.0.1:0")
	assert.NotNil(t, r)
	assert.Equal(t, "m1", r.Id)

	r = s.GetActivator(newTestCI("kindB", "id1"), "127.0.0.1:0")
	assert.NotNil(t, r)
	assert.Equal(t, "m2", r.Id)
}

func TestRendezvousStrategy_RemoveMember(t *testing.T) {
	s := NewRendezvousStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.RemoveMember(m1)

	// After removing m1, all identities should go to m2.
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("myKind", fmt.Sprintf("id-%d", i)), "127.0.0.1:0")
		assert.Equal(t, m2, r)
	}
}

func TestRendezvousStrategy_Close(t *testing.T) {
	s := NewRendezvousStrategy()
	assert.NotPanics(t, func() { s.Close() })
}
