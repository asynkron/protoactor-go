package cluster

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inmemoryProvider use for test
type inmemoryProvider struct {
	cluster *Cluster
	members map[string]*Member
	self    *Member
}

func newInmemoryProvider() *inmemoryProvider {
	return &inmemoryProvider{members: map[string]*Member{}}
}

func (p *inmemoryProvider) init(c *Cluster) error {
	name := c.Config.Name
	host, port, err := c.ActorSystem.GetHostPort()
	if err != nil {
		return err
	}
	p.cluster = c
	p.self = &Member{
		Host:  host,
		Port:  int32(port),
		Id:    fmt.Sprintf("%s@%s:%d", name, host, port),
		Kinds: c.GetClusterKinds(),
	}

	return nil
}

func (p *inmemoryProvider) publishClusterTopologyEvent() {
	var members Members
	for _, m := range p.members {
		members = append(members, m)
	}

	res := members

	p.cluster.MemberList.UpdateClusterTopology(res)
	// p.cluster.ActorSystem.EventStream.Publish(res)
}

func (p *inmemoryProvider) StartMember(c *Cluster) error {
	err := p.init(c)
	if err != nil {
		return err
	}
	p.members[p.self.Id] = p.self
	p.publishClusterTopologyEvent()
	return nil
}

func (p *inmemoryProvider) StartClient(c *Cluster) error {
	err := p.init(c)
	if err != nil {
		return err
	}
	p.publishClusterTopologyEvent()
	return nil
}

func (p *inmemoryProvider) Shutdown(graceful bool) error {
	delete(p.members, p.self.Id)

	return nil
}

type fakeIdentityLookup struct {
	m       sync.Map
	cluster *Cluster
}

func (l *fakeIdentityLookup) Get(identity *ClusterIdentity) *actor.PID {
	if val, ok := l.m.Load(identity.Identity); ok {
		return val.(*actor.PID)
	}
	// if the kind is registered, spawn the actor on first lookup
	if l.cluster != nil {
		if kind := l.cluster.GetClusterKind(identity.Kind); kind != nil {
			props := WithClusterIdentity(kind.Props, identity)
			pid, err := l.cluster.ActorSystem.Root.SpawnNamed(props, identity.Identity)
			if err == nil {
				l.m.Store(identity.Identity, pid)
				return pid
			}
		}
	}
	return nil
}

func (l *fakeIdentityLookup) RemovePid(identity *ClusterIdentity, pid *actor.PID) {
	if existPid := l.Get(identity); existPid != nil && existPid.Equal(pid) {
		l.m.Delete(identity.Identity)
	}
}

func (lu *fakeIdentityLookup) Setup(cluster *Cluster, kinds []string, isClient bool) {
	lu.cluster = cluster
}

func (lu *fakeIdentityLookup) Shutdown() {
}

func (l *fakeIdentityLookup) Peek(identity *ClusterIdentity) (*PeekResult, error) {
	if val, ok := l.m.Load(identity.Identity); ok {
		pid := val.(*actor.PID)
		return &PeekResult{
			GrainInfo: &GrainInfo{
				Identity: identity.Identity,
				Kind:     identity.Kind,
				PID:      pid,
			},
			Status: PeekStatusAlive,
		}, nil
	}
	return &PeekResult{
		GrainInfo: &GrainInfo{
			Identity: identity.Identity,
			Kind:     identity.Kind,
		},
		Status: PeekStatusNotFound,
	}, nil
}

func newClusterForTest(name string, cp ClusterProvider, opts ...ConfigOption) *Cluster {
	system := actor.NewActorSystem()
	lookup := fakeIdentityLookup{}
	cfg := Configure(name, cp, &lookup, remote.Configure("127.0.0.1", 0), opts...)
	c := New(system, cfg)

	c.MemberList = NewMemberList(c)
	c.Config.RequestTimeoutTime = 1 * time.Second
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	c.IdentityLookup = &lookup
	return c
}

