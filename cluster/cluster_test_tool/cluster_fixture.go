package cluster_test_tool

import (
	"context"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/test"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type ClusterFixture interface {
	GetMembers() []*cluster.Cluster
	GetClusterSize() int
	SpawnNode() *cluster.Cluster
	RemoveNode(node *cluster.Cluster, graceful bool)
	ShutDown()
}

type ClusterFixtureConfig struct {
	GetClusterKinds    func() []*cluster.Kind
	GetClusterProvider func() cluster.ClusterProvider
	Configure          func(*cluster.Config) *cluster.Config
	GetIdentityLookup  func(clusterName string) cluster.IdentityLookup
	OnDeposing         func()
}

type ClusterFixtureOption func(*ClusterFixtureConfig)

// WithGetClusterKinds sets the cluster kinds for the cluster fixture
func WithGetClusterKinds(getKinds func() []*cluster.Kind) ClusterFixtureOption {
	return func(c *ClusterFixtureConfig) {
		c.GetClusterKinds = getKinds
	}
}

// WithClusterConfigure sets the cluster configure function for the cluster fixture
func WithClusterConfigure(configure func(*cluster.Config) *cluster.Config) ClusterFixtureOption {
	return func(c *ClusterFixtureConfig) {
		c.Configure = configure
	}
}

// WithGetClusterProvider sets the cluster provider for the cluster fixture
func WithGetClusterProvider(getProvider func() cluster.ClusterProvider) ClusterFixtureOption {
	return func(c *ClusterFixtureConfig) {
		c.GetClusterProvider = getProvider
	}
}

// WithGetIdentityLookup sets the identity lookup function for the cluster fixture
func WithGetIdentityLookup(identityLookup func(clusterName string) cluster.IdentityLookup) ClusterFixtureOption {
	return func(c *ClusterFixtureConfig) {
		c.GetIdentityLookup = identityLookup
	}
}

// WithOnDeposing sets the on deposing function for the cluster fixture
func WithOnDeposing(onDeposing func()) ClusterFixtureOption {
	return func(c *ClusterFixtureConfig) {
		c.OnDeposing = onDeposing
	}
}

const InvalidIdentity string = "invalid"

type BaseClusterFixture struct {
	clusterName string
	clusterSize int
	config      *ClusterFixtureConfig
	members     []*cluster.Cluster
}

func NewBaseClusterFixture(clusterSize int, opts ...ClusterFixtureOption) *BaseClusterFixture {
	config := &ClusterFixtureConfig{
		GetClusterKinds:    func() []*cluster.Kind { return make([]*cluster.Kind, 0) },
		GetClusterProvider: func() cluster.ClusterProvider { return test.NewTestProvider(test.NewInMemAgent()) },
		Configure:          func(c *cluster.Config) *cluster.Config { return c },
		GetIdentityLookup:  func(clusterName string) cluster.IdentityLookup { return disthash.New() },
		OnDeposing:         func() {},
	}
	for _, opt := range opts {
		opt(config)
	}

	fixTure := &BaseClusterFixture{
		clusterSize: clusterSize,
		clusterName: "test-cluster-" + uuid.NewString()[0:6],
		config:      config,
		members:     make([]*cluster.Cluster, 0),
	}
	return fixTure
}

// Initialize initializes the cluster fixture
func (b *BaseClusterFixture) Initialize() {
	nodes := b.spawnClusterNodes()
	b.members = append(b.members, nodes...)
}

func (b *BaseClusterFixture) GetMembers() []*cluster.Cluster {
	return b.members
}

func (b *BaseClusterFixture) GetClusterSize() int {
	return b.clusterSize
}

func (b *BaseClusterFixture) SpawnNode() *cluster.Cluster {
	node := b.spawnClusterMember()
	b.members = append(b.members, node)
	return node
}

func (b *BaseClusterFixture) RemoveNode(node *cluster.Cluster, graceful bool) {
	has := false
	for i, member := range b.members {
		if member == node {
			has = true
			b.members = append(b.members[:i], b.members[i+1:]...)
			member.Shutdown(graceful)
			break
		}
	}
	if !has {
		plog.Error("node not found", log.Object("node", node))
	}
}

func (b *BaseClusterFixture) ShutDown() {
	b.config.OnDeposing()
	b.waitForMembersToShutdown()
	b.members = b.members[:0]
}

// spawnClusterNodes spawns a number of cluster nodes
func (b *BaseClusterFixture) spawnClusterNodes() []*cluster.Cluster {
	nodes := make([]*cluster.Cluster, 0, b.clusterSize)
	for i := 0; i < b.clusterSize; i++ {
		nodes = append(nodes, b.spawnClusterMember())
	}

	bgCtx := context.Background()
	timeoutCtx, cancel := context.WithTimeout(bgCtx, time.Second*10)
	defer cancel()
	group := new(errgroup.Group)
	for _, node := range nodes {
		tmpNode := node
		group.Go(func() error {
			done := make(chan struct{})
			go func() {
				tmpNode.MemberList.TopologyConsensus(timeoutCtx)
				close(done)
			}()

			select {
			case <-timeoutCtx.Done():
				return timeoutCtx.Err()
			case <-done:
				return nil
			}
		})
	}
	err := group.Wait()
	if err != nil {
		panic("Failed to reach consensus")
	}

	return nodes
}

// spawnClusterMember spawns a cluster members
func (b *BaseClusterFixture) spawnClusterMember() *cluster.Cluster {
	config := cluster.Configure(b.clusterName, b.config.GetClusterProvider(), b.config.GetIdentityLookup(b.clusterName),
		remote.Configure("localhost", 0),
		cluster.WithKinds(b.config.GetClusterKinds()...),
	)
	config = b.config.Configure(config)

	system := actor.NewActorSystem()

	c := cluster.New(system, config)
	c.StartMember()
	return c
}

// waitForMembersToShutdown waits for the members to shutdown
func (b *BaseClusterFixture) waitForMembersToShutdown() {
	for _, member := range b.members {
		plog.Info("Preparing shutdown for cluster member", log.String("member", member.ActorSystem.ID))
	}

	group := new(errgroup.Group)
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second*1000)
	defer cancel()

	for _, member := range b.members {
		member := member
		group.Go(func() error {
			done := make(chan struct{})
			go func() {
				plog.Info("Shutting down cluster member", log.String("member", member.ActorSystem.ID))
				member.Shutdown(true)
				close(done)
			}()

			select {
			case <-timeoutCtx.Done():
				return timeoutCtx.Err()
			case <-done:
				return nil
			}
		})
	}
	err := group.Wait()
	if err != nil {
		panic(err)
	}
}
