package zk

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
)

func NewZkProvider() (*Provider, error) {
	endpoints := strings.Split(os.Getenv("ZK_ENDPOINTS"), " ")
	return New(endpoints)
}

func newClusterForTest(name string, addr string, cp cluster.ClusterProvider) *cluster.Cluster {
	host, _port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	port, _ := strconv.Atoi(_port)
	remoteConfig := remote.Configure(host, port)
	config := cluster.Configure(name, cp, remoteConfig)

	system := actor.NewActorSystem()
	c := cluster.New(system, config)
	// use for test without start remote
	c.ActorSystem.ProcessRegistry.Address = addr
	c.MemberList = cluster.NewMemberList(c)
	return c
}

func TestStartMember(t *testing.T) {
	if testing.Short() {
		return
	}
	assert := assert.New(t)

	p, _ := NewZkProvider()
	defer p.Shutdown(true)

	c := newClusterForTest("test_zk_provider", "127.0.0.1:8000", p)
	eventstream := c.ActorSystem.EventStream
	ch := make(chan interface{}, 16)
	eventstream.Subscribe(func(m interface{}) {
		if _, ok := m.(*cluster.ClusterTopologyEventV2); ok {
			ch <- m
		}
	})

	err := p.StartMember(c)
	assert.NoError(err)

	select {
	case <-time.After(5 * time.Second):
		assert.FailNow("no member joined yet")

	case m := <-ch:
		// member joined
		msg := m.(*cluster.ClusterTopologyEventV2)
		members := []*cluster.Member{
			{
				Id:    "test_zk_provider@127.0.0.1:8000",
				Host:  "127.0.0.1",
				Port:  8000,
				Kinds: []string{},
			},
		}

		expected := &cluster.ClusterTopology{
			Members: members,
			Joined:  members,
			EventId: msg.ClusterTopology.EventId,
		}
		assert.Equal(expected, msg.ClusterTopology)

	}
}

func TestStartMember_Multiple(t *testing.T) {
	if testing.Short() {
		return
	}
	assert := assert.New(t)
	members := []struct {
		cluster string
		host    string
		port    int
	}{
		{"mycluster2", "127.0.0.1", 8001},
		{"mycluster2", "127.0.0.1", 8002},
		{"mycluster2", "127.0.0.1", 8003},
	}

	var p = make([]*Provider, len(members))
	var err error
	t.Cleanup(func() {
		for i := range p {
			p[i].Shutdown(true)
		}
	})
	for i, member := range members {
		addr := fmt.Sprintf("%s:%d", member.host, member.port)
		p[i], err = NewZkProvider()
		assert.NoError(err)
		c := newClusterForTest(member.cluster, addr, p[i])
		err := p[i].StartMember(c)
		assert.NoError(err)
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
		nodes, _, err := p[i].fetchNodes()
		assert.NoError(err)
		assert.Equal(len(members), len(nodes))
		flag := isNodesEqual(nodes)
		assert.Truef(flag, "Member not found - %+v", p[i].self)
	}
}

func TestUpdateMemberState(t *testing.T) {
	if testing.Short() {
		return
	}
	assert := assert.New(t)

	p, _ := NewZkProvider()
	defer p.Shutdown(true)

	c := newClusterForTest("mycluster3", "127.0.0.1:8000", p)
	err := p.StartMember(c)
	assert.NoError(err)

	state := cluster.ClusterState{[]string{"yes"}}
	err = p.UpdateClusterState(state)
	assert.NoError(err)
}

func TestUpdateMemberState_DoesNotReregisterAfterShutdown(t *testing.T) {
	if testing.Short() {
		return
	}
	assert := assert.New(t)

	p, _ := NewZkProvider()
	c := newClusterForTest("mycluster4", "127.0.0.1:8001", p)
	err := p.StartMember(c)
	assert.NoError(err)
	t.Cleanup(func() {
		p.Shutdown(true)
	})

	state := cluster.ClusterState{[]string{"yes"}}
	err = p.UpdateClusterState(state)
	assert.NoError(err)

	err = p.Shutdown(true)
	assert.NoError(err)

	err = p.UpdateClusterState(state)
	assert.Error(err)
}