func TestCluster_Call(t *testing.T) {
	assert := assert.New(t)

	members := Members{
		{
			Id:    "1",
			Host:  "nonhost",
			Port:  -1,
			Kinds: []string{"kind"},
		},
	}
	c := newClusterForTest("mycluster", newInmemoryProvider())
	c.MemberList.UpdateClusterTopology(members)
	t.Run("invalid kind", func(t *testing.T) {
		msg := struct{}{}
		resp, err := c.Request("name", "nonkind", &msg)
		assert.ErrorContains(err, "max retries")
		assert.Nil(resp)
	})

	testProps := actor.PropsFromFunc(
		func(context actor.Context) {
			switch msg := context.Message().(type) {
			case *struct{ Code int }:
				msg.Code++
				context.Respond(msg)
			}
		})
	pid := c.ActorSystem.Root.Spawn(testProps)
	assert.NotNil(pid)
	c.PidCache.Set("name", "kind", pid)
	t.Run("normal", func(t *testing.T) {
		msg := struct{ Code int }{9527}
		resp, err := c.Request("name", "kind", &msg)
		assert.NoError(err)
		assert.Equal(&struct{ Code int }{9528}, resp)
	})

	t.Run("timeout", func(t *testing.T) {
		msg := struct{}{}
		resp, err := c.Request("name", "kind", &msg, WithTimeout(time.Millisecond))
		assert.ErrorIs(err, context.DeadlineExceeded)
		assert.Nil(resp)
	})
}

func TestCluster_Get(t *testing.T) {
	cp := newInmemoryProvider()
	kind := NewKind("kind", actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started:
			_ = msg
		}
	}))
	c := newClusterForTest("mycluster", cp, WithKinds(kind))
	err := c.StartMember()
	assert.NoError(t, err)
	cp.publishClusterTopologyEvent()
	t.Run("invalid kind", func(t *testing.T) {
		assert := assert.New(t)
		assert.Equal(1, c.MemberList.Length())
		pid := c.Get("name", "nonkind")
		assert.Nil(pid)
	})

	t.Run("ok", func(t *testing.T) {
		assert := assert.New(t)
		pid := c.Get("name", "kind")
		assert.NotNil(pid)
	})
}

func TestClusterPeek_DelegatesToIdentityLookup(t *testing.T) {
	cp := newInmemoryProvider()
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	kind := NewKind("test-kind", props)
	c := newClusterForTest("peek-test", cp, WithKinds(kind))
	err := c.StartMember()
	require.NoError(t, err)
	defer c.Shutdown(true)

	result, err := c.Peek("some-identity", "test-kind")
	require.NoError(t, err)
	assert.Equal(t, PeekStatusNotFound, result.Status)
	assert.Equal(t, "some-identity", result.Identity)
	assert.Equal(t, "test-kind", result.Kind)
}

func TestCluster_Shutdown_Graceful(t *testing.T) {
	cp := newInmemoryProvider()
	c := newClusterForTest("test-shutdown", cp)

	err := c.StartMember()
	assert.NoError(t, err)

	// Should not panic
	assert.NotPanics(t, func() {
		c.Shutdown(true)
	})
}

func TestCluster_Shutdown_NotGraceful(t *testing.T) {
	cp := newInmemoryProvider()
	c := newClusterForTest("test-shutdown-fast", cp)

	err := c.StartMember()
	assert.NoError(t, err)

	assert.NotPanics(t, func() {
		c.Shutdown(false)
	})
}

func TestNewCluster_DefaultPidCacheTTL_NoExpiry(t *testing.T) {
	c := newClusterForTest("test-default-ttl", newInmemoryProvider())

	pid := actor.NewPID("localhost:8080", "test/grain-1")
	c.PidCache.Set("grain-1", "test", pid)

	// With zero TTL, entry should persist indefinitely.
	time.Sleep(50 * time.Millisecond)
	got, ok := c.PidCache.Get("grain-1", "test")
	assert.True(t, ok, "entry should still exist with zero TTL (default)")
	assert.True(t, pid.Equal(got), "cached PID should match")
}

func TestNewCluster_WithPidCacheTTL_ExpiresCachedEntries(t *testing.T) {
	c := newClusterForTest("test-ttl-expiry", newInmemoryProvider(),
		WithPidCacheTTL(100*time.Millisecond),
	)

	pid := actor.NewPID("localhost:8080", "test/grain-1")
	c.PidCache.Set("grain-1", "test", pid)

	// Entry should exist immediately.
	got, ok := c.PidCache.Get("grain-1", "test")
	assert.True(t, ok, "entry should exist before TTL")
	assert.True(t, pid.Equal(got))

	// After TTL elapses, entry should be gone.
	time.Sleep(150 * time.Millisecond)
	_, ok = c.PidCache.Get("grain-1", "test")
	assert.False(t, ok, "entry should have expired after TTL")
}
