package k8s

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newClusterForTest(name string, addr string, cp cluster.ClusterProvider, id cluster.IdentityLookup) *cluster.Cluster {
	host, _port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	port, _ := strconv.Atoi(_port)
	remoteConfig := remote.Configure(host, port)
	config := cluster.Configure(name, cp, id, remoteConfig)

	system := actor.NewActorSystem()
	c := cluster.New(system, config)

	// use for test without start remote
	c.ActorSystem.ProcessRegistry.Address = addr
	c.MemberList = cluster.NewMemberList(c)
	c.Remote = remote.NewRemote(system, config.RemoteConfig)
	return c
}

func TestStartMember(t *testing.T) {
	if testing.Short() {
		return
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		t.Skipf("Skipped k8s testcases")
	}
	assert := assert.New(t)

	p, _ := New()
	p, newErr := New()
	if newErr != nil {
		panic(fmt.Errorf("could not create new cluster provider: %w", newErr))
	}
	id := disthash.New()
	defer p.Shutdown(true)

	c := newClusterForTest("k8scluster", "127.0.0.1:8000", p, id)
	eventstream := c.ActorSystem.EventStream
	ch := make(chan interface{}, 16)
	eventstream.Subscribe(func(m interface{}) {
		if _, ok := m.(*cluster.ClusterTopology); ok {
			ch <- m
		}
	})

	err := p.StartMember(c)
	assert.NoError(err)

	select {
	case <-time.After(10 * time.Second):
		assert.FailNow("no member joined yet")

	case m := <-ch:
		msg := m.(*cluster.ClusterTopology)
		// member joined
		members := []*cluster.Member{
			{
				Id:    "k8scluster@127.0.0.1:8000",
				Host:  "127.0.0.1",
				Port:  8000,
				Kinds: []string{},
			},
		}

		expected := &cluster.ClusterTopology{
			Members: members,
			Joined:  members,
		}
		assert.Equal(expected, msg)
	}
}

func TestRegisterMultipleMembers(t *testing.T) {
	if testing.Short() {
		return
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		t.Skipf("Skipped k8s testcases")
	}
	assert := assert.New(t)

	members := []struct {
		cluster string
		host    string
		port    int
	}{
		{"k8scluster2", "127.0.0.1", 8001},
		{"k8scluster2", "127.0.0.1", 8002},
		{"k8scluster2", "127.0.0.1", 8003},
	}

	p, _ := New()
	defer p.Shutdown(true)
	for _, member := range members {
		addr := fmt.Sprintf("%s:%d", member.host, member.port)
		_p, _ := New()
		_id := disthash.New()
		c := newClusterForTest(member.cluster, addr, _p, _id)
		err := p.StartMember(c)
		assert.NoError(err)
		t.Cleanup(func() {
			_p.Shutdown(true)
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	pods, err := p.client.CoreV1().Pods(p.retrieveNamespace()).List(ctx, metav1.ListOptions{})
	assert.NoError(err)
	assert.Equal(pods.Size(), len(members))
}

func TestUpdateMemberState(t *testing.T) {
	if testing.Short() {
		return
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		t.Skipf("Skipped k8s testcases")
	}
	assert := assert.New(t)

	p, _ := New()
	id := disthash.New()
	defer p.Shutdown(true)

	c := newClusterForTest("k8scluster3", "127.0.0.1:8000", p, id)
	err := p.StartMember(c)
	assert.NoError(err)
}

func TestUpdateMemberState_DoesNotReregisterAfterShutdown(t *testing.T) {
	if testing.Short() {
		return
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		t.Skipf("Skipped k8s testcases")
	}
	assert := assert.New(t)

	p, _ := New()
	id := disthash.New()
	c := newClusterForTest("k8scluster4", "127.0.0.1:8001", p, id)
	err := p.StartMember(c)
	assert.NoError(err)
	t.Cleanup(func() {
		p.Shutdown(true)
	})

	err = p.Shutdown(true)
	assert.NoError(err)

	assert.Equal(ProviderShuttingDownError, err)
}
