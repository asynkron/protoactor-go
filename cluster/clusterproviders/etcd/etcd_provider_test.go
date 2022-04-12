package etcd

import (
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
)

func newClusterForTest(name string, addr string, cp cluster.ClusterProvider) *cluster.Cluster {
	host, _port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	port, _ := strconv.Atoi(_port)
	remoteConfig := remote.Configure(host, port)
	config := cluster.Configure(name, cp, nil, remoteConfig)

	system := actor.NewActorSystem()
	c := cluster.New(system, config)
	// use for test without start remote
	c.ActorSystem.ProcessRegistry.Address = addr
	c.MemberList = cluster.NewMemberList(c)
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)

	return c
}

func TestStartMember(t *testing.T) {
	if testing.Short() {
		return
	}

	a := assert.New(t)

	p, err := New()
	a.NoError(err)
	defer p.Shutdown(true)

	c := newClusterForTest("test_etcd_provider", "127.0.0.1:8000", p)
	eventstream := c.ActorSystem.EventStream
	ch := make(chan interface{}, 16)

	eventstream.Subscribe(func(m interface{}) {
		if _, ok := m.(*cluster.ClusterTopology); ok {
			ch <- m
		}
	})

	err = p.StartMember(c)
	a.NoError(err)

	select {
	case <-time.After(5 * time.Second):
		a.FailNow("no member joined yet")

	case m := <-ch:
		// member joined
		msg, _ := m.(*cluster.ClusterTopology)

		members := []*cluster.Member{
			{
				// Id:    "test_etcd_provider@127.0.0.1:8000",
				Id:    fmt.Sprintf("test_etcd_provider@%s", c.ActorSystem.ID),
				Host:  "127.0.0.1",
				Port:  8000,
				Kinds: []string{},
			},
		}

		expected := &cluster.ClusterTopology{
			Members:      members,
			Joined:       members,
			Left:         []*cluster.Member{},
			TopologyHash: msg.TopologyHash,
		}
		a.Equal(expected, msg)

	}
}

func TestStartMember_Multiple(t *testing.T) {
	if testing.Short() {
		return
	}

	a := assert.New(t)
	members := []struct {
		cluster string
		host    string
		port    int
	}{
		{"mycluster2", "127.0.0.1", 8001},
		{"mycluster2", "127.0.0.1", 8002},
		{"mycluster2", "127.0.0.1", 8003},
	}

	p := make([]*Provider, len(members))

	var err error

	t.Cleanup(func() {
		for i := range p {
			_ = p[i].Shutdown(true)
		}
	})

	for i, member := range members {
		addr := fmt.Sprintf("%s:%d", member.host, member.port)
		p[i], err = New()
		a.NoError(err)

		c := newClusterForTest(member.cluster, addr, p[i])
		err := p[i].StartMember(c)
		a.NoError(err)
	}

	isNodesEqual := func(nodes []*Node) bool {
		for _, node := range nodes {
			for _, member := range members {
				if node.Host == member.host && node.Port == member.port {
					return true
				}
			}
		}

		return false
	}

	for i := range p {
		nodes, err := p[i].fetchNodes()
		a.NoError(err)
		a.Equal(len(members), len(nodes))
		flag := isNodesEqual(nodes)
		a.Truef(flag, "Member not found - %+v", p[i].self)
	}
}

//func TestUpdateMemberState(t *testing.T) {
//	if testing.Short() {
//		return
//	}
//	assert := assert.New(t)
//
//	p, _ := New()
//	defer p.Shutdown(true)
//
//	c := newClusterForTest("mycluster3", "127.0.0.1:8000", p)
//	err := p.StartMember(c)
//	assert.NoError(err)
//
//	state := cluster.ClusterState{[]string{"yes"}}
//	err = p.UpdateClusterState(state)
//	assert.NoError(err)
//}
//
//func TestUpdateMemberState_DoesNotReregisterAfterShutdown(t *testing.T) {
//	if testing.Short() {
//		return
//	}
//	assert := assert.New(t)
//
//	p, _ := New()
//	c := newClusterForTest("mycluster4", "127.0.0.1:8001", p)
//	err := p.StartMember(c)
//	assert.NoError(err)
//	t.Cleanup(func() {
//		p.Shutdown(true)
//	})
//
//	state := cluster.ClusterState{[]string{"yes"}}
//	err = p.UpdateClusterState(state)
//	assert.NoError(err)
//
//	err = p.Shutdown(true)
//	assert.NoError(err)
//
//	err = p.UpdateClusterState(state)
//	assert.Error(err)
//}
