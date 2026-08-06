package natskv

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

// severableProxy is a minimal TCP proxy that can be "blocked" to simulate a
// network partition: while blocked it closes every live connection and rejects
// new ones, so a client routed through it is forced offline and keeps retrying
// until the proxy is unblocked. It is used to give one provider's NATS
// connection a controllable gap without disturbing any other client.
type severableProxy struct {
	ln      net.Listener
	target  string
	blocked atomic.Bool
	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	closed  atomic.Bool
}

func newSeverableProxy(t *testing.T, target string) *severableProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "proxy listen")
	p := &severableProxy{ln: ln, target: target, conns: make(map[net.Conn]struct{})}
	go p.acceptLoop()
	t.Cleanup(p.stop)
	return p
}

func (p *severableProxy) addr() string { return p.ln.Addr().String() }

func (p *severableProxy) acceptLoop() {
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return // listener closed
		}
		if p.blocked.Load() {
			_ = c.Close()
			continue
		}
		go p.handle(c)
	}
}

func (p *severableProxy) handle(client net.Conn) {
	upstream, err := net.Dial("tcp", p.target)
	if err != nil {
		_ = client.Close()
		return
	}
	p.track(client)
	p.track(upstream)
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
	_ = client.Close()
	_ = upstream.Close()
	p.untrack(client)
	p.untrack(upstream)
}

func (p *severableProxy) track(c net.Conn)   { p.mu.Lock(); p.conns[c] = struct{}{}; p.mu.Unlock() }
func (p *severableProxy) untrack(c net.Conn) { p.mu.Lock(); delete(p.conns, c); p.mu.Unlock() }

// block partitions every client routed through the proxy and rejects new
// connections until unblock is called.
func (p *severableProxy) block() {
	p.blocked.Store(true)
	p.mu.Lock()
	for c := range p.conns {
		_ = c.Close()
	}
	p.mu.Unlock()
}

func (p *severableProxy) unblock() { p.blocked.Store(false) }

func (p *severableProxy) stop() {
	if p.closed.Swap(true) {
		return
	}
	_ = p.ln.Close()
	p.mu.Lock()
	for c := range p.conns {
		_ = c.Close()
	}
	p.mu.Unlock()
}

// setupObserverViaProxy builds a provider whose NATS connection is routed
// through the severable proxy, so its watcher can be partitioned on demand.
func setupObserverViaProxy(t *testing.T, proxyAddr, clusterName string, opts ...Option) (*Provider, *cluster.Cluster) {
	t.Helper()
	nc, err := nats.Connect("nats://"+proxyAddr,
		nats.ReconnectWait(150*time.Millisecond),
		nats.MaxReconnects(-1),
		nats.RetryOnFailedConnect(true),
		nats.Timeout(1*time.Second),
	)
	require.NoError(t, err, "observer connect via proxy")
	t.Cleanup(func() { nc.Close() })

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteConfig)
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)
	return p, c
}

// TestMissedDelete_ReconcileRecoversStrandedMember reproduces the exact failure
// that wedged the cluster and proves the reconcile is what recovers from it,
// end-to-end through real NATS:
//
//  1. Observer A (watcher + reconcile) discovers a member B from a real KV Put.
//  2. A's connection is severed (partition) so its watcher goes offline.
//  3. B's key is deleted during the gap, via a separate connection, so the
//     delete event is never delivered to A's watcher.
//  4. A reconnects; its watcher re-subscribes with UpdatesOnly and therefore
//     never sees the delete -- B is stranded in A's topology.
//
// With reconcile ENABLED, A prunes the stranded B within a couple of intervals.
// The companion test below runs the identical scenario with reconcile DISABLED
// and shows B persists, so this is a genuine failure-on-demand / fix-on-demand
// pair, not an assertion against injected state.
func TestMissedDelete_ReconcileRecoversStrandedMember(t *testing.T) {
	if pruned := runMissedDeleteScenario(t, true); !pruned {
		t.Fatal("reconcile should have pruned the stranded member after the missed delete")
	}
}

