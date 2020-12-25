package cluster

import (
	"fmt"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
)

// inmemoryProvider use for test
type inmemoryProvider struct {
	cluster  *Cluster
	members  map[string]*Member
	self     *Member
	revision uint64
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
	members := []*Member{}
	for _, m := range p.members {
		members = append(members, m)
	}

	res := TopologyEvent(members)

	p.revision++
	p.cluster.MemberList.UpdateClusterTopology(res, p.revision)
	// p.cluster.ActorSystem.EventStream.Publish(res)
}

func (p *inmemoryProvider) StartMember(c *Cluster) error {
	p.init(c)
	p.members[p.self.Id] = p.self
	p.publishClusterTopologyEvent()
	return nil
}
func (p *inmemoryProvider) StartClient(c *Cluster) error {
	p.init(c)
	p.publishClusterTopologyEvent()
	return nil
}
func (p *inmemoryProvider) Shutdown(graceful bool) error {
	delete(p.members, p.self.Id)
	return nil
}
func (p *inmemoryProvider) UpdateClusterState(state ClusterState) error {
	return fmt.Errorf("Not implemented yet")
}

func TestCluster_Call(t *testing.T) {
	assert := assert.New(t)

	system := actor.NewActorSystem()

	c := New(system, Configure("mycluster", nil, remote.Configure("nonhost", 0)))
	c.partitionValue = setupPartition(c, []string{"kind"})
	c.pidCache = setupPidCache(c.ActorSystem)
	c.MemberList = setupMemberList(c)
	c.Config.TimeoutTime = 1 * time.Second
	c.partitionManager = newPartitionManager(c)
	c.partitionManager.Start()

	members := []*Member{
		{
			Id:    "1",
			Host:  "nonhost",
			Port:  -1,
			Kinds: []string{"kind"},
		},
	}
	c.MemberList.UpdateClusterTopology(members, 1)
	// address := memberList.getPartitionMember("name", "kind")
	t.Run("invalid kind", func(t *testing.T) {
		msg := struct{}{}
		resp, err := c.Call("name", "nonkind", &msg)
		assert.Equal(remote.ErrUnAvailable, err)
		assert.Nil(resp)
	})

	// FIXME: testcase
	// t.Run("timeout", func(t *testing.T) {
	// 	msg := struct{}{}
	// 	callopts := NewGrainCallOptions(c).WithRetry(2).WithTimeout(1 * time.Second)
	// 	resp, err := c.Call("name", "kind", &msg, callopts)
	// 	assert.Equalf(remote.ErrUnknownError, err, "%v", err)
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
	pid := system.Root.Spawn(testProps)
	assert.NotNil(pid)
	c.pidCache.addCache("kind/name", pid)
	t.Run("normal", func(t *testing.T) {
		msg := struct{ Code int }{9527}
		resp, err := c.Call("name", "kind", &msg)
		assert.NoError(err)
		assert.Equal(&struct{ Code int }{9528}, resp)
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
	c := New(system, Configure("mycluster", cp, remote.Configure("127.0.0.1", 0), kind))
	c.Start()
	cp.publishClusterTopologyEvent()
	t.Run("invalid kind", func(t *testing.T) {
		assert := assert.New(t)
		assert.Equal(1, c.MemberList.Length())
		pid, code := c.Get("name", "nonkind")
		assert.Equal(remote.ResponseStatusCodeUNAVAILABLE, code)
		assert.Nil(pid)
	})

	t.Run("ok", func(t *testing.T) {
		assert := assert.New(t)
		pid, code := c.Get("name", "kind")
		assert.Equal(remote.ResponseStatusCodeOK, code)
		assert.NotNil(pid)
	})
}
