package cluster

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/emptypb"
)

// testProvider implements an in-memory ClusterProvider used only by tests.
// Tests use it to manually manage the member list so topology changes and
// recoveries can be simulated without real networking.
type testProvider struct {
	mu       sync.Mutex
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
// the cluster.
func (p *testProvider) StartMember(c *Cluster) error {
	host, port, _ := c.ActorSystem.GetHostPort()
	self := &Member{Host: host, Port: int32(port), Id: fmt.Sprintf("%s@%s:%d", c.Config.Name, host, port), Kinds: c.GetClusterKinds()}
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
	host, port, _ := c.ActorSystem.GetHostPort()
	id := fmt.Sprintf("%s@%s:%d", c.Config.Name, host, port)
	p.mu.Lock()
	delete(p.members, id)
	p.mu.Unlock()
	p.publish()
}

// membershipRecovery restarts a previously stopped cluster node and
// re-registers it with the test provider to mimic a recovered member.
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

	fut = c1.ActorSystem.Root.RequestFuture(pid2, &emptypb.Empty{}, 200*time.Millisecond)
	_, err = fut.Result()
	assert.Error(t, err)
	assert.Equal(t, 1, c1.MemberList.Length())

	// node recovers with a fresh cluster instance
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
