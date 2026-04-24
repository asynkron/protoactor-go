package cluster

import (
	"testing"

	"github.com/awevoke/protoactor-go/eventstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestGossipStrategy_ReturnsNilWhenNoMembers(t *testing.T) {
	s := NewGossipStrategy(nil)
	defer s.Close()
	result := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Nil(t, result)
}

func TestGossipStrategy_FallsBackToRoundRobinWithNoGossipData(t *testing.T) {
	s := NewGossipStrategy(nil)
	defer s.Close()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	// With no gossip data, should cycle through members (round-robin).
	seen := map[string]int{}
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
		require.NotNil(t, r)
		seen[r.Id]++
	}
	assert.Equal(t, 5, seen["m1"])
	assert.Equal(t, 5, seen["m2"])
}

func TestGossipStrategy_SelectsLeastLoadedMember(t *testing.T) {
	s := NewGossipStrategy(nil)
	defer s.Close()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	m3 := newTestMember("m3", "host3", 1002, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.AddMember(m3)

	// Inject gossip data directly.
	s.mu.Lock()
	s.actorCounts["m1"] = map[string]int64{"myKind": 100}
	s.actorCounts["m2"] = map[string]int64{"myKind": 5}
	s.actorCounts["m3"] = map[string]int64{"myKind": 50}
	s.mu.Unlock()

	// m2 has the fewest actors, should always be selected.
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
		require.NotNil(t, r)
		assert.Equal(t, "m2", r.Id)
	}
}

func TestGossipStrategy_CustomScoreMember(t *testing.T) {
	// Custom scorer: prefer the member with the highest count (inverted).
	scorer := func(member *Member, kind string, counts map[string]int64) float64 {
		if counts == nil {
			return 0
		}
		return -float64(counts[kind])
	}
	s := NewGossipStrategy(nil, WithScoreMember(scorer))
	defer s.Close()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	s.mu.Lock()
	s.actorCounts["m1"] = map[string]int64{"myKind": 10}
	s.actorCounts["m2"] = map[string]int64{"myKind": 100}
	s.mu.Unlock()

	// Custom scorer inverts, so m2 (count 100 -> score -100) should win.
	r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	require.NotNil(t, r)
	assert.Equal(t, "m2", r.Id)
}

func TestGossipStrategy_RemoveMemberClearsGossipData(t *testing.T) {
	s := NewGossipStrategy(nil)
	defer s.Close()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	s.mu.Lock()
	s.actorCounts["m1"] = map[string]int64{"myKind": 10}
	s.actorCounts["m2"] = map[string]int64{"myKind": 20}
	s.mu.Unlock()

	s.RemoveMember(m1)

	// m1's gossip data should be gone.
	s.mu.RLock()
	_, exists := s.actorCounts["m1"]
	s.mu.RUnlock()
	assert.False(t, exists, "gossip data for removed member should be deleted")

	// Only m2 left.
	r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	require.NotNil(t, r)
	assert.Equal(t, "m2", r.Id)
}

func TestGossipStrategy_Close_Unsubscribes(t *testing.T) {
	es := eventstream.NewEventStream()
	s := NewGossipStrategy(es)

	assert.Equal(t, int32(1), es.Length(), "should have 1 subscriber")

	s.Close()
	assert.Equal(t, int32(0), es.Length(), "should have 0 subscribers after Close")

	// Double-close should not panic.
	assert.NotPanics(t, func() { s.Close() })
}

func TestGossipStrategy_KindFiltering(t *testing.T) {
	s := NewGossipStrategy(nil)
	defer s.Close()
	s.AddMember(newTestMember("m1", "host1", 1000, "kindA"))
	s.AddMember(newTestMember("m2", "host2", 1001, "kindB"))
	s.AddMember(newTestMember("m3", "host3", 1002, "kindA", "kindB"))

	s.mu.Lock()
	s.actorCounts["m1"] = map[string]int64{"kindA": 50}
	s.actorCounts["m2"] = map[string]int64{"kindB": 10}
	s.actorCounts["m3"] = map[string]int64{"kindA": 5, "kindB": 100}
	s.mu.Unlock()

	// For kindA: m1 has 50, m3 has 5 -> m3 wins.
	r := s.GetActivator(newTestCI("kindA", "id"), "127.0.0.1:0")
	require.NotNil(t, r)
	assert.Equal(t, "m3", r.Id)

	// For kindB: m2 has 10, m3 has 100 -> m2 wins.
	r = s.GetActivator(newTestCI("kindB", "id"), "127.0.0.1:0")
	require.NotNil(t, r)
	assert.Equal(t, "m2", r.Id)
}

func TestGossipStrategy_AddDuplicate(t *testing.T) {
	s := NewGossipStrategy(nil)
	defer s.Close()
	m := newTestMember("m1", "host1", 1000, "myKind")
	s.AddMember(m)
	s.AddMember(m) // duplicate

	s.mu.RLock()
	count := len(s.members)
	s.mu.RUnlock()
	assert.Equal(t, 1, count, "duplicate member should not be added")
}

func TestGossipStrategy_EventStreamUpdatesActorCounts(t *testing.T) {
	es := eventstream.NewEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	// Publish a gossip update for m1 with high count.
	hb1 := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{
			ActorCount: map[string]int64{"myKind": 100},
		},
	}
	val1, err := anypb.New(hb1)
	require.NoError(t, err)
	es.Publish(&GossipUpdate{
		MemberID: "m1",
		Key:      HeartbeatKey,
		Value:    val1,
	})

	// Publish a gossip update for m2 with low count.
	hb2 := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{
			ActorCount: map[string]int64{"myKind": 3},
		},
	}
	val2, err := anypb.New(hb2)
	require.NoError(t, err)
	es.Publish(&GossipUpdate{
		MemberID: "m2",
		Key:      HeartbeatKey,
		Value:    val2,
	})

	// m2 should be selected as least loaded.
	r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	require.NotNil(t, r)
	assert.Equal(t, "m2", r.Id)
}

func TestGossipStrategy_IgnoresNonHeartbeatUpdates(t *testing.T) {
	es := eventstream.NewEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	// Publish a non-heartbeat gossip update.
	es.Publish(&GossipUpdate{
		MemberID: "m1",
		Key:      "some-other-key",
		Value:    nil,
	})

	s.mu.RLock()
	count := len(s.actorCounts)
	s.mu.RUnlock()
	assert.Equal(t, 0, count, "non-heartbeat updates should be ignored")
}

func TestGossipStrategy_MemberWithNoGossipCountsTreatedAsZero(t *testing.T) {
	s := NewGossipStrategy(nil)
	defer s.Close()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	// Only m1 has gossip data; m2 has none -> m2 count defaults to 0.
	s.mu.Lock()
	s.actorCounts["m1"] = map[string]int64{"myKind": 10}
	s.mu.Unlock()

	r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	require.NotNil(t, r)
	assert.Equal(t, "m2", r.Id, "member with no gossip data should be treated as 0 count")
}
