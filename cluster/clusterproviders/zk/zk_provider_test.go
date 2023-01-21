package zk

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/suite"
)

type ZookeeperTestSuite struct {
	suite.Suite
}

func (suite *ZookeeperTestSuite) SetupTest() {
	plog.SetLevel(log.ErrorLevel)
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

func (self *ClusterAndSystem) Shutdown() {
	self.Cluster.Shutdown(true)
}

func (suite *ZookeeperTestSuite) start(name string, opts ...cluster.ConfigOption) *ClusterAndSystem {
	cp, _ := New([]string{`localhost:8000`})
	remoteConfig := remote.Configure("localhost", 0)
	config := cluster.Configure(name, cp, disthash.New(), remoteConfig, opts...)
	system := actor.NewActorSystem()
	c := cluster.New(system, config)
	c.StartMember()
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
