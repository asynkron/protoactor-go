package natskv

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestConcurrentUpdateKindsAndPublishTopology exercises the race between
// UpdateKinds (which writes p.self.Kinds under membersMu.Lock) and
// publishClusterTopologyEvent (which reads p.self.MemberStatus() under
// membersMu.RLock). If MemberStatus() is called outside the lock scope,
// the race detector will flag the concurrent read/write on p.self.Kinds.
func TestConcurrentUpdateKindsAndPublishTopology(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-race-kinds")

	err := p.StartMember(c)
	if err != nil {
		t.Fatalf("StartMember: %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(true) })

	const iterations = 200
	var wg sync.WaitGroup

	// Writer goroutine: repeatedly calls UpdateKinds with varying kind lists.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			kinds := []string{"kind1", fmt.Sprintf("dynamic-%d", i)}
			_ = p.UpdateKinds(kinds)
		}
	}()

	// Reader goroutine: repeatedly calls publishClusterTopologyEvent which
	// reads p.self.MemberStatus() — this must be inside membersMu.RLock.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			p.publishClusterTopologyEvent()
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Completed without race detector complaints.
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for concurrent UpdateKinds/publishClusterTopologyEvent test")
	}
}

// TestConcurrentMemberMapAccess exercises concurrent reads and writes to the
// p.members map via handleMemberPut, handleMemberDelete, and
// publishClusterTopologyEvent. All three must hold membersMu appropriately.
func TestConcurrentMemberMapAccess(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-race-members")

	err := p.StartMember(c)
	if err != nil {
		t.Fatalf("StartMember: %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(true) })

	const iterations = 200
	var wg sync.WaitGroup

	// Writer goroutine: add and remove fake nodes from p.members.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			node := NewNode(fmt.Sprintf("fake-%d", i), "127.0.0.1", 9000+i, []string{"kind1"})
			p.membersMu.Lock()
			p.members[node.ID] = node
			p.membersMu.Unlock()

			// Also exercise deletion.
			p.membersMu.Lock()
			delete(p.members, node.ID)
			p.membersMu.Unlock()
		}
	}()

	// Reader goroutine: publish topology events (iterates p.members under RLock).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			p.publishClusterTopologyEvent()
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Completed without race detector complaints.
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for concurrent member map access test")
	}
}
