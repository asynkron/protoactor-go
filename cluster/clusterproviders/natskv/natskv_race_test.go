package natskv

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/require"
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

// TestConcurrentShutdownAndGet exercises the race between Shutdown() and an
// in-flight Get() call on the IdentityLookup.
//
// The race detector flags unsynchronized accesses on these shared fields:
//   - il.defunct: read in Get()/Peek(), written in Shutdown()
//   - il.placementPID: read in activateLocal() RequestFuture, written in Shutdown()
//   - il.strategyMgr: read in resolveIdentity() GetActivator, written in Shutdown()
//
// The fire pattern from the field report: a reconciler tick is mid-Get while
// the test harness tears down the cluster, triggering concurrent reads/writes.
func TestConcurrentShutdownAndGet(t *testing.T) {
	srv := startEmbeddedNATS(t)

	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	kind := cluster.NewKind("TestKind", kindProps)

	p, c := setupClusterWithKindsEmbedded(t, srv, "test-race-shutdown-get",
		[]*cluster.Kind{kind})

	err := c.Remote.Start()
	require.NoError(t, err)
	c.InitKindsForTest(kind)

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)

	host, port, err := c.ActorSystem.GetHostPort()
	require.NoError(t, err)
	self := &cluster.Member{
		Host:  host,
		Port:  int32(port),
		Id:    il.memberID,
		Kinds: []string{"TestKind"},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	t.Cleanup(func() { c.Remote.Shutdown(true) })

	// Launch goroutines calling Get() with unique identities (bypass PID cache)
	// so each call exercises the defunct check, strategyMgr lookup, and
	// placementPID RequestFuture path inside resolveIdentity/activateLocal.
	const goroutines = 8
	var wg sync.WaitGroup
	started := make(chan struct{})
	stop := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(gi int) {
			defer wg.Done()
			// Signal readiness.
			if gi == 0 {
				close(started)
			}
			for j := 0; ; j++ {
				select {
				case <-stop:
					return
				default:
				}
				ci := &cluster.ClusterIdentity{
					Kind:     "TestKind",
					Identity: fmt.Sprintf("race-id-%d-%d", gi, j),
				}
				_ = il.Get(ci)
			}
		}(i)
	}

	<-started
	// Let some Get() calls reach the critical reads (defunct, strategyMgr, placementPID).
	time.Sleep(20 * time.Millisecond)

	// Now invoke Shutdown concurrently. Race detector should flag the
	// concurrent reads/writes on il.defunct, il.placementPID, il.strategyMgr.
	il.Shutdown()

	close(stop)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Completed without race detector complaints (after fix).
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for Get goroutines to stop after Shutdown")
	}
}
