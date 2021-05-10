package zk

import (
	"strings"
	"testing"

	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
)

func (suite *MiscTestSuite) TestIntToStr() {
	suite.Equal("100", intToStr(100))
}

func (suite *MiscTestSuite) TestStrToInt() {
	suite.Equal(int(100), strToInt("100"))
	suite.Equal(int(0), strToInt("str0"))
}

func (suite *MiscTestSuite) TestIsStrBlank() {
	suite.True(isStrBlank(""))
	suite.False(isStrBlank("e"))
}

func (suite *MiscTestSuite) TestFormatBaseKey() {
	suite.Equal("/", formatBaseKey(""))
	suite.Equal("/a/b/c", formatBaseKey("a/b/c"))
	suite.Equal("/", formatBaseKey("/"))
	suite.Equal("/a", formatBaseKey("a/"))
}

func (suite *MiscTestSuite) TestParseSeq() {
	seq, err := parseSeq("/proto.actors/dev/my_cluster/_c_f4245284934bdaf384102b6fc233bd14-actor-0000000042")
	suite.Nil(err)
	suite.Equal(42, seq)
}

func (suite *MiscTestSuite) TestStringContains() {
	suite.True(stringContains([]string{"a", "b"}, "b"))
	suite.False(stringContains([]string{"a", "b"}, "c"))
}

func (suite *MiscTestSuite) TestMapString() {
	orig := []string{"A", "B", "C"}
	suite.ElementsMatch([]string{"a", "b", "c"}, mapString(orig, strings.ToLower))
	suite.ElementsMatch([]string{"A", "B", "C"}, orig)
}

func (suite *MiscTestSuite) TestSafeRun() {
	suite.NotPanics(func() { safeRun(func() { panic("don't worry, should panic here") }) })
}

func (suite *MiscTestSuite) TestNode() {
	node := NewNode("pod1", "192.168.0.1", 7788, []string{"kind1", "kind2"})
	host, port := node.GetAddress()
	suite.Equal("192.168.0.1", host)
	suite.Equal(7788, port)

	suite.Equal("192.168.0.1:7788", node.GetAddressString())

	suite.True(NewNode("pod1", "", 0, nil).Equal(node))

	_, ok := node.GetMeta("key")
	suite.False(ok)

	node.SetMeta(metaKeySeq, "100")
	suite.Equal(100, node.GetSeq())

	suite.Equal(&cluster.Member{
		Id:    "pod1",
		Host:  "192.168.0.1",
		Port:  int32(7788),
		Kinds: []string{"kind1", "kind2"},
	}, node.MemberStatus())

	data, err := node.Serialize()
	suite.Nil(err)
	suite.Equal(`{"id":"pod1","name":"pod1","host":"192.168.0.1","address":"192.168.0.1","port":7788,"kinds":["kind1","kind2"],"alive":true}`, string(data))

	node2 := &Node{}
	err = node2.Deserialize([]byte(`{"id":"pod1","name":"pod1","host":"192.168.0.1","address":"192.168.0.1","port":7788,"kinds":["kind1","kind2"],"alive":true}`))
	suite.Nil(err)
	node.Meta = nil
	suite.Equal(node2, node)
}

type MiscTestSuite struct {
	suite.Suite
	ctrl *gomock.Controller
}

func (suite *MiscTestSuite) SetupTest() {
	suite.ctrl = gomock.NewController(suite.T())
}

func (suite *MiscTestSuite) TearDownTest() {
	suite.ctrl.Finish()
}

func TestMiscTestSuite(t *testing.T) {
	suite.Run(t, new(MiscTestSuite))
}
