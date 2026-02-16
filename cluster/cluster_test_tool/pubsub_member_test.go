package cluster_test_tool

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/suite"
)

type PubSubMemberTestSuite struct {
	suite.Suite
	fixture *PubSubClusterFixture
}

func (suite *PubSubMemberTestSuite) SetupTest() {
	suite.fixture = NewPubSubClusterFixture(suite.T(), 3, false)
	suite.fixture.Initialize()
}

func (suite *PubSubMemberTestSuite) TestWhenMemberLeavesPidSubscribersGetRemovedFromTheSubscriberList() {
	const topic = "leaving-member"

	props := actor.PropsFromFunc(func(context actor.Context) {
		if msg, ok := context.Message().(*DataPublished); ok {
			suite.fixture.AppendDelivery(Delivery{Identity: context.Self().String(), Data: int(msg.Data)})
		}
	})
	// spawn on members
	members := suite.fixture.GetMembers()
	leavingMember := members[0]
	leavingPid := leavingMember.ActorSystem.Root.Spawn(props)
	stayingMember := members[len(members)-1]
	stayingPid := stayingMember.ActorSystem.Root.Spawn(props)

	// subscribe by pids
	_, err := leavingMember.SubscribeByPid(topic, leavingPid)
	suite.Assert().NoError(err)
	_, err = stayingMember.SubscribeByPid(topic, stayingPid)
	suite.Assert().NoError(err)

	// to spice things up, also subscribe virtual actors
	subscribeIds := suite.fixture.SubscriberIds("leaving", 20)
	suite.fixture.SubscribeAllTo(topic, subscribeIds)

	// publish data
	_, err = suite.fixture.PublishData(topic, 1)
	suite.Assert().NoError(err)

	// everyone should have received the data
	WaitUntil(suite.T(), func() bool {
		suite.fixture.DeliveriesLock.RLock()
		defer suite.fixture.DeliveriesLock.RUnlock()
		return len(suite.fixture.Deliveries) == len(subscribeIds)+2
	}, "all subscribers should have received the data", DefaultWaitTimeout)

	suite.fixture.DeliveriesLock.RLock()
	suite.Assert().Equal(len(subscribeIds)+2, len(suite.fixture.Deliveries))
	suite.fixture.DeliveriesLock.RUnlock()

	suite.fixture.RemoveNode(leavingMember, true)

	WaitUntil(suite.T(), func() bool {
		blockedOnlyOne := true
		for _, member := range suite.fixture.GetMembers() {
			blockList := member.Remote.BlockList()
			blockedOnlyOne = blockedOnlyOne && blockList.Len() == 1
		}
		return blockedOnlyOne
	}, "Member should leave cluster", DefaultWaitTimeout)

	suite.fixture.ClearDeliveries()
	_, err = suite.fixture.PublishData(topic, 2)
	suite.Assert().NoError(err)

	// the failure in delivery caused topic actor to remove subscribers from the member that left
	// next publish should succeed and deliver to remaining subscribers
	WaitUntil(suite.T(), func() bool {
		suite.fixture.DeliveriesLock.RLock()
		defer suite.fixture.DeliveriesLock.RUnlock()
		return len(suite.fixture.Deliveries) == len(subscribeIds)+1
	}, "All subscribers apart the one that left should get the message", DefaultWaitTimeout)

	WaitUntil(suite.T(), func() bool {
		subscribers, err := suite.fixture.GetSubscribersForTopic(topic)
		suite.Assert().NoError(err)

		dontContainLeavingMember := true
		for _, subscriber := range subscribers.Subscribers {
			pid := subscriber.GetPid()
			if pid != nil && pid.Address == leavingPid.Address && pid.Id == leavingPid.Id {
				dontContainLeavingMember = false
				break
			}
		}
		return dontContainLeavingMember
	}, "Subscriber that left should be removed from subscribers list", DefaultWaitTimeout)
}

func (suite *PubSubMemberTestSuite) TearDownTest() {
	suite.fixture.ShutDown()
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestPubSubMemberTestSuite(t *testing.T) {
	suite.Run(t, new(PubSubMemberTestSuite))
}
