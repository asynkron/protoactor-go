package consul

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
)

func newClusterForTest(name string, addr string, cp cluster.ClusterProvider) *cluster.Cluster {
	host, _port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	port, _ := strconv.Atoi(_port)
	remoteConfig := remote.Configure(host, port)
	config := cluster.Configure(name, cp, remoteConfig)
	// return cluster.NewForTest(system, config)

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

	p, _ := New()
	defer p.Shutdown(true)

	c := newClusterForTest("mycluster", "127.0.0.1:8000", p)
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
	case <-time.After(10 * time.Second):
		assert.FailNow("no member joined yet")

	case m := <-ch:
		msg := m.(*cluster.ClusterTopologyEventV2)
		// member joined
		members := []*cluster.Member{
			{
				Id:    "mycluster@127.0.0.1:8000",
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

func TestRegisterMultipleMembers(t *testing.T) {
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

	p, _ := New()
	defer p.Shutdown(true)
	for _, member := range members {
		addr := fmt.Sprintf("%s:%d", member.host, member.port)
		_p, _ := New()
		c := newClusterForTest(member.cluster, addr, _p)
		err := p.StartMember(c)
		assert.NoError(err)
		t.Cleanup(func() {
			_p.Shutdown(true)
		})
	}

	entries, _, err := p.client.Health().Service("mycluster2", "", true, nil)
	assert.NoError(err)

	found := false
	for _, entry := range entries {
		found = false
		for _, member := range members {
			if entry.Service.Port == member.port {
				found = true
			}
		}
		assert.Truef(found, "Member port not found - ID:%v Address: %v:%v",
			entry.Service.ID, entry.Service.Address, entry.Service.Port)
	}
}

func TestUpdateMemberState(t *testing.T) {
	if testing.Short() {
		return
	}
	assert := assert.New(t)

	p, _ := New()
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

	p, _ := New()
	c := newClusterForTest("mycluster4", "127.0.0.1:8001", p)
	err := p.StartMember(c)
	assert.NoError(err)
	t.Cleanup(func() {
		p.Shutdown(true)
	})

	found, _ := findService(t, p)
	assert.True(found, "service was not registered in consul")

	state := cluster.ClusterState{[]string{"yes"}}
	err = p.UpdateClusterState(state)
	assert.NoError(err)

	err = p.Shutdown(true)
	assert.NoError(err)

	err = p.UpdateClusterState(state)
	assert.Equal(ProviderShuttingDownError, err)

	found, status := findService(t, p)
	assert.Falsef(found, "service was re-registered in consul after shutdown (status: %s)", status)
}

func TestUpdateTTL_DoesNotReregisterAfterShutdown(t *testing.T) {
	if testing.Short() {
		return
	}
	assert := assert.New(t)

	p, _ := New()
	c := newClusterForTest("mycluster5", "127.0.0.1:8001", p)
	port := c.Config.RemoteConfig.Port

	originalBlockingUpdateTTLFunc := blockingUpdateTTLFunc
	defer func() {
		blockingUpdateTTLFunc = originalBlockingUpdateTTLFunc
	}()

	registeredInConsul := false

	var blockingUpdateTTLBlockReachedWg sync.WaitGroup
	blockingUpdateTTLBlockReachedWg.Add(1)

	// shutdownShouldHaveResolved := false

	shutdownShouldHaveResolved := make(chan bool, 1)

	// this simulates `blockingUpdateTTL` in `UpdateTTL` to be slower than `Shutdown`
	blockingUpdateTTLFunc = func(p *Provider) error {
		// default behaviour until `StartMember` was called
		if !registeredInConsul || p.port != port {
			return originalBlockingUpdateTTLFunc(p)
		}

		blockingUpdateTTLBlockReachedWg.Done()

		// wait until it is safe to assume that `Shutdown` will not finish until this call resolves or that `Shutdown` is already done
		<-shutdownShouldHaveResolved
		return originalBlockingUpdateTTLFunc(p)
	}

	err := p.StartMember(c)
	assert.NoError(err)
	registeredInConsul = true

	found, _ := findService(t, p)
	assert.True(found, "service was not registered in consul")

	// Wait until `blockingUpdateTTL` waits for the deregistration/shutdown of the member
	blockingUpdateTTLBlockReachedWg.Wait()

	go func() {
		// if after 5 seconds `Shutdown` did not resolve, assume that it will not resolve until `blockingUpdateTTL` resolves
		time.Sleep(5 * time.Second)
		shutdownShouldHaveResolved <- true
	}()

	err = p.Shutdown(true)
	assert.NoError(err)
	shutdownShouldHaveResolved <- true

	// since `UpdateTTL` runs in a separate goroutine we need to wait until it is actually finished before checking the member's clusterstatus
	p.updateTTLWaitGroup.Wait()
	found, status := findService(t, p)
	assert.Falsef(found, "service was still registered in consul after shutdown (service status: %s)", status)
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
