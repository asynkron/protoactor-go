package cluster

import (
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/emptypb"
)

// testProvider implements an in-memory ClusterProvider used only by tests.
// Tests use it to manually manage the member list so topology changes and
// recoveries can be simulated without real networking. Members are keyed by
// their ActorSystem IDs rather than address, mirroring real cluster semantics
// where a rebooted node receives a new system ID and must be treated as a
// completely new member.
type testProvider struct {
        mu sync.Mutex
        // members are keyed by ActorSystem ID to emulate node identity in the
        // production provider. A restarted node gets a new ID, so using the ID
        // as the map key prevents accidental reuse of stale memberships.
        members  map[string]*Member
        clusters []*Cluster
}

// newTestProvider constructs a fresh testProvider for unit tests.
func newTestProvider() *testProvider {
	return &testProvider{members: make(map[string]*Member)}
}

// publish broadcasts the current member set to all registered clusters.
// It emulates the membership dissemination a real provider would perform.
func (p *testProvider) publish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	var ms Members
	for _, m := range p.members {
		ms = append(ms, m)
	}
	for _, c := range p.clusters {
		c.MemberList.UpdateClusterTopology(ms)
	}
}

// StartMember registers the given cluster with the provider and immediately
// publishes the updated topology. Tests call this to simulate a node joining
// the cluster. A member's ID is its ActorSystem's unique ID, matching the real
// cluster providers' behaviour.
func (p *testProvider) StartMember(c *Cluster) error {
	host, port, _ := c.ActorSystem.GetHostPort()
	self := &Member{Host: host, Port: int32(port), Id: c.ActorSystem.ID, Kinds: c.GetClusterKinds()}
	p.mu.Lock()
	p.members[self.Id] = self
	p.clusters = append(p.clusters, c)
	p.mu.Unlock()
	p.publish()
	return nil
}

// StartClient mirrors StartMember because the test provider makes no
// distinction between clients and members.
func (p *testProvider) StartClient(c *Cluster) error { return p.StartMember(c) }

// Shutdown is a no-op since the test provider holds no external resources.
func (p *testProvider) Shutdown(bool) error { return nil }

// removeCluster deletes the cluster from the member list and broadcasts the
// change. Tests use it to simulate a node failure.
func (p *testProvider) removeCluster(c *Cluster) {
	id := c.ActorSystem.ID
	p.mu.Lock()
	delete(p.members, id)
	p.mu.Unlock()
	p.publish()
}

// membershipRecovery starts the supplied cluster node. Tests invoke this on a
// freshly created cluster instance to mimic a rebooted member rejoining with a
// new ActorSystem ID.
func membershipRecovery(c *Cluster) {
	c.StartMember()
}

// TestCluster_MembershipRecovery verifies member list and routing behaviour
// when a node goes down and later rejoins the cluster.
func TestCluster_MembershipRecovery(t *testing.T) {
	prov := newTestProvider()
	kind := NewKind("echo", actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	}))

	c1 := newClusterForTest("node1", prov, WithKinds(kind))
	c2 := newClusterForTest("node2", prov, WithKinds(kind))

	c1.StartMember()
	c2.StartMember()

	// spawn echo actor on second node and cache its PID in the first
	pid2, err := c2.ActorSystem.Root.SpawnNamed(kind.Props, "echo")
	assert.NoError(t, err)
	c1.PidCache.Set("echo", "echo", pid2)

	fut := c1.ActorSystem.Root.RequestFuture(pid2, &emptypb.Empty{}, time.Second)
	res, err := fut.Result()
	assert.NoError(t, err)
	_, ok := res.(*emptypb.Empty)
	assert.True(t, ok)

	// simulate node loss
	prov.removeCluster(c2)
	c2.Shutdown(true)

	// the provider broadcasts the removal which clears the PID cache
	_, ok = c1.PidCache.Get("echo", "echo")
	assert.False(t, ok)
	assert.Equal(t, 1, c1.MemberList.Length())

	// node recovers with a fresh cluster instance and new system ID
	c2a := newClusterForTest("node2", prov, WithKinds(kind))
	membershipRecovery(c2a)
	pid2a, err := c2a.ActorSystem.Root.SpawnNamed(kind.Props, "echo")
	assert.NoError(t, err)
	c1.PidCache.Set("echo", "echo", pid2a)

	fut = c1.ActorSystem.Root.RequestFuture(pid2a, &emptypb.Empty{}, time.Second)
	res, err = fut.Result()
	assert.NoError(t, err)
	_, ok = res.(*emptypb.Empty)
	assert.True(t, ok)
	assert.Equal(t, 2, c1.MemberList.Length())
}
