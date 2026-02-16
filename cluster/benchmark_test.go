package cluster

import (
	"fmt"
	"testing"

	"github.com/asynkron/protoactor-go/eventstream"
)

// BenchmarkRendezvousHashing_10Members benchmarks GetByClusterIdentity with
// 10 cluster members, which is a common small-cluster size. This exercises
// the FNV1A32 hashing and member filtering hot path.
func BenchmarkRendezvousHashing_10Members(b *testing.B) {
	members := newMembersForTest(10)
	r := NewRendezvous()
	r.UpdateMembers(members)

	ci := &ClusterIdentity{
		Kind:     "kind",
		Identity: "test-identity-0123456789",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr := r.GetByClusterIdentity(ci)
		if addr == "" {
			b.Fatal("empty address")
		}
	}
}

// BenchmarkRendezvousHashing_100Members benchmarks GetByClusterIdentity with
// 100 cluster members, exercising linear scan scaling behavior.
func BenchmarkRendezvousHashing_100Members(b *testing.B) {
	members := newMembersForTest(100)
	r := NewRendezvous()
	r.UpdateMembers(members)

	ci := &ClusterIdentity{
		Kind:     "kind",
		Identity: "test-identity-0123456789",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr := r.GetByClusterIdentity(ci)
		if addr == "" {
			b.Fatal("empty address")
		}
	}
}

// BenchmarkRendezvousHashing_VaryingIdentities benchmarks GetByClusterIdentity
// with different identity strings to measure per-lookup cost including the
// byte conversion and hashing of varying keys.
func BenchmarkRendezvousHashing_VaryingIdentities(b *testing.B) {
	members := newMembersForTest(10)
	r := NewRendezvous()
	r.UpdateMembers(members)

	// Pre-generate identities to avoid allocation in the hot loop.
	identities := make([]*ClusterIdentity, 1000)
	for i := 0; i < 1000; i++ {
		identities[i] = &ClusterIdentity{
			Kind:     "kind",
			Identity: fmt.Sprintf("identity-%d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr := r.GetByClusterIdentity(identities[i%1000])
		if addr == "" {
			b.Fatal("empty address")
		}
	}
}

// BenchmarkRendezvousUpdateMembers benchmarks the cost of updating the member
// list, which happens on topology changes. This involves rebuilding the
// internal member data and acquiring a write lock.
func BenchmarkRendezvousUpdateMembers(b *testing.B) {
	members := newMembersForTest(10)
	r := NewRendezvous()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.UpdateMembers(members)
	}
}

// BenchmarkEventStreamPublish benchmarks the event stream publish path with
// a single subscriber. This is the core mechanism for distributing topology
// changes, dead letters, and other system events.
func BenchmarkEventStreamPublish(b *testing.B) {
	es := eventstream.NewEventStream()
	received := 0
	es.Subscribe(func(evt interface{}) {
		received++
	})

	evt := "test-event"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		es.Publish(evt)
	}
}

// BenchmarkEventStreamPublish_10Subscribers benchmarks event stream publish
// fan-out to 10 subscribers, simulating a moderately loaded system.
func BenchmarkEventStreamPublish_10Subscribers(b *testing.B) {
	es := eventstream.NewEventStream()
	for i := 0; i < 10; i++ {
		es.Subscribe(func(evt interface{}) {})
	}

	evt := "test-event"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		es.Publish(evt)
	}
}

// BenchmarkEventStreamPublish_WithPredicate benchmarks event stream publish
// where subscribers use predicates to filter messages.
func BenchmarkEventStreamPublish_WithPredicate(b *testing.B) {
	es := eventstream.NewEventStream()
	es.SubscribeWithPredicate(
		func(evt interface{}) {},
		func(evt interface{}) bool {
			_, ok := evt.(string)
			return ok
		},
	)

	evt := "test-event"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		es.Publish(evt)
	}
}

// BenchmarkTopologyHash benchmarks the TopologyHash computation, which is used
// to detect topology changes efficiently.
func BenchmarkTopologyHash(b *testing.B) {
	members := newMembersForTest(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Copy to avoid mutating the sorted order across iterations
		cp := make(Members, len(members))
		copy(cp, members)
		TopologyHash(cp)
	}
}
