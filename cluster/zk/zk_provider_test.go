package zk

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/remote"
	"github.com/go-zookeeper/zk"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
)

func (suite *ZkProviderTestSuite) TestTestStartMember() {
	name := "zk_test_cluster"
	host := "127.0.0.1"
	port := 8000

	conn := NewMockzkConn(suite.ctrl)
	conn.EXPECT().Exists(filepath.Join("/protoactor", name)).Return(true, &zk.Stat{}, nil)
	conn.EXPECT().CreateProtectedEphemeralSequentialForAnyInput().DoAndReturn(func(path string, data []byte, acl []zk.ACL) (string, error) {
		return fmt.Sprintf("%s%d", path, 1), nil
	})
	conn.EXPECT().Children(filepath.Join("/protoactor", name)).Return([]string{fmt.Sprintf("actor-%d", 1)}, &zk.Stat{Cversion: 1}, nil)
	node := NewNode(fmt.Sprintf("%v@%v:%v", name, host, port), host, port, []string{"kind1"})
	data, _ := node.Serialize()
	conn.EXPECT().Get(fmt.Sprintf("/protoactor/%s/actor-%d", name, 1)).Return(data, &zk.Stat{Cversion: 1}, nil)
	conn.EXPECT().ChildrenW(filepath.Join("/protoactor", name)).Return(
		[]string{fmt.Sprintf("actor-%d", 1)},
		&zk.Stat{NumChildren: 1, Cversion: 1},
		make(chan zk.Event, 1),
		nil,
	).AnyTimes()

	p, c := suite.newClusterForTest("zk_test_cluster", "127.0.0.1:8000", conn)
	eventstream := c.ActorSystem.EventStream
	ch := make(chan interface{}, 16)
	eventstream.Subscribe(func(m interface{}) {
		if _, ok := m.(*cluster.ClusterTopologyEventV2); ok {
			ch <- m
		}
	})

	err := p.StartMember(c)
	suite.NoError(err)

	select {
	case <-time.After(5 * time.Second):
		suite.FailNow("no member joined yet")

	case m := <-ch:
		// member joined
		msg := m.(*cluster.ClusterTopologyEventV2)
		members := []*cluster.Member{
			{
				Id:    "zk_test_cluster@127.0.0.1:8000",
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
		suite.Equal(expected, msg.ClusterTopology)
	}
}

func (suite *ZkProviderTestSuite) TestStartMember_Multiple() {
	name, host := "mycluster2", "127.0.0.1"
	ports := []int{8001, 8002, 8003}

	conn := NewMockzkConn(suite.ctrl)
	conn.EXPECT().Exists(filepath.Join("/protoactor", name)).Return(true, &zk.Stat{}, nil).AnyTimes()
	conn.EXPECT().CreateProtectedEphemeralSequentialForAnyInput().DoAndReturn(func(path string, data []byte, acl []zk.ACL) (string, error) {
		return fmt.Sprintf("%s%d", path, 1), nil
	})
	conn.EXPECT().CreateProtectedEphemeralSequentialForAnyInput().DoAndReturn(func(path string, data []byte, acl []zk.ACL) (string, error) {
		return fmt.Sprintf("%s%d", path, 2), nil
	})
	conn.EXPECT().CreateProtectedEphemeralSequentialForAnyInput().DoAndReturn(func(path string, data []byte, acl []zk.ACL) (string, error) {
		return fmt.Sprintf("%s%d", path, 3), nil
	})
	conn.EXPECT().Children(filepath.Join("/protoactor", name)).Return(actorPaths(1), &zk.Stat{Cversion: 1}, nil)
	conn.EXPECT().Children(filepath.Join("/protoactor", name)).Return(actorPaths(1, 2), &zk.Stat{Cversion: 2}, nil)
	conn.EXPECT().Children(filepath.Join("/protoactor", name)).Return(actorPaths(1, 2, 3), &zk.Stat{Cversion: 3}, nil).AnyTimes()

	conn.EXPECT().GetForAnyInput().DoAndReturn(func(path string) ([]byte, *zk.Stat, error) {
		idx := strToInt(last(strings.Split(path, "-"))) - 1
		node := NewNode(fmt.Sprintf("%v@%v:%v", name, host, ports[idx]), host, ports[idx], []string{"kind1"})
		data, _ := node.Serialize()
		return data, &zk.Stat{}, nil
	}).AnyTimes()

	conn.EXPECT().ChildrenW(filepath.Join("/protoactor", name)).Return(
		actorPaths(1),
		&zk.Stat{NumChildren: 1, Cversion: 1},
		make(chan zk.Event, 1),
		nil,
	).AnyTimes()

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
	for i, member := range members {
		addr := fmt.Sprintf("%s:%d", member.host, member.port)
		pr, c := suite.newClusterForTest(member.cluster, addr, conn)
		p[i] = pr
		err := p[i].StartMember(c)
		suite.NoError(err)
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
		suite.NoError(err)
		suite.Equal(len(members), len(nodes))
		flag := isNodesEqual(nodes)
		suite.Truef(flag, "Member not found - %+v", p[i].self)
	}
}

func (suite *ZkProviderTestSuite) TestUpdateMemberState() {
	name := "mycluster3"
	host := "127.0.0.1"
	port := 8000

	conn := NewMockzkConn(suite.ctrl)
	conn.EXPECT().Exists(filepath.Join("/protoactor", name)).Return(true, &zk.Stat{}, nil)
	conn.EXPECT().CreateProtectedEphemeralSequentialForAnyInput().DoAndReturn(func(path string, data []byte, acl []zk.ACL) (string, error) {
		return fmt.Sprintf("%s%d", path, 1), nil
	}).Times(2)
	conn.EXPECT().Children(filepath.Join("/protoactor", name)).Return([]string{fmt.Sprintf("actor-%d", 1)}, &zk.Stat{Cversion: 1}, nil)
	node := NewNode(fmt.Sprintf("%v@%v:%v", name, host, port), host, port, []string{"kind1"})
	data, _ := node.Serialize()
	conn.EXPECT().Get(fmt.Sprintf("/protoactor/%s/actor-%d", name, 1)).Return(data, &zk.Stat{Cversion: 1}, nil)
	conn.EXPECT().ChildrenW(filepath.Join("/protoactor", name)).Return(
		[]string{fmt.Sprintf("actor-%d", 1)},
		&zk.Stat{NumChildren: 1, Cversion: 1},
		make(chan zk.Event, 1),
		nil,
	).AnyTimes()

	p, c := suite.newClusterForTest("mycluster3", "127.0.0.1:8000", conn)
	err := p.StartMember(c)
	suite.NoError(err)

	state := cluster.ClusterState{[]string{"yes"}}
	err = p.UpdateClusterState(state)
	suite.NoError(err)
}

func (suite *ZkProviderTestSuite) TestUpdateMemberState_DoesNotReregisterAfterShutdown() {
	name := "mycluster4"
	host := "127.0.0.1"
	port := 8000

	conn := NewMockzkConn(suite.ctrl)
	conn.EXPECT().Exists(filepath.Join("/protoactor", name)).Return(true, &zk.Stat{}, nil)
	conn.EXPECT().CreateProtectedEphemeralSequentialForAnyInput().DoAndReturn(func(path string, data []byte, acl []zk.ACL) (string, error) {
		return fmt.Sprintf("%s%d", path, 1), nil
	}).Times(2)
	conn.EXPECT().Children(filepath.Join("/protoactor", name)).Return([]string{fmt.Sprintf("actor-%d", 1)}, &zk.Stat{Cversion: 1}, nil)
	node := NewNode(fmt.Sprintf("%v@%v:%v", name, host, port), host, port, []string{"kind1"})
	data, _ := node.Serialize()
	conn.EXPECT().Get(fmt.Sprintf("/protoactor/%s/actor-%d", name, 1)).Return(data, &zk.Stat{Cversion: 1}, nil)
	conn.EXPECT().ChildrenW(filepath.Join("/protoactor", name)).Return(
		[]string{fmt.Sprintf("actor-%d", 1)},
		&zk.Stat{NumChildren: 1, Cversion: 1},
		make(chan zk.Event, 1),
		nil,
	).AnyTimes()
	conn.EXPECT().DeleteForAnyInput()
	conn.EXPECT().Close()

	p, c := suite.newClusterForTest("mycluster4", "127.0.0.1:8000", conn)
	err := p.StartMember(c)
	suite.NoError(err)

	state := cluster.ClusterState{[]string{"yes"}}
	err = p.UpdateClusterState(state)
	suite.NoError(err)

	err = p.Shutdown(true)
	suite.NoError(err)

	err = p.UpdateClusterState(state)
	suite.Error(err)
}

func (suite *ZkProviderTestSuite) createMockProvider(conn *MockzkConn, opts ...Option) *Provider {
	zkCfg := defaultConfig()
	for _, fn := range opts {
		fn(zkCfg)
	}
	p := &Provider{
		cluster:             &cluster.Cluster{},
		baseKey:             zkCfg.BaseKey,
		clusterName:         "",
		deregistered:        false,
		shutdown:            false,
		self:                &Node{},
		members:             map[string]*Node{},
		conn:                conn,
		revision:            0,
		fullpath:            "",
		roleChangedListener: zkCfg.RoleChanged,
		roleChangedChan:     make(chan RoleType, 1),
		role:                Follower,
	}
	return p
}

func (suite *ZkProviderTestSuite) newClusterForTest(name string, addr string, conn *MockzkConn) (*Provider, *cluster.Cluster) {
	host, _port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}

	port, _ := strconv.Atoi(_port)

	cp := suite.createMockProvider(conn)

	remoteConfig := remote.Configure(host, port)
	config := cluster.Configure(name, cp, remoteConfig)

	system := actor.NewActorSystem()
	c := cluster.New(system, config)
	// use for test without start remote
	c.ActorSystem.ProcessRegistry.Address = addr
	c.MemberList = cluster.NewMemberList(c)
	return cp, c
}

type ZkProviderTestSuite struct {
	suite.Suite
	ctrl *gomock.Controller
}

func (suite *ZkProviderTestSuite) SetupTest() {
	suite.ctrl = gomock.NewController(suite.T())
}

func (suite *ZkProviderTestSuite) TearDownTest() {
	suite.ctrl.Finish()
}

func TestZkProviderTestSuite(t *testing.T) {
	suite.Run(t, new(ZkProviderTestSuite))
}

func actorPaths(seqs ...int) []string {
	p := make([]string, len(seqs))
	for i, seq := range seqs {
		p[i] = fmt.Sprintf("actor-%d", seq)
	}
	return p
}

func actorPath(seq int) string {
	return actorPaths(seq)[0]
}

func last(arr []string) string { return arr[len(arr)-1] }
