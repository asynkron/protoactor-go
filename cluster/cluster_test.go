package cluster

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
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
	m sync.Map
}

func (l *fakeIdentityLookup) Get(identity *ClusterIdentity) *actor.PID {
	if val, ok := l.m.Load(identity.Identity); ok {
		return val.(*actor.PID)
	} else {
		// pid := actor.NewPID("127.0.0.1", fmt.Sprintf("%s/%s", identity.Kind, identity.Identity))
		// l.m.Store(identity.Identity, pid)
		// return pid
	}
	return nil
}

func (l *fakeIdentityLookup) RemovePid(identity *ClusterIdentity, pid *actor.PID) {
	if existPid := l.Get(identity); existPid.Equal(pid) {
		l.m.Delete(identity.Identity)
	}
}

func (lu *fakeIdentityLookup) Setup(cluster *Cluster, kinds []string, isClient bool) {
}

func (lu *fakeIdentityLookup) Shutdown() {
}

func newClusterForTest(name string, cp ClusterProvider, opts ...ConfigOption) *Cluster {
	system := actor.NewActorSystem()
	lookup := fakeIdentityLookup{}
	cfg := Configure(name, cp, &lookup, remote.Configure("127.0.0.1", 0), opts...)
	c := New(system, cfg)

	c.MemberList = NewMemberList(c)
	c.Config.RequestTimeoutTime = 1 * time.Second
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	return c
}

func TestCluster_Call(t *testing.T) {
	t.Skipf("Maintaining")
	assert := assert.New(t)

	members := Members{
		{
			Id:    "1",
			Host:  "nonhost",
			Port:  -1,
			Kinds: []string{"kind"},
		},
	}
	c := newClusterForTest("mycluster", nil)
	c.MemberList.UpdateClusterTopology(members)
	t.Run("invalid kind", func(t *testing.T) {
		msg := struct{}{}
		resp, err := c.Request("name", "nonkind", &msg)
		assert.Equal(remote.ErrUnAvailable, err)
		assert.Nil(resp)
	})

	// FIXME: testcase
	// t.Run("timeout", func(t *testing.T) {
	// 	msg := struct{}{}
	// 	callopts := NewGrainCallOptions(c).WithRetryCount(2).WithRequestTimeout(1 * time.Second)
	// 	resp, err := c.Call("name", "kind", &msg, callopts)
	// 	assert.Equalf(Remote.ErrUnknownError, err, "%v", err)
	// 	assert.Nil(resp)
	// })

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
	// t.Fatalf("need more testcases for cluster.Call")
}

func TestCluster_Get(t *testing.T) {
	t.Skipf("Maintaining")
	cp := newInmemoryProvider()
	kind := NewKind("kind", actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started:
			_ = msg
		}
	}))
	c := newClusterForTest("mycluster", cp, WithKinds(kind))
	c.StartMember()
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
