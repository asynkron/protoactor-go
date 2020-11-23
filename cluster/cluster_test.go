package cluster

import (
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
)

func TestCluster_Call(t *testing.T) {
	assert := assert.New(t)

	system := actor.NewActorSystem()

	c := New(system, Configure("mycluster", nil, remote.Configure("nonhost", 0)))
	c.partitionValue = setupPartition(c, []string{"kind"})
	c.pidCache = setupPidCache(c.ActorSystem)
	c.MemberList = setupMemberList(c)
	c.Config.TimeoutTime = 1 * time.Second

	members := []*MemberStatus{
		{
			MemberID: "1",
			Host:     "nonhost",
			Port:     -1,
			Kinds:    []string{"kind"},
			Alive:    true,
		},
	}
	system.EventStream.Publish(TopologyEvent(members))
	// address := memberList.getPartitionMember("name", "kind")
	t.Run("invalid kind", func(t *testing.T) {
		msg := struct{}{}
		resp, err := c.Call("name", "nonkind", &msg)
		assert.Equal(remote.ErrUnAvailable, err)
		assert.Nil(resp)
	})

	t.Run("timeout", func(t *testing.T) {
		msg := struct{}{}
		callopts := NewGrainCallOptions(c).WithRetry(2).WithTimeout(1 * time.Second)
		resp, err := c.Call("name", "kind", &msg, callopts)
		assert.Equal(remote.ErrTimeout, err)
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
	pid := system.Root.Spawn(testProps)
	assert.NotNil(pid)
	c.pidCache.addCache("name", pid)
	t.Run("normal", func(t *testing.T) {
		msg := struct{ Code int }{9527}
		resp, err := c.Call("name", "kind", &msg)
		assert.NoError(err)
		assert.Equal(&struct{ Code int }{9528}, resp)
	})
	// t.Fatalf("need more testcases for cluster.Call")
}
