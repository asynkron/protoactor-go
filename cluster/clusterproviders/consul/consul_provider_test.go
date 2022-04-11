package consul

import (
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"

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
	lookup := disthash.New()
	config := cluster.Configure(name, cp, lookup, remoteConfig)
	// return cluster.NewForTest(system, config)

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

	p, _ := New()
	defer p.Shutdown(true)

	c := newClusterForTest("mycluster", "127.0.0.1:8000", p)
	eventstream := c.ActorSystem.EventStream
	ch := make(chan interface{}, 16)
	eventstream.Subscribe(func(m interface{}) {
		if _, ok := m.(*cluster.ClusterTopology); ok {
			ch <- m
		}
	})

	err := p.StartMember(c)
	a.NoError(err)

	select {
	case <-time.After(10 * time.Second):
		a.FailNow("no member joined yet")

	case m := <-ch:
		msg := m.(*cluster.ClusterTopology)
		// member joined
		members := []*cluster.Member{
			{
				// Id:    "mycluster@127.0.0.1:8000",
				Id:    fmt.Sprintf("%s", c.ActorSystem.ID),
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

func TestRegisterMultipleMembers(t *testing.T) {
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

	p, _ := New()
	defer p.Shutdown(true)
	for _, member := range members {
		addr := fmt.Sprintf("%s:%d", member.host, member.port)
		_p, _ := New()
		c := newClusterForTest(member.cluster, addr, _p)
		err := p.StartMember(c)
		a.NoError(err)
		t.Cleanup(func() {
			_p.Shutdown(true)
		})
	}

	entries, _, err := p.client.Health().Service("mycluster2", "", true, nil)
	a.NoError(err)

	found := false
	for _, entry := range entries {
		found = false
		for _, member := range members {
			if entry.Service.Port == member.port {
				found = true
			}
		}
		a.Truef(found, "Member port not found - ExtensionID:%v Address: %v:%v",
			entry.Service.ID, entry.Service.Address, entry.Service.Port)
	}
}

func TestUpdateTTL_DoesNotReregisterAfterShutdown(t *testing.T) {
	if testing.Short() {
		return
	}
	a := assert.New(t)

	p, _ := New()
	c := newClusterForTest("mycluster5", "127.0.0.1:8001", p)

	shutdownShouldHaveResolved := make(chan bool, 1)

	err := p.StartMember(c)
	a.NoError(err)

	time.Sleep(time.Second)
	found, _ := findService(t, p)
	a.True(found, "service was not registered in consul")

	go func() {
		// if after 5 seconds `Shutdown` did not resolve, assume that it will not resolve until `blockingUpdateTTL` resolves
		time.Sleep(5 * time.Second)
		shutdownShouldHaveResolved <- true
	}()

	err = p.Shutdown(true)
	a.NoError(err)
	shutdownShouldHaveResolved <- true

	// since `UpdateTTL` runs in a separate goroutine we need to wait until it is actually finished before checking the member's clusterstatus
	p.updateTTLWaitGroup.Wait()
	found, status := findService(t, p)
	a.Falsef(found, "service was still registered in consul after shutdown (service status: %s)", status)
}

func findService(t *testing.T, p *Provider) (found bool, status string) {
	service := p.cluster.Config.Name
	port := p.cluster.Config.RemoteConfig.Port
	entries, _, err := p.client.Health().Service(service, "", false, nil)
	if err != nil {
		t.Error(err)
	}

	for _, entry := range entries {
		if entry.Service.Port == port {
			return true, entry.Checks.AggregatedStatus()
		}
	}
	return false, ""
}
