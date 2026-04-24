//go:build integration

package consul

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/disthash"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var consulAddr string

func TestMain(m *testing.M) {
	addr := os.Getenv("CONSUL_ADDR")
	if addr != "" {
		consulAddr = addr
		os.Exit(m.Run())
	}

	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "hashicorp/consul:1.20",
			ExposedPorts: []string{"8500/tcp"},
			Cmd:          []string{"agent", "-dev", "-client", "0.0.0.0"},
			WaitingFor:   wait.ForHTTP("/v1/status/leader").WithPort("8500/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start consul container: %v\n", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		fmt.Fprintf(os.Stderr, "failed to get container host: %v\n", err)
		os.Exit(1)
	}

	port, err := container.MappedPort(ctx, "8500")
	if err != nil {
		_ = container.Terminate(ctx)
		fmt.Fprintf(os.Stderr, "failed to get container port: %v\n", err)
		os.Exit(1)
	}

	consulAddr = fmt.Sprintf("%s:%s", host, port.Port())

	code := m.Run()

	_ = container.Terminate(ctx)
	os.Exit(code)
}

func newClusterForTest(name string, addr string, cp cluster.ClusterProvider) *cluster.Cluster {
	host, _port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	port, _ := strconv.Atoi(_port)
	remoteConfig := remote.Configure(host, port)
	lookup := disthash.New()
	config := cluster.Configure(name, cp, lookup, remoteConfig)

	system := actor.NewActorSystem()
	c := cluster.NewCluster(system, config)

	// use for test without start remote
	c.ActorSystem.ProcessRegistry.Address = addr
	c.MemberList = cluster.NewMemberList(c)
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)
	return c
}

func TestStartMember(t *testing.T) {
	a := assert.New(t)

	p, _ := NewWithConfig(&api.Config{Address: consulAddr})
	defer func() { _ = p.Shutdown(true) }()

	c := newClusterForTest("mycluster", "127.0.0.1:8000", p)
	eventstream := c.ActorSystem.EventStream
	ch := make(chan any, 16)
	eventstream.Subscribe(func(m any) {
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
				Id:    c.ActorSystem.ID,
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

	p, _ := NewWithConfig(&api.Config{Address: consulAddr})
	defer func() { _ = p.Shutdown(true) }()
	for _, member := range members {
		addr := fmt.Sprintf("%s:%d", member.host, member.port)
		_p, _ := NewWithConfig(&api.Config{Address: consulAddr})
		c := newClusterForTest(member.cluster, addr, _p)
		err := p.StartMember(c)
		a.NoError(err)
		t.Cleanup(func() {
			_ = _p.Shutdown(true)
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
	a := assert.New(t)

	p, _ := NewWithConfig(&api.Config{Address: consulAddr})
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
