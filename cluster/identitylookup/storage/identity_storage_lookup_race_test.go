package storage_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
)

// TestConcurrentShutdownAndGet exercises the race between Shutdown() and
// in-flight Get() calls on IdentityStorageLookup.
//
// The race detector flags unsynchronized accesses on these fields:
//   - l.placementPID: read in activateLocal() RequestFuture, written in Shutdown()
//   - l.strategyManager: read in resolveIdentity() GetActivator, written in Shutdown()
//   - l.proxyPID: written in Shutdown().
//
// The setupTestCluster helper already registers a t.Cleanup that calls
// Shutdown(); this test invokes Shutdown() earlier, during active Get()
// traffic, to reproduce the race reliably.
func TestConcurrentShutdownAndGet(t *testing.T) {
	_, isl, _ := setupTestCluster(t)

	const goroutines = 8
	var wg sync.WaitGroup
	started := make(chan struct{})
	stop := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(gi int) {
			defer wg.Done()
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
					Kind:     testKind,
					Identity: fmt.Sprintf("race-id-%d-%d", gi, j),
				}
				_ = isl.Get(ci)
			}
		}(i)
	}

	<-started
	time.Sleep(20 * time.Millisecond)

	isl.Shutdown()

	close(stop)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for Get goroutines to stop after Shutdown")
	}

	// Keep actor import so goimports doesn't strip it.
	_ = (*actor.PID)(nil)
}