// TestMissedDelete_WithoutReconcileMemberStranded runs the same missed-delete
// scenario with reconcile disabled and confirms the stranded member persists --
// the watcher alone cannot recover, which is the bug the reconcile fixes.
func TestMissedDelete_WithoutReconcileMemberStranded(t *testing.T) {
	if pruned := runMissedDeleteScenario(t, false); pruned {
		t.Fatal("without reconcile the stranded member must persist (watcher missed the delete)")
	}
}

// runMissedDeleteScenario drives the missed-delete sequence and reports whether
// the observer ended up pruning the stranded member.
func runMissedDeleteScenario(t *testing.T, reconcileEnabled bool) bool {
	t.Helper()
	srv := startEmbeddedNATS(t)
	proxy := newSeverableProxy(t, srv.Addr().String())
	ctx := context.Background()

	clusterName := "test-missed-delete"
	// Short TTL/intervals keep the test fast; TTL must exceed the partition
	// window is NOT required -- we delete B's key explicitly during the gap.
	opts := []Option{
		WithMemberTTL(2 * time.Second),
		WithRefreshInterval(400 * time.Millisecond),
	}
	if reconcileEnabled {
		opts = append(opts, WithReconcileInterval(400*time.Millisecond))
	} else {
		opts = append(opts, WithReconcileInterval(-1))
	}

	pA, cA := setupObserverViaProxy(t, proxy.addr(), clusterName, opts...)
	require.NoError(t, pA.StartMember(cA))
	t.Cleanup(func() { _ = pA.Shutdown(true) })

	// Control connection straight to NATS (not through the proxy): used to plant
	// and later delete B's member key while A is partitioned.
	_, jsCtl := connectNATS(t, srv)
	bktCtl, err := jsCtl.KeyValue(ctx, pA.config.memberBucketName(clusterName))
	require.NoError(t, err, "bind member bucket via control connection")

	victimID := clusterName + "_victim-01"
	victim := NewNode(victimID, "10.0.0.77", 6001, nil)
	vdata, err := victim.Serialize()
	require.NoError(t, err)
	_, err = bktCtl.Put(ctx, pA.memberKey(victimID), vdata)
	require.NoError(t, err, "plant victim member key")

	// A's watcher must observe the victim before we partition it.
	require.Eventually(t, func() bool {
		pA.membersMu.RLock()
		_, ok := pA.members[victimID]
		pA.membersMu.RUnlock()
		return ok
	}, 10*time.Second, 100*time.Millisecond, "observer should discover the victim member")

	// Sever the observer's watcher, then delete the victim's key during the gap
	// so the delete event is never delivered to the (offline) watcher.
	proxy.block()
	err = bktCtl.Delete(ctx, pA.memberKey(victimID))
	require.NoError(t, err, "delete victim key during partition")

	// Hold the partition well past the bucket's MaxAge (== MemberTTL, 2s) so the
	// delete marker is purged from the stream. On reconnect the watcher's
	// ordered consumer has nothing to replay for the victim, so it genuinely
	// misses the delete -- the condition only the reconcile can recover from.
	// (A shorter gap lets the resuming consumer redeliver the marker, and the
	// watcher self-recovers without the reconcile.)
	time.Sleep(6 * time.Second)

	// Reconnect the observer. Its watcher re-subscribes with UpdatesOnly and
	// will not see the delete that happened while it was offline.
	proxy.unblock()

	if reconcileEnabled {
		pruned := func() bool {
			deadline := time.Now().Add(15 * time.Second)
			for time.Now().Before(deadline) {
				pA.membersMu.RLock()
				_, ok := pA.members[victimID]
				pA.membersMu.RUnlock()
				if !ok {
					return true
				}
				time.Sleep(200 * time.Millisecond)
			}
			return false
		}()
		return pruned
	}

	// Reconcile disabled: give the reconnected watcher ample time; the stranded
	// member must still be present (only the reconcile could have removed it).
	time.Sleep(6 * time.Second)
	pA.membersMu.RLock()
	_, ok := pA.members[victimID]
	pA.membersMu.RUnlock()
	return !ok
}
