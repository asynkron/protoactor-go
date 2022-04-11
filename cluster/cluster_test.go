package cluster

import (
	"fmt"
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

func TestCluster_Call(t *testing.T) {
	a := assert.New(t)

	system := actor.NewActorSystem()

	c := New(system, Configure("mycluster", nil, nil, remote.Configure("nonhost", 0)))

	c.MemberList = NewMemberList(c)
	c.Config.RequestTimeoutTime = 1 * time.Second

	members := Members{
		{
			Id:    "1",
			Host:  "nonhost",
			Port:  -1,
			Kinds: []string{"kind"},
		},
	}
	c.MemberList.UpdateClusterTopology(members)
	// address := memberList.GetPartitionMember("name", "kind")
	t.Run("invalid kind", func(t *testing.T) {
		msg := struct{}{}
		resp, err := c.Request("name", "nonkind", &msg)
		a.Equal(remote.ErrUnAvailable, err)
		a.Nil(resp)
	})

	// FIXME: testcase
	// t.Run("timeout", func(t *testing.T) {
	// 	msg := struct{}{}
	// 	callopts := NewGrainCallOptions(c).WithRetry(2).WithRequestTimeout(1 * time.Second)
	// 	resp, err := c.Call("name", "kind", &msg, callopts)
	// 	a.Equalf(Remote.ErrUnknownError, err, "%v", err)
	// 	a.Nil(resp)
	// })

	testProps := actor.PropsFromFunc(
		func(context actor.Context) {
			switch msg := context.Message().(type) {
			case *struct{ Code int }:
				msg.Code++
				context.Respond(msg)
			}
		})
	pid := system.Root.Spawn(testProps)
	a.NotNil(pid)
	c.PidCache.Set("name", "kind", pid)
	t.Run("normal", func(t *testing.T) {
		msg := struct{ Code int }{9527}
		resp, err := c.Request("name", "kind", &msg)
		a.NoError(err)
		a.Equal(&struct{ Code int }{9528}, resp)
	})
	// t.Fatalf("need more testcases for cluster.Call")
}

func TestCluster_Get(t *testing.T) {
	cp := newInmemoryProvider()
	system := actor.NewActorSystem()
	kind := NewKind("kind", actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started:
			_ = msg
		}
	}))
	c := New(system, Configure("mycluster", cp, nil, remote.Configure("127.0.0.1", 0), WithKinds(kind)))
	c.StartMember()
	cp.publishClusterTopologyEvent()
	t.Run("invalid kind", func(t *testing.T) {
		a := assert.New(t)
		a.Equal(1, c.MemberList.Length())
		pid := c.Get("name", "nonkind")
		a.Nil(pid)
	})

	t.Run("ok", func(t *testing.T) {
		a := assert.New(t)
		pid := c.Get("name", "kind")
		a.NotNil(pid)
	})
}
