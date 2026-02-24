//go:build integration

package natsstream

import (
	"slices"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
)

// setupClusterWithKinds is like setupClusterFromURL but also registers initial
// kinds via cluster.WithKinds so that the cluster starts with a known set of
// virtual actor kinds.
func setupClusterWithKinds(t *testing.T, natsURL, clusterName string, kinds []*cluster.Kind, opts ...Option) (*Provider, *cluster.Cluster) {
	t.Helper()
	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteCfg,
		cluster.WithKinds(kinds...),
	)
	c := cluster.NewCluster(system, clusterCfg)
	c.Remote = remote.NewRemote(system, remoteCfg)

	return p, c
}

// dummyKind creates a minimal Kind suitable for integration tests. The actor
// is never actually spawned; we only need the kind metadata to propagate
// through the cluster topology.
func dummyKind(name string) *cluster.Kind {
	return cluster.NewKind(name, actor.PropsFromProducer(func() actor.Actor {
		return &kindTestActor{}
	}))
}

type kindTestActor struct{}

func (d *kindTestActor) Receive(_ actor.Context) {}

// TestIntegration_RuntimeKindRegistration starts a 2-node cluster where each
// node initially has kindA. It then registers kindB on node 1 via
// UpdateKinds and verifies that node 2 discovers the new kind through
// normal topology convergence (heartbeat propagation).
func TestIntegration_RuntimeKindRegistration(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithHeartbeatInterval(500 * time.Millisecond),
		WithHeartbeatTTL(2 * time.Second),
		WithMemberTimeout(3 * time.Second),
		WithCheckInterval(500 * time.Millisecond),
	}

	kindA := dummyKind("kindA")

	p1, c1 := setupClusterWithKinds(t, natsURL, "integ-kind-reg",
		[]*cluster.Kind{kindA}, opts...)
	c1.InitKindsForTest(kindA)

	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterWithKinds(t, natsURL, "integ-kind-reg",
		[]*cluster.Kind{kindA}, opts...)
	c2.InitKindsForTest(kindA)

	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Wait for mutual discovery.
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 1 should discover member 2")

	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 2 should discover member 1")

	// Verify both nodes initially have only kindA.
	p1.membersMu.RLock()
	require.Equal(t, []string{"kindA"}, p1.self.Kinds, "node 1 should start with kindA only")
	p1.membersMu.RUnlock()

	// Register kindB on node 1 and propagate via UpdateKinds.
	kindB := dummyKind("kindB")
	c1.InitKindsForTest(kindB) // register locally on the cluster
	err = p1.UpdateKinds(c1.GetClusterKinds())
	require.NoError(t, err)

	// Verify node 1 now has both kindA and kindB locally.
	p1.membersMu.RLock()
	require.True(t, slices.Contains(p1.self.Kinds, "kindA"), "node 1 should still have kindA")
	require.True(t, slices.Contains(p1.self.Kinds, "kindB"), "node 1 should now have kindB")
	p1.membersMu.RUnlock()

	// Wait for node 2 to see that node 1 now has kindB.
	p1ID := p1.self.ID
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		defer p2.membersMu.RUnlock()
		node, ok := p2.members[p1ID]
		if !ok {
			return false
		}
		return slices.Contains(node.Kinds, "kindB")
	}, 15*time.Second, 200*time.Millisecond, "member 2 should see kindB on member 1")

	// Verify node 2's MemberList can find an activator for kindB.
	require.Eventually(t, func() bool {
		return c2.MemberList.GetActivatorMember("kindB", "") != ""
	}, 15*time.Second, 200*time.Millisecond, "member 2 MemberList should have an activator for kindB")
}

// TestIntegration_RuntimeKindDeregistration starts a 2-node cluster where
// each node initially has kindA and kindB. It then removes kindB from node 1
// via UpdateKinds and verifies the local state is updated.
func TestIntegration_RuntimeKindDeregistration(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithHeartbeatInterval(500 * time.Millisecond),
		WithHeartbeatTTL(2 * time.Second),
		WithMemberTimeout(3 * time.Second),
		WithCheckInterval(500 * time.Millisecond),
	}

	kindA := dummyKind("kindA")
	kindB := dummyKind("kindB")

	p1, c1 := setupClusterWithKinds(t, natsURL, "integ-kind-dereg",
		[]*cluster.Kind{kindA, kindB}, opts...)
	c1.InitKindsForTest(kindA, kindB)

	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterWithKinds(t, natsURL, "integ-kind-dereg",
		[]*cluster.Kind{kindA, kindB}, opts...)
	c2.InitKindsForTest(kindA, kindB)

	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Wait for mutual discovery.
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 2 should discover member 1")

	// Verify node 1 initially has both kinds.
	p1.membersMu.RLock()
	require.True(t, slices.Contains(p1.self.Kinds, "kindA"))
	require.True(t, slices.Contains(p1.self.Kinds, "kindB"))
	p1.membersMu.RUnlock()

	// Deregister kindB from the cluster and update the provider.
	err = c1.DeregisterKind("kindB")
	require.NoError(t, err)

	// Propagate the change to the provider.
	err = p1.UpdateKinds(c1.GetClusterKinds())
	require.NoError(t, err)

	// Verify node 1 no longer has kindB locally.
	p1.membersMu.RLock()
	require.True(t, slices.Contains(p1.self.Kinds, "kindA"), "node 1 should still have kindA")
	require.False(t, slices.Contains(p1.self.Kinds, "kindB"), "node 1 should no longer have kindB")
	p1.membersMu.RUnlock()

	// Wait for node 2 to see that node 1 no longer has kindB.
	p1ID := p1.self.ID
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		defer p2.membersMu.RUnlock()
		node, ok := p2.members[p1ID]
		if !ok {
			return false
		}
		return !slices.Contains(node.Kinds, "kindB")
	}, 15*time.Second, 200*time.Millisecond, "member 2 should see kindB removed from member 1")
}
