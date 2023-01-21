package cluster_test_tool

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PubSubDefaultRegistrationTestSuite struct {
	suite.Suite
	fixture *PubSubClusterFixture
}

func (suite *PubSubDefaultRegistrationTestSuite) SetupTest() {
	suite.fixture = NewPubSubClusterFixture(suite.T(), 1, true)
	suite.fixture.Initialize()
}

func (suite *PubSubDefaultRegistrationTestSuite) TearDownTest() {
	suite.fixture.ShutDown()
}

func (suite *PubSubDefaultRegistrationTestSuite) TestPubSubWorksWithDefaultTopicRegistration() {
	subscriberIds := suite.fixture.SubscriberIds("topic-default", 20)
	const topic = "topic-default-registration"
	const numMessage = 100

	suite.fixture.SubscribeAllTo(topic, subscriberIds)

	for i := 0; i < numMessage; i++ {
		data, err := suite.fixture.PublishData(topic, i)
		suite.Assert().NoError(err, "message "+strconv.Itoa(i)+" should not has error")
		suite.Assert().NotNil(data, "response "+strconv.Itoa(i)+" should not be nil")
	}

	suite.fixture.VerifyAllSubscribersGotAllTheData(subscriberIds, numMessage)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestPubSubDefaultRegistrationTestSuite(t *testing.T) {
	suite.Run(t, new(PubSubDefaultRegistrationTestSuite))
}
