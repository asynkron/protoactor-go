package natsstream

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/stretchr/testify/require"
)

// TestConcurrentShutdownAndGet exercises the race between Shutdown() and an
// in-flight Get() call on the IdentityLookup.
//
// The race detector flags unsynchronized accesses on these shared fields:
//   - il.placementPID: read in activateLocal() RequestFuture, written in Shutdown()
//   - il.strategyMgr: read in resolveIdentity() GetActivator, written in Shutdown()
//   - il.proxyPID: (written in Shutdown; not read by Get path but exposed)
//
// The fire pattern mirrors the field report for natskv: a reconciler tick is
// mid-Get while the test harness tears down the cluster.
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
					Kind:     "TestKind",
					Identity: fmt.Sprintf("race-id-%d-%d", gi, j),
				}
				_ = il.Get(ci)
			}
		}(i)
	}

	<-started
	time.Sleep(20 * time.Millisecond)

	il.Shutdown()

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
}
