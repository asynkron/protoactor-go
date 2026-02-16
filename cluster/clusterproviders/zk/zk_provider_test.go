//go:build integration

package zk

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var zkEndpoint string

func TestMain(m *testing.M) {
	endpoint := os.Getenv("ZK_ENDPOINT")
	if endpoint != "" {
		zkEndpoint = endpoint
		os.Exit(m.Run())
	}

	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "zookeeper:3.9",
			ExposedPorts: []string{"2181/tcp"},
			WaitingFor:   wait.ForLog("PrepRequestProcessor (sid:0) started").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start zookeeper container: %v\n", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		fmt.Fprintf(os.Stderr, "failed to get container host: %v\n", err)
		os.Exit(1)
	}

	port, err := container.MappedPort(ctx, "2181")
	if err != nil {
		_ = container.Terminate(ctx)
		fmt.Fprintf(os.Stderr, "failed to get container port: %v\n", err)
		os.Exit(1)
	}

	zkEndpoint = fmt.Sprintf("%s:%s", host, port.Port())

	code := m.Run()

	_ = container.Terminate(ctx)
	os.Exit(code)
}

type ZookeeperTestSuite struct {
	suite.Suite
}

func (suite *ZookeeperTestSuite) SetupTest() {
}

func (suite *ZookeeperTestSuite) TearDownTest() {
}

func TestZookeeperTestSuite(t *testing.T) {
	suite.Run(t, new(ZookeeperTestSuite))
}

type ClusterAndSystem struct {
	Cluster *cluster.Cluster
	System  *actor.ActorSystem
}

func (cs *ClusterAndSystem) Shutdown() {
	cs.Cluster.Shutdown(true)
}

func (suite *ZookeeperTestSuite) start(name string, opts ...cluster.ConfigOption) *ClusterAndSystem {
	cp, _ := New([]string{zkEndpoint})
	remoteConfig := remote.Configure("localhost", 0)
	config := cluster.Configure(name, cp, disthash.New(), remoteConfig, opts...)
	system := actor.NewActorSystem()
	c := cluster.NewCluster(system, config)
	if err := c.StartMember(); err != nil {
		suite.T().Fatalf("failed to start member: %v", err)
	}
	return &ClusterAndSystem{Cluster: c, System: system}
}

func (suite *ZookeeperTestSuite) TestEmptyExecute() {
	name := `cluster0`
	suite.start(name).Shutdown()
}

func (suite *ZookeeperTestSuite) TestMultiNodes() {
	var actorCount int32
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			atomic.AddInt32(&actorCount, 1)
		}
	})
	helloKind := cluster.NewKind("hello", props)

	name := `cluster1`
	c1 := suite.start(name, cluster.WithKinds(helloKind))
	defer c1.Shutdown()
	c2 := suite.start(name, cluster.WithKinds(helloKind))
	defer c2.Shutdown()
	c1.Cluster.Get(`a1`, `hello`)
	c2.Cluster.Get(`a2`, `hello`)
	for actorCount != 2 {
		time.Sleep(time.Microsecond * 5)
	}
	suite.Assert().Equal(2, c1.Cluster.MemberList.Members().Len(), "Expected 2 members in the cluster")
	suite.Assert().Equal(2, c2.Cluster.MemberList.Members().Len(), "Expected 2 members in the cluster")
}
